#!/usr/bin/env python3
"""
Ingest HotpotQA paragraphs into Memex.

Usage:
    python ingest.py [--split validation|train] [--limit N]
"""

import argparse
import hashlib
import json
import sys
import time
from pathlib import Path

import httpx
from datasets import load_from_disk

MEMEX_URL = "http://localhost:8080"
DATA_DIR = Path(__file__).parent / "data"
PROGRESS_FILE = Path(__file__).parent / ".ingest_progress.json"


def make_node_id(title: str, content: str) -> str:
    """Create content-addressed node ID."""
    return "hotpot:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def load_progress() -> set:
    """Load set of already ingested node IDs."""
    if PROGRESS_FILE.exists():
        with open(PROGRESS_FILE) as f:
            return set(json.load(f))
    return set()


def save_progress(ingested: set):
    """Save progress to file."""
    with open(PROGRESS_FILE, "w") as f:
        json.dump(list(ingested), f)


def extract_paragraphs(dataset) -> list[dict]:
    """Extract unique paragraphs from dataset."""
    seen = set()
    paragraphs = []

    for ex in dataset:
        titles = ex["context"]["title"]
        sentences = ex["context"]["sentences"]

        for title, sents in zip(titles, sentences):
            content = " ".join(sents)
            node_id = make_node_id(title, content)

            if node_id not in seen:
                seen.add(node_id)
                paragraphs.append({
                    "id": node_id,
                    "type": "Paragraph",
                    "content": content,
                    "meta": {
                        "title": title,
                        "source": "hotpotqa",
                    }
                })

    return paragraphs


def ingest_node(client: httpx.Client, node: dict) -> bool:
    """Ingest a single node. Returns True if created, False if exists."""
    # Check if exists
    resp = client.get(f"/api/nodes/{node['id']}")
    if resp.status_code == 200:
        return False  # Already exists

    # Create node
    resp = client.post("/api/nodes", json=node)
    if resp.status_code in (200, 201):
        return True
    else:
        print(f"Error creating node {node['id']}: {resp.status_code} {resp.text}")
        return False


def main():
    parser = argparse.ArgumentParser(description="Ingest HotpotQA into Memex")
    parser.add_argument("--split", default="validation", choices=["train", "validation"])
    parser.add_argument("--limit", type=int, default=None, help="Limit number of paragraphs")
    parser.add_argument("--batch-size", type=int, default=100, help="Save progress every N nodes")
    args = parser.parse_args()

    print(f"Loading dataset from {DATA_DIR}...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]
    print(f"Loaded {len(dataset)} examples from {args.split} split")

    print("Extracting unique paragraphs...")
    paragraphs = extract_paragraphs(dataset)
    print(f"Found {len(paragraphs)} unique paragraphs")

    if args.limit:
        paragraphs = paragraphs[:args.limit]
        print(f"Limited to {len(paragraphs)} paragraphs")

    # Load progress
    ingested = load_progress()
    print(f"Already ingested: {len(ingested)} nodes")

    # Filter out already ingested
    to_ingest = [p for p in paragraphs if p["id"] not in ingested]
    print(f"Remaining to ingest: {len(to_ingest)} nodes")

    if not to_ingest:
        print("Nothing to ingest!")
        return

    # Ingest
    client = httpx.Client(base_url=MEMEX_URL, timeout=30)
    created = 0
    skipped = 0
    start_time = time.time()

    try:
        for i, node in enumerate(to_ingest):
            if ingest_node(client, node):
                created += 1
                ingested.add(node["id"])
            else:
                skipped += 1
                ingested.add(node["id"])  # Mark as done either way

            # Progress update
            if (i + 1) % args.batch_size == 0:
                save_progress(ingested)
                elapsed = time.time() - start_time
                rate = (i + 1) / elapsed
                remaining = (len(to_ingest) - i - 1) / rate if rate > 0 else 0
                print(f"Progress: {i + 1}/{len(to_ingest)} "
                      f"(created: {created}, skipped: {skipped}) "
                      f"[{rate:.1f}/s, ~{remaining:.0f}s remaining]")

    except KeyboardInterrupt:
        print("\nInterrupted! Saving progress...")
    finally:
        save_progress(ingested)

    elapsed = time.time() - start_time
    print(f"\nDone! Created: {created}, Skipped: {skipped}")
    print(f"Time: {elapsed:.1f}s ({len(to_ingest) / elapsed:.1f} nodes/s)")


if __name__ == "__main__":
    main()
