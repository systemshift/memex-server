#!/usr/bin/env python3
"""
AI-First Ingestion Pipeline for Memex.

Uses LLM to extract entities, relationships, and build a real knowledge graph.
Not just text storage - genuine comprehension and ontology building.

Usage:
    python ingest_ai.py [--limit N] [--start N] [--concurrency N]

Monitor progress:
    tail -f ingest.log
    cat .ingest_ai_progress.json | python -m json.tool
"""

import argparse
import asyncio
import hashlib
import json
import logging
import os
import re
import sys
import time
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional

import httpx
from datasets import load_from_disk
from dotenv import load_dotenv
from openai import AsyncOpenAI

# Load .env file
load_dotenv()

MEMEX_URL = "http://localhost:8080"
DATA_DIR = Path(__file__).parent / "data"

# Will be set in main() based on --worker-id
PROGRESS_FILE = None
LOG_FILE = None

# Use GPT-5-nano for cost efficiency
MODEL = "gpt-5-nano"

# Logger will be configured after parsing args
logger = logging.getLogger(__name__)

# Suppress httpx logging noise
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("httpcore").setLevel(logging.WARNING)

# Extraction prompt
EXTRACTION_PROMPT = """Analyze this text and extract structured knowledge.

Text: {content}
Title: {title}

Extract:
1. Named entities (people, places, organizations, concepts, events, works)
2. Relationships between entities
3. Key facts with dates if mentioned

Return JSON only, no markdown:
{{
  "entities": [
    {{"name": "Full Name", "type": "Person|Place|Organization|Concept|Event|Work|Technology", "description": "brief description"}}
  ],
  "relationships": [
    {{"source": "Entity Name", "target": "Entity Name", "type": "RELATIONSHIP_TYPE", "properties": {{}}}}
  ]
}}

Relationship types should be specific: BORN_IN, FOUNDED, CREATED, WROTE, LOCATED_IN, MEMBER_OF, WORKED_AT, SUCCEEDED_BY, PART_OF, INFLUENCED, DEFEATED, MARRIED_TO, etc.

Be thorough but precise. Only extract what's explicitly stated."""


def make_entity_id(name: str, entity_type: str) -> str:
    """Create deterministic entity ID for deduplication."""
    normalized = re.sub(r'[^a-z0-9]', '-', name.lower().strip())
    normalized = re.sub(r'-+', '-', normalized).strip('-')
    return f"{entity_type.lower()}:{normalized}"


def make_source_id(title: str, content: str) -> str:
    """Create content-addressed source ID."""
    return "source:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def load_progress() -> dict:
    """Load ingestion progress."""
    if PROGRESS_FILE.exists():
        with open(PROGRESS_FILE) as f:
            return json.load(f)
    return {
        "completed_sources": [],
        "last_index": 0,
        "entities_created": 0,
        "relationships_created": 0,
        "sources_created": 0,
        "errors": 0,
        "started_at": datetime.now().isoformat(),
        "last_update": datetime.now().isoformat(),
    }


def save_progress(progress: dict):
    """Save progress to file."""
    progress["last_update"] = datetime.now().isoformat()
    with open(PROGRESS_FILE, "w") as f:
        json.dump(progress, f, indent=2)


async def check_memex_health(http_client: httpx.AsyncClient) -> bool:
    """Check if Memex server is healthy."""
    try:
        resp = await http_client.get("/health")
        return resp.status_code == 200
    except Exception as e:
        logger.error(f"Memex health check failed: {e}")
        return False


async def extract_knowledge(client: AsyncOpenAI, title: str, content: str, retries: int = 3) -> Optional[dict]:
    """Use LLM to extract entities and relationships from text with retries."""
    for attempt in range(retries):
        try:
            response = await client.chat.completions.create(
                model=MODEL,
                messages=[{
                    "role": "user",
                    "content": EXTRACTION_PROMPT.format(title=title, content=content)
                }],
                max_completion_tokens=8000,
            )

            text = response.choices[0].message.content
            if text is None:
                logger.warning(f"Empty response for '{title[:50]}...' (attempt {attempt + 1})")
                if attempt < retries - 1:
                    await asyncio.sleep(2 ** attempt)  # Exponential backoff
                    continue
                return None
            text = text.strip()

            # Parse JSON from response
            # Handle potential markdown code blocks
            if "```" in text:
                match = re.search(r'```(?:json)?\s*([\s\S]*?)```', text)
                if match:
                    text = match.group(1)

            return json.loads(text)

        except json.JSONDecodeError as e:
            logger.warning(f"JSON parse error for '{title[:50]}...': {e}")
            return None
        except Exception as e:
            logger.warning(f"Extraction error for '{title[:50]}...' (attempt {attempt + 1}): {e}")
            if attempt < retries - 1:
                await asyncio.sleep(2 ** attempt)  # Exponential backoff
            else:
                return None
    return None


async def find_existing_entity(http_client: httpx.AsyncClient, entity_id: str) -> bool:
    """Check if entity already exists in Memex."""
    try:
        resp = await http_client.get(f"/api/nodes/{entity_id}")
        return resp.status_code == 200
    except:
        return False


async def create_node(http_client: httpx.AsyncClient, node: dict) -> bool:
    """Create a node in Memex."""
    try:
        resp = await http_client.post("/api/nodes", json=node)
        return resp.status_code in (200, 201)
    except Exception as e:
        logger.debug(f"Error creating node {node.get('id', 'unknown')}: {e}")
        return False


async def create_link(http_client: httpx.AsyncClient, link: dict) -> bool:
    """Create a link in Memex."""
    try:
        resp = await http_client.post("/api/links", json=link)
        return resp.status_code in (200, 201)
    except Exception as e:
        # Links may fail if nodes don't exist - that's ok
        return False


async def update_attention(http_client: httpx.AsyncClient, entity_ids: list[str], query_id: str):
    """Update attention edges between co-occurring entities."""
    if len(entity_ids) < 2:
        return

    # Create attention edges between all pairs (entities co-occur in same paragraph)
    for i, source in enumerate(entity_ids):
        for target in entity_ids[i + 1:]:
            try:
                await http_client.post("/api/edges/attention", json={
                    "source": source,
                    "target": target,
                    "query_id": query_id,
                    "weight": 0.7,  # Co-occurrence weight
                })
            except:
                pass


async def ingest_paragraph(
    llm_client: AsyncOpenAI,
    http_client: httpx.AsyncClient,
    title: str,
    content: str,
    progress: dict,
) -> tuple[int, int, bool]:
    """
    Ingest a single paragraph with AI extraction.
    Returns (entities_created, relationships_created, success).
    """
    source_id = make_source_id(title, content)

    # Skip if already processed
    if source_id in progress["completed_sources"]:
        return 0, 0, True

    # 1. Create source node (immutable, content-addressed)
    source_node = {
        "id": source_id,
        "type": "Source",
        "content": content,
        "meta": {
            "title": title,
            "dataset": "hotpotqa",
        }
    }
    await create_node(http_client, source_node)

    # 2. Extract knowledge using LLM
    extraction = await extract_knowledge(llm_client, title, content)

    if not extraction:
        progress["completed_sources"].append(source_id)
        progress["sources_created"] += 1
        progress["errors"] += 1
        return 0, 0, False

    entities_created = 0
    relationships_created = 0
    entity_ids = []

    # 3. Create entity nodes
    for entity in extraction.get("entities", []):
        try:
            entity_id = make_entity_id(entity["name"], entity["type"])
            entity_ids.append(entity_id)

            # Check if exists
            exists = await find_existing_entity(http_client, entity_id)

            if not exists:
                entity_node = {
                    "id": entity_id,
                    "type": entity["type"],
                    "content": entity.get("description", entity["name"]),
                    "meta": {
                        "name": entity["name"],
                    }
                }
                if await create_node(http_client, entity_node):
                    entities_created += 1

            # Link entity to source (provenance)
            await create_link(http_client, {
                "source": entity_id,
                "target": source_id,
                "type": "EXTRACTED_FROM",
                "meta": {}
            })
        except KeyError as e:
            logger.debug(f"Missing key in entity: {e}")
            continue

    # 4. Create relationship edges
    for rel in extraction.get("relationships", []):
        try:
            source_entity_id = make_entity_id(rel["source"], "Entity")  # Generic lookup
            target_entity_id = make_entity_id(rel["target"], "Entity")

            # Try to find actual entity IDs from our extracted list
            for eid in entity_ids:
                if rel["source"].lower().replace(" ", "-") in eid:
                    source_entity_id = eid
                if rel["target"].lower().replace(" ", "-") in eid:
                    target_entity_id = eid

            link = {
                "source": source_entity_id,
                "target": target_entity_id,
                "type": rel["type"],
                "meta": rel.get("properties", {})
            }
            if await create_link(http_client, link):
                relationships_created += 1
        except KeyError as e:
            logger.debug(f"Missing key in relationship: {e}")
            continue

    # 5. Update attention DAG (entities co-occur)
    await update_attention(http_client, entity_ids, source_id)

    progress["completed_sources"].append(source_id)
    progress["sources_created"] += 1

    return entities_created, relationships_created, True


async def get_graph_stats(http_client: httpx.AsyncClient) -> dict:
    """Get current graph statistics."""
    try:
        resp = await http_client.get("/api/graph/map")
        if resp.status_code == 200:
            return resp.json()
    except:
        pass
    return {}


async def main():
    global PROGRESS_FILE, LOG_FILE

    parser = argparse.ArgumentParser(description="AI-First Ingestion for Memex")
    parser.add_argument("--limit", type=int, default=100, help="Number of paragraphs to process")
    parser.add_argument("--start", type=int, default=0, help="Starting paragraph index")
    parser.add_argument("--concurrency", type=int, default=3, help="Concurrent API calls")
    parser.add_argument("--split", default="validation", choices=["train", "validation"])
    parser.add_argument("--resume", action="store_true", help="Resume from last progress")
    parser.add_argument("--worker-id", type=int, default=0, help="Worker ID for parallel runs")
    args = parser.parse_args()

    # Set up per-worker files
    base_dir = Path(__file__).parent
    if args.worker_id == 0:
        PROGRESS_FILE = base_dir / ".ingest_ai_progress.json"
        LOG_FILE = base_dir / "ingest.log"
    else:
        PROGRESS_FILE = base_dir / f".ingest_progress_worker_{args.worker_id}.json"
        LOG_FILE = base_dir / f"ingest_worker_{args.worker_id}.log"

    # Configure logging now that we know the log file
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s [%(levelname)s] %(message)s',
        handlers=[
            logging.FileHandler(LOG_FILE),
            logging.StreamHandler(sys.stdout)
        ],
        force=True  # Override any existing config
    )

    # Check API key
    if not os.environ.get("OPENAI_API_KEY"):
        logger.error("OPENAI_API_KEY environment variable not set")
        return

    logger.info(f"{'='*60}")
    logger.info(f"AI-FIRST INGESTION PIPELINE STARTING")
    logger.info(f"{'='*60}")
    logger.info(f"Model: {MODEL}")
    logger.info(f"Limit: {args.limit}, Start: {args.start}, Concurrency: {args.concurrency}")

    logger.info(f"Loading dataset from {DATA_DIR}...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]
    logger.info(f"Loaded {len(dataset)} examples from {args.split} split")

    # Extract unique paragraphs
    logger.info("Extracting unique paragraphs...")
    seen = set()
    paragraphs = []

    for ex in dataset:
        titles = ex["context"]["title"]
        sentences = ex["context"]["sentences"]

        for title, sents in zip(titles, sentences):
            content = " ".join(sents)
            source_id = make_source_id(title, content)

            if source_id not in seen:
                seen.add(source_id)
                paragraphs.append({"title": title, "content": content})

    logger.info(f"Found {len(paragraphs)} unique paragraphs")

    # Apply limits
    end_idx = min(args.start + args.limit, len(paragraphs))
    paragraphs = paragraphs[args.start:end_idx]
    logger.info(f"Processing paragraphs {args.start} to {end_idx}")

    # Load or reset progress
    if args.resume:
        progress = load_progress()
        logger.info(f"Resuming from progress: {progress['sources_created']} sources, "
              f"{progress['entities_created']} entities, "
              f"{progress['relationships_created']} relationships")
    else:
        progress = load_progress()
        # Reset counters but keep completed_sources for deduplication
        progress["started_at"] = datetime.now().isoformat()

    # Initialize clients
    llm_client = AsyncOpenAI()
    http_client = httpx.AsyncClient(base_url=MEMEX_URL, timeout=60)

    # Health check
    if not await check_memex_health(http_client):
        logger.error("Memex server is not healthy. Please start it first.")
        return

    logger.info("Memex server health check passed")

    start_time = time.time()
    total_entities = 0
    total_relationships = 0
    processed = 0
    successful = 0

    # Process with concurrency control
    semaphore = asyncio.Semaphore(args.concurrency)

    async def process_with_semaphore(para):
        async with semaphore:
            return await ingest_paragraph(
                llm_client, http_client,
                para["title"], para["content"],
                progress
            )

    try:
        # Process in batches for progress reporting
        batch_size = 10
        for batch_start in range(0, len(paragraphs), batch_size):
            batch = paragraphs[batch_start:batch_start + batch_size]

            tasks = [process_with_semaphore(p) for p in batch]
            results = await asyncio.gather(*tasks, return_exceptions=True)

            for result in results:
                if isinstance(result, tuple):
                    entities, rels, success = result
                    total_entities += entities
                    total_relationships += rels
                    progress["entities_created"] += entities
                    progress["relationships_created"] += rels
                    if success:
                        successful += 1
                elif isinstance(result, Exception):
                    logger.error(f"Task exception: {result}")
                    progress["errors"] += 1
                processed += 1

            # Save progress and report
            save_progress(progress)
            elapsed = time.time() - start_time
            rate = processed / elapsed if elapsed > 0 else 0
            remaining_secs = (len(paragraphs) - processed) / rate if rate > 0 else 0
            remaining = timedelta(seconds=int(remaining_secs))

            success_rate = (successful / processed * 100) if processed > 0 else 0

            logger.info(f"Progress: {processed}/{len(paragraphs)} "
                  f"({success_rate:.0f}% success) "
                  f"| Entities: {total_entities} | Rels: {total_relationships} "
                  f"| Rate: {rate:.2f}/s | ETA: {remaining}")

    except KeyboardInterrupt:
        logger.info("\nInterrupted by user!")
    finally:
        save_progress(progress)

        # Get final graph stats
        stats = await get_graph_stats(http_client)
        await http_client.aclose()

    elapsed = time.time() - start_time
    elapsed_td = timedelta(seconds=int(elapsed))

    logger.info(f"\n{'='*60}")
    logger.info(f"INGESTION COMPLETE")
    logger.info(f"{'='*60}")
    logger.info(f"Paragraphs processed: {processed}")
    logger.info(f"Successful extractions: {successful} ({successful/processed*100:.1f}%)" if processed > 0 else "")
    logger.info(f"Entities created: {total_entities}")
    logger.info(f"Relationships created: {total_relationships}")
    logger.info(f"Errors: {progress.get('errors', 0)}")
    logger.info(f"Time: {elapsed_td} ({processed/elapsed:.2f} paragraphs/s)" if elapsed > 0 else "")

    if stats:
        logger.info(f"\nGraph Statistics:")
        logger.info(f"  Total nodes: {stats.get('stats', {}).get('total_nodes', 'N/A')}")
        logger.info(f"  Total edges: {stats.get('stats', {}).get('total_edges', 'N/A')}")
        logger.info(f"  Node types: {stats.get('node_types', {})}")


if __name__ == "__main__":
    asyncio.run(main())
