#!/usr/bin/env python3
"""
Memex LLM Extractor

Extracts entities and relationships from content using OpenAI,
then stores them in the Memex graph via API.
"""

import os
import sys
import json
import requests
from openai import OpenAI

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")

if not OPENAI_API_KEY:
    print("Error: OPENAI_API_KEY environment variable not set")
    sys.exit(1)

client = OpenAI(api_key=OPENAI_API_KEY)


EXTRACTION_PROMPT = """You are an expert at extracting structured information from text.
Given content, extract entities and relationships in JSON format.

Response format (return ONLY valid JSON, no markdown):
{
  "entities": [
    {"id": "unique-id", "type": "EntityType", "properties": {"key": "value"}}
  ],
  "relationships": [
    {"source": "entity-id", "target": "entity-id", "type": "RELATIONSHIP_TYPE"}
  ]
}

Guidelines:
- Create meaningful, unique IDs for entities (use lowercase-with-dashes)
- Use clear, consistent entity types (e.g., Person, Organization, Concept, Commit, File)
- Extract key properties relevant to the entity
- Relationships should represent semantic connections
- Be precise and avoid hallucination
- Return ONLY the JSON object, no extra text"""


def ingest_source(content, format_hint="text"):
    """Store raw content in Memex and get source ID"""
    response = requests.post(
        f"{MEMEX_URL}/api/ingest",
        json={"content": content, "format": format_hint}
    )
    response.raise_for_status()
    data = response.json()
    return data["source_id"]


def extract_with_llm(source_id, content, format_hint="text"):
    """Use OpenAI to extract entities and relationships"""
    user_prompt = f"Content format: {format_hint}\n\nContent:\n{content}\n\nExtract entities and relationships:"

    response = client.chat.completions.create(
        model="gpt-4o-mini",  # Cheaper for testing, use gpt-4 for production
        messages=[
            {"role": "system", "content": EXTRACTION_PROMPT},
            {"role": "user", "content": user_prompt}
        ],
        temperature=0.1,
        response_format={"type": "json_object"}
    )

    result = json.loads(response.choices[0].message.content)

    # Add metadata to all entities
    for entity in result.get("entities", []):
        if "properties" not in entity:
            entity["properties"] = {}
        entity["properties"]["extracted_from"] = source_id
        entity["properties"]["extractor"] = "openai"
        entity["properties"]["model"] = "gpt-4o-mini"

    # Add metadata to all relationships
    for rel in result.get("relationships", []):
        if "meta" not in rel:
            rel["meta"] = {}
        rel["meta"]["extracted_from"] = source_id
        rel["meta"]["extractor"] = "openai"

    return result


def store_ontology(extraction, source_id):
    """Store extracted entities and relationships in Memex"""
    created_nodes = []
    created_links = []

    # Store entities as nodes
    for entity in extraction.get("entities", []):
        try:
            response = requests.post(
                f"{MEMEX_URL}/api/nodes",
                json={
                    "id": entity["id"],
                    "type": entity["type"],
                    "meta": entity.get("properties", {})
                }
            )
            response.raise_for_status()
            created_nodes.append(entity["id"])
            print(f"  ✓ Created entity: {entity['id']} ({entity['type']})")
        except requests.HTTPError as e:
            print(f"  ✗ Failed to create entity {entity['id']}: {e}")

    # Create extracted_from links
    for entity_id in created_nodes:
        try:
            response = requests.post(
                f"{MEMEX_URL}/api/links",
                json={
                    "source": entity_id,
                    "target": source_id,
                    "type": "extracted_from",
                    "meta": {"extractor": "openai"}
                }
            )
            response.raise_for_status()
        except requests.HTTPError as e:
            print(f"  ✗ Failed to create extracted_from link: {e}")

    # Store relationships as links
    for rel in extraction.get("relationships", []):
        try:
            response = requests.post(
                f"{MEMEX_URL}/api/links",
                json={
                    "source": rel["source"],
                    "target": rel["target"],
                    "type": rel["type"],
                    "meta": rel.get("meta", {})
                }
            )
            response.raise_for_status()
            created_links.append(f"{rel['source']} -> {rel['target']}")
            print(f"  ✓ Created link: {rel['source']} --{rel['type']}--> {rel['target']}")
        except requests.HTTPError as e:
            print(f"  ✗ Failed to create link: {e}")

    return {"nodes": created_nodes, "links": created_links}


def extract_and_store(content, format_hint="text"):
    """Complete extraction pipeline"""
    print(f"1. Ingesting source content ({len(content)} bytes)...")
    source_id = ingest_source(content, format_hint)
    print(f"   Source ID: {source_id}")

    print(f"\n2. Extracting entities and relationships with LLM...")
    extraction = extract_with_llm(source_id, content, format_hint)
    print(f"   Found {len(extraction.get('entities', []))} entities")
    print(f"   Found {len(extraction.get('relationships', []))} relationships")

    print(f"\n3. Storing ontology in Memex...")
    result = store_ontology(extraction, source_id)

    print(f"\n✓ Complete!")
    print(f"  Created {len(result['nodes'])} nodes")
    print(f"  Created {len(result['links'])} links")

    return {"source_id": source_id, **result}


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python extract.py <content> [format]")
        print("   or: python extract.py --file <path> [format]")
        sys.exit(1)

    if sys.argv[1] == "--file":
        if len(sys.argv) < 3:
            print("Error: --file requires a path")
            sys.exit(1)
        with open(sys.argv[2], 'r') as f:
            content = f.read()
        format_hint = sys.argv[3] if len(sys.argv) > 3 else "text"
    else:
        content = sys.argv[1]
        format_hint = sys.argv[2] if len(sys.argv) > 2 else "text"

    try:
        extract_and_store(content, format_hint)
    except Exception as e:
        print(f"\n✗ Error: {e}")
        sys.exit(1)
