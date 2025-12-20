#!/usr/bin/env python3
"""
ML Worker - Processes pending screenshots from memex.

Polls for unprocessed screenshots, runs vision LLM extraction,
creates entity nodes, updates status.

Usage:
    python ml_worker.py

Environment:
    MEMEX_URL       - Memex API URL (default: http://localhost:8080)
    POLL_INTERVAL   - Seconds between polls (default: 10)
    VISION_MODEL    - OpenAI model for vision (default: gpt-4o)
    OPENAI_API_KEY  - Required for LLM calls
"""

import asyncio
import base64
import json
import logging
import os
import re
import time
from typing import Optional

import httpx
from dotenv import load_dotenv
from openai import AsyncOpenAI

load_dotenv()

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")
POLL_INTERVAL = int(os.getenv("POLL_INTERVAL", "10"))
MODEL = os.getenv("VISION_MODEL", "gpt-4o")

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s'
)
logger = logging.getLogger(__name__)

EXTRACTION_PROMPT = """Analyze this screenshot and extract workflow knowledge.

Identify:
1. What application or website is visible
2. What task or activity the user appears to be doing
3. Key information visible (names, projects, documents, data)
4. Any workflow patterns, processes, or procedures being followed

Return JSON only, no markdown:
{
  "application": "application or website name",
  "task": "brief description of what user is doing",
  "entities": [
    {"name": "entity name", "type": "Person|Project|Tool|Document|Organization|Concept", "description": "brief description"}
  ],
  "relationships": [
    {"source": "entity name", "target": "entity name", "type": "WORKS_ON|USES|VIEWS|CREATES|EDITS|RELATED_TO"}
  ],
  "summary": "1-2 sentence summary of the screenshot content"
}

Focus on work-relevant knowledge. Only extract what is clearly visible."""


async def get_pending_screenshots(client: httpx.AsyncClient) -> list:
    """Get screenshots with status=pending."""
    try:
        # Query for Screenshot type nodes
        resp = await client.get(
            f"{MEMEX_URL}/api/query/filter",
            params={"type": "Screenshot"},
            timeout=30,
        )
        if resp.status_code != 200:
            logger.warning(f"Filter query failed: {resp.status_code}")
            return []

        data = resp.json()
        pending = []

        # Check each screenshot's status
        for node_id in data.get("nodes", []):
            try:
                node_resp = await client.get(f"{MEMEX_URL}/api/nodes/{node_id}")
                if node_resp.status_code == 200:
                    node = node_resp.json()
                    meta = node.get("Meta", {})
                    if meta.get("status") == "pending":
                        pending.append(node)
            except Exception as e:
                logger.debug(f"Error fetching node {node_id}: {e}")

        return pending

    except Exception as e:
        logger.error(f"Error fetching pending screenshots: {e}")
        return []


async def extract_knowledge(llm: AsyncOpenAI, screenshot_b64: str) -> Optional[dict]:
    """Use vision LLM to extract knowledge from screenshot."""
    try:
        response = await llm.chat.completions.create(
            model=MODEL,
            messages=[{
                "role": "user",
                "content": [
                    {"type": "text", "text": EXTRACTION_PROMPT},
                    {
                        "type": "image_url",
                        "image_url": {"url": f"data:image/png;base64,{screenshot_b64}"}
                    },
                ],
            }],
            max_tokens=2000,
        )

        text = response.choices[0].message.content
        if not text:
            return None

        text = text.strip()

        # Handle markdown code blocks
        if "```" in text:
            match = re.search(r'```(?:json)?\s*([\s\S]*?)```', text)
            if match:
                text = match.group(1)

        return json.loads(text)

    except json.JSONDecodeError as e:
        logger.warning(f"JSON parse error: {e}")
        return None
    except Exception as e:
        logger.error(f"Extraction error: {e}")
        return None


async def update_status(
    client: httpx.AsyncClient,
    node_id: str,
    status: str,
    summary: Optional[str] = None
):
    """Update screenshot node status via PATCH endpoint."""
    meta = {"status": status, "processed_at": time.strftime("%Y-%m-%dT%H:%M:%SZ")}
    if summary:
        meta["summary"] = summary

    try:
        resp = await client.patch(
            f"{MEMEX_URL}/api/nodes/{node_id}",
            json={"meta": meta},
            timeout=10,
        )
        if resp.status_code != 200:
            logger.warning(f"Failed to update status for {node_id}: {resp.status_code}")
    except Exception as e:
        logger.error(f"Error updating status: {e}")


def make_entity_id(name: str, entity_type: str) -> str:
    """Create deterministic entity ID."""
    normalized = re.sub(r'[^a-z0-9]', '-', name.lower().strip())
    normalized = re.sub(r'-+', '-', normalized).strip('-')
    return f"{entity_type.lower()}:{normalized}"


async def process_screenshot(
    http: httpx.AsyncClient,
    llm: AsyncOpenAI,
    screenshot: dict,
) -> bool:
    """Process a single screenshot."""
    node_id = screenshot.get("ID")
    content_b64 = screenshot.get("Content", "")

    if not content_b64:
        logger.warning(f"Screenshot {node_id} has no content")
        await update_status(http, node_id, "failed")
        return False

    logger.info(f"Processing: {node_id}")

    # Extract knowledge using vision LLM
    extraction = await extract_knowledge(llm, content_b64)

    if not extraction:
        logger.warning(f"Extraction failed for {node_id}")
        await update_status(http, node_id, "failed")
        return False

    entity_ids = []

    # Create entity nodes
    for entity in extraction.get("entities", []):
        try:
            entity_id = make_entity_id(entity["name"], entity["type"])
            entity_ids.append(entity_id)

            await http.post(f"{MEMEX_URL}/api/nodes", json={
                "id": entity_id,
                "type": entity["type"],
                "content": entity.get("description", entity["name"]),
                "meta": {
                    "name": entity["name"],
                    "source_screenshot": node_id,
                },
            })

            # Link entity to screenshot
            await http.post(f"{MEMEX_URL}/api/links", json={
                "source": entity_id,
                "target": node_id,
                "type": "EXTRACTED_FROM",
            })

        except Exception as e:
            logger.debug(f"Error creating entity: {e}")

    # Create relationships between entities
    for rel in extraction.get("relationships", []):
        try:
            src_id = make_entity_id(rel["source"], "Entity")
            tgt_id = make_entity_id(rel["target"], "Entity")

            # Try to match actual entity IDs
            for eid in entity_ids:
                if rel["source"].lower().replace(" ", "-") in eid:
                    src_id = eid
                if rel["target"].lower().replace(" ", "-") in eid:
                    tgt_id = eid

            await http.post(f"{MEMEX_URL}/api/links", json={
                "source": src_id,
                "target": tgt_id,
                "type": rel["type"],
            })

        except Exception as e:
            logger.debug(f"Error creating relationship: {e}")

    # Update attention edges for co-occurring entities
    for i, src in enumerate(entity_ids):
        for tgt in entity_ids[i + 1:]:
            try:
                await http.post(f"{MEMEX_URL}/api/edges/attention", json={
                    "source": src,
                    "target": tgt,
                    "query_id": f"screenshot:{node_id}",
                    "weight": 0.7,
                })
            except:
                pass

    # Update screenshot status
    await update_status(http, node_id, "processed", extraction.get("summary"))

    logger.info(
        f"Processed: {node_id} - "
        f"{len(extraction.get('entities', []))} entities, "
        f"{len(extraction.get('relationships', []))} relationships"
    )
    return True


async def main():
    # Check for API key
    if not os.environ.get("OPENAI_API_KEY"):
        logger.error("OPENAI_API_KEY environment variable not set")
        return

    logger.info("=" * 60)
    logger.info("ML Worker Starting")
    logger.info("=" * 60)
    logger.info(f"Memex URL: {MEMEX_URL}")
    logger.info(f"Poll interval: {POLL_INTERVAL}s")
    logger.info(f"Vision model: {MODEL}")
    logger.info("=" * 60)

    llm = AsyncOpenAI()
    http = httpx.AsyncClient(timeout=60)

    try:
        while True:
            pending = await get_pending_screenshots(http)

            if pending:
                logger.info(f"Found {len(pending)} pending screenshots")
                for screenshot in pending:
                    await process_screenshot(http, llm, screenshot)
            else:
                logger.debug("No pending screenshots")

            await asyncio.sleep(POLL_INTERVAL)

    except KeyboardInterrupt:
        logger.info("Interrupted by user")
    finally:
        await http.aclose()
        logger.info("ML Worker stopped")


if __name__ == "__main__":
    asyncio.run(main())
