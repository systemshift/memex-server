#!/usr/bin/env python3
"""
Ingest Mock Company Data into Demo Neo4j

Reads generated mock data and ingests directly into the demo Neo4j instance.
Uses LLM for entity extraction.

Usage:
    python ingest_demo.py [--dry-run]
"""

import argparse
import asyncio
import json
import logging
import os
import re
import sys
import time
from pathlib import Path

from dotenv import load_dotenv
from neo4j import GraphDatabase
from openai import OpenAI

load_dotenv()

# Demo Neo4j on different port
NEO4J_URI = "bolt://localhost:7688"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "demopass"

MOCK_DATA_DIR = Path(__file__).parent / "mock_data"

MODEL = "gpt-5-nano"

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)


EXTRACTION_PROMPT = """Analyze this {doc_type} and extract structured knowledge for a business knowledge graph.

{content}

Extract:
1. Named entities (people, companies, projects, concepts, technologies, roles)
2. Relationships between entities
3. Key facts, decisions, amounts, dates

Return JSON only:
{{
  "entities": [
    {{"name": "Full Name", "type": "Person|Company|Project|Concept|Technology|Role|Event|Amount", "description": "brief description"}}
  ],
  "relationships": [
    {{"source": "Entity Name", "target": "Entity Name", "type": "RELATIONSHIP_TYPE"}}
  ]
}}

Relationship types: WORKS_AT, SENT_EMAIL, RECEIVED_EMAIL, MENTIONED_IN, PART_OF,
DISCUSSED, APPROVED, NEGOTIATED, CUSTOMER_OF, VENDOR_OF, MANAGES, ATTENDED, etc.

Be thorough. Extract all business-relevant entities and relationships."""


def make_entity_id(name: str, entity_type: str) -> str:
    """Create deterministic entity ID."""
    normalized = re.sub(r'[^a-z0-9]', '-', name.lower().strip())
    normalized = re.sub(r'-+', '-', normalized).strip('-')
    return f"{entity_type.lower()}:{normalized}"


def make_source_id(doc_type: str, doc_id: str) -> str:
    """Create source ID."""
    return f"source:{doc_type.lower()}:{doc_id}"


def format_email(email: dict) -> str:
    to_str = ", ".join(email.get("to", []))
    cc_str = ", ".join(email.get("cc", [])) if email.get("cc") else ""
    parts = [f"From: {email['from']}", f"To: {to_str}"]
    if cc_str:
        parts.append(f"CC: {cc_str}")
    parts.extend([f"Subject: {email['subject']}", f"Date: {email['date']}", "", email['body']])
    return "\n".join(parts)


def format_slack(msg: dict) -> str:
    return f"Channel: {msg['channel']}\nAuthor: {msg['author']}\nDate: {msg['date']}\n\n{msg['content']}"


def format_document(doc: dict) -> str:
    return f"Title: {doc['title']}\nAuthor: {doc['author']}\nDate: {doc['date']}\n\n{doc['content']}"


def format_calendar(event: dict) -> str:
    attendees = ", ".join(event.get("attendees", []))
    return f"Event: {event['title']}\nOrganizer: {event['organizer']}\nAttendees: {attendees}\nDate: {event['date']}\n\n{event.get('description', '')}"


def format_invoice(inv: dict) -> str:
    return f"Invoice: {inv['id']}\nCustomer: {inv['customer']}\nAmount: ${inv['amount']:,.2f}\nDescription: {inv['description']}\nStatus: {inv['status']}"


def format_po(po: dict) -> str:
    return f"PO: {po['id']}\nVendor: {po['vendor']}\nAmount: ${po['amount']:,.2f}\nDescription: {po['description']}\nApproved By: {po.get('approved_by', 'N/A')}"


def extract_knowledge(client: OpenAI, doc_type: str, content: str) -> dict | None:
    """Use LLM to extract entities and relationships."""
    try:
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{
                "role": "user",
                "content": EXTRACTION_PROMPT.format(doc_type=doc_type, content=content)
            }],
            max_completion_tokens=4000,
            reasoning_effort="low",
        )
        text = response.choices[0].message.content
        if not text:
            return None
        text = text.strip()
        if "```" in text:
            match = re.search(r'```(?:json)?\s*([\s\S]*?)```', text)
            if match:
                text = match.group(1)
        return json.loads(text)
    except Exception as e:
        logger.warning(f"Extraction error: {e}")
        return None


def create_node(session, node_id: str, node_type: str, properties: dict):
    """Create or merge a node."""
    props_json = json.dumps(properties)
    session.run("""
        MERGE (n:Node {id: $id})
        SET n.type = $type, n.properties = $props
    """, id=node_id, type=node_type, props=props_json)


def create_link(session, source_id: str, target_id: str, rel_type: str):
    """Create a relationship between nodes."""
    session.run("""
        MATCH (a:Node {id: $source})
        MATCH (b:Node {id: $target})
        MERGE (a)-[r:LINK {type: $rel_type}]->(b)
    """, source=source_id, target=target_id, rel_type=rel_type)


def ingest_document(
    driver,
    llm_client: OpenAI,
    doc_type: str,
    doc: dict,
    formatted_content: str,
) -> tuple[int, int]:
    """Ingest a single document."""

    source_id = make_source_id(doc_type, doc['id'])

    # Create source node
    with driver.session() as session:
        create_node(session, source_id, "Source", {
            "doc_type": doc_type,
            "original_id": doc['id'],
            "content": formatted_content[:5000],  # Truncate for storage
            **{k: str(v)[:500] for k, v in doc.items() if k not in ('id', 'type', 'body', 'content')}
        })

    # Extract knowledge
    extraction = extract_knowledge(llm_client, doc_type, formatted_content)

    if not extraction:
        return 0, 0

    entities_created = 0
    relationships_created = 0
    entity_ids = []

    with driver.session() as session:
        # Create entity nodes
        for entity in extraction.get("entities", []):
            try:
                entity_id = make_entity_id(entity["name"], entity["type"])
                entity_ids.append((entity_id, entity["name"]))

                create_node(session, entity_id, entity["type"], {
                    "name": entity["name"],
                    "description": entity.get("description", ""),
                })
                entities_created += 1

                # Link entity to source
                create_link(session, entity_id, source_id, "EXTRACTED_FROM")

            except (KeyError, TypeError):
                continue

        # Create relationship edges
        for rel in extraction.get("relationships", []):
            try:
                # Find matching entity IDs
                source_entity_id = None
                target_entity_id = None

                for eid, ename in entity_ids:
                    if rel["source"].lower() == ename.lower():
                        source_entity_id = eid
                    if rel["target"].lower() == ename.lower():
                        target_entity_id = eid

                # Fallback to generated IDs
                if not source_entity_id:
                    source_entity_id = make_entity_id(rel["source"], "Entity")
                if not target_entity_id:
                    target_entity_id = make_entity_id(rel["target"], "Entity")

                create_link(session, source_entity_id, target_entity_id, rel["type"])
                relationships_created += 1

            except (KeyError, TypeError):
                continue

    return entities_created, relationships_created


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        logger.error("OPENAI_API_KEY not set")
        return

    logger.info("="*60)
    logger.info("DEMO DATA INGESTION")
    logger.info("="*60)
    logger.info(f"Neo4j: {NEO4J_URI}")

    # Load all mock data
    all_docs = []

    with open(MOCK_DATA_DIR / "emails.json") as f:
        for e in json.load(f):
            all_docs.append(("Email", e, format_email(e)))

    with open(MOCK_DATA_DIR / "slack.json") as f:
        for s in json.load(f):
            all_docs.append(("Slack", s, format_slack(s)))

    with open(MOCK_DATA_DIR / "documents.json") as f:
        for d in json.load(f):
            all_docs.append(("Document", d, format_document(d)))

    with open(MOCK_DATA_DIR / "calendar.json") as f:
        for e in json.load(f):
            all_docs.append(("Calendar", e, format_calendar(e)))

    with open(MOCK_DATA_DIR / "invoices.json") as f:
        for i in json.load(f):
            all_docs.append(("Invoice", i, format_invoice(i)))

    with open(MOCK_DATA_DIR / "purchase_orders.json") as f:
        for p in json.load(f):
            all_docs.append(("PurchaseOrder", p, format_po(p)))

    logger.info(f"Total documents: {len(all_docs)}")

    if args.dry_run:
        logger.info("DRY RUN - extraction only")
        driver = None
    else:
        driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
        # Verify connection
        with driver.session() as s:
            s.run("RETURN 1")
        logger.info("Connected to Neo4j")

    llm_client = OpenAI()

    start_time = time.time()
    total_entities = 0
    total_rels = 0

    for i, (doc_type, doc, content) in enumerate(all_docs):
        if args.dry_run:
            extraction = extract_knowledge(llm_client, doc_type, content)
            if extraction:
                total_entities += len(extraction.get("entities", []))
                total_rels += len(extraction.get("relationships", []))
        else:
            entities, rels = ingest_document(driver, llm_client, doc_type, doc, content)
            total_entities += entities
            total_rels += rels

        if (i + 1) % 10 == 0:
            elapsed = time.time() - start_time
            logger.info(f"Progress: {i+1}/{len(all_docs)} | Entities: {total_entities} | Rels: {total_rels} | {elapsed:.1f}s")

    if driver:
        # Get final stats
        with driver.session() as s:
            nodes = s.run("MATCH (n) RETURN count(n) as c").single()["c"]
            edges = s.run("MATCH ()-[r]->() RETURN count(r) as c").single()["c"]
        driver.close()
        logger.info(f"\nGraph stats: {nodes} nodes, {edges} edges")

    elapsed = time.time() - start_time
    logger.info(f"\n{'='*60}")
    logger.info("INGESTION COMPLETE")
    logger.info(f"{'='*60}")
    logger.info(f"Documents: {len(all_docs)}")
    logger.info(f"Entities: {total_entities}")
    logger.info(f"Relationships: {total_rels}")
    logger.info(f"Time: {elapsed:.1f}s")


if __name__ == "__main__":
    main()
