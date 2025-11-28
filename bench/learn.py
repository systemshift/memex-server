#!/usr/bin/env python3
"""
Run queries through LLM to build attention DAG.

The LLM identifies which paragraphs are relevant to each question,
and we update Memex attention edges based on co-relevance.

Usage:
    python learn.py [--limit N] [--start N]
"""

import argparse
import hashlib
import json
import os
import time
from pathlib import Path

import httpx
from datasets import load_from_disk
from openai import OpenAI

MEMEX_URL = "http://localhost:8080"
DATA_DIR = Path(__file__).parent / "data"
PROGRESS_FILE = Path(__file__).parent / ".learn_progress.json"

# Use GPT-5 nano for cost efficiency
MODEL = "gpt-5-nano"


def make_node_id(title: str, content: str) -> str:
    """Create content-addressed node ID."""
    return "hotpot:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def load_progress() -> dict:
    """Load learning progress."""
    if PROGRESS_FILE.exists():
        with open(PROGRESS_FILE) as f:
            return json.load(f)
    return {"completed": [], "last_index": 0}


def save_progress(progress: dict):
    """Save progress to file."""
    with open(PROGRESS_FILE, "w") as f:
        json.dump(progress, f)


def get_relevant_paragraphs(client: OpenAI, question: str, paragraphs: list[dict]) -> list[str]:
    """
    Ask LLM which paragraphs are relevant to the question.
    Returns list of relevant node IDs.
    """
    # Format paragraphs for prompt
    para_text = ""
    for i, p in enumerate(paragraphs):
        para_text += f"\n[{i}] {p['title']}: {p['content'][:500]}...\n"

    prompt = f"""Question: {question}

Here are {len(paragraphs)} paragraphs. Which ones contain information needed to answer the question?

{para_text}

Return ONLY a JSON array of paragraph indices that are relevant. Example: [0, 3, 5]
If none are relevant, return: []"""

    try:
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            max_tokens=100,
            temperature=0,
        )

        # Parse response
        text = response.choices[0].message.content.strip()
        # Extract JSON array from response
        import re
        match = re.search(r'\[[\d,\s]*\]', text)
        if match:
            indices = json.loads(match.group())
            return [paragraphs[i]["id"] for i in indices if i < len(paragraphs)]
    except Exception as e:
        print(f"Error calling LLM: {e}")

    return []


def update_attention(client: httpx.Client, query_id: str, relevant_ids: list[str]):
    """Update attention edges between co-relevant paragraphs."""
    if len(relevant_ids) < 2:
        return 0

    edges_created = 0
    for i, source in enumerate(relevant_ids):
        for target in relevant_ids[i + 1:]:
            try:
                resp = client.post("/api/edges/attention", json={
                    "source": source,
                    "target": target,
                    "query_id": query_id,
                    "weight": 0.8,
                })
                if resp.status_code == 200:
                    edges_created += 1
            except Exception as e:
                print(f"Error updating attention: {e}")

    return edges_created


def main():
    parser = argparse.ArgumentParser(description="Build attention DAG from queries")
    parser.add_argument("--limit", type=int, default=1000, help="Number of queries to run")
    parser.add_argument("--start", type=int, default=0, help="Starting index")
    parser.add_argument("--split", default="validation", choices=["train", "validation"])
    args = parser.parse_args()

    # Check API key
    if not os.environ.get("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY environment variable not set")
        return

    print(f"Loading dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    # Load progress
    progress = load_progress()
    start_idx = max(args.start, progress["last_index"])

    end_idx = min(start_idx + args.limit, len(dataset))
    print(f"Processing queries {start_idx} to {end_idx} ({end_idx - start_idx} total)")

    openai_client = OpenAI()
    memex_client = httpx.Client(base_url=MEMEX_URL, timeout=30)

    total_edges = 0
    total_relevant = 0
    start_time = time.time()

    try:
        for i in range(start_idx, end_idx):
            ex = dataset[i]
            question = ex["question"]

            # Build paragraph list with IDs
            paragraphs = []
            titles = ex["context"]["title"]
            sentences = ex["context"]["sentences"]
            for title, sents in zip(titles, sentences):
                content = " ".join(sents)
                paragraphs.append({
                    "id": make_node_id(title, content),
                    "title": title,
                    "content": content,
                })

            # Get LLM judgment on relevance
            relevant_ids = get_relevant_paragraphs(openai_client, question, paragraphs)
            total_relevant += len(relevant_ids)

            # Update attention DAG
            query_id = hashlib.sha256(question.encode()).hexdigest()[:16]
            edges = update_attention(memex_client, query_id, relevant_ids)
            total_edges += edges

            # Progress
            progress["last_index"] = i + 1
            progress["completed"].append(ex["id"])

            if (i - start_idx + 1) % 10 == 0:
                save_progress(progress)
                elapsed = time.time() - start_time
                rate = (i - start_idx + 1) / elapsed
                print(f"Progress: {i - start_idx + 1}/{end_idx - start_idx} "
                      f"(edges: {total_edges}, avg relevant: {total_relevant/(i-start_idx+1):.1f}) "
                      f"[{rate:.1f}/s]")

    except KeyboardInterrupt:
        print("\nInterrupted!")
    finally:
        save_progress(progress)

    elapsed = time.time() - start_time
    queries_done = progress["last_index"] - start_idx
    print(f"\nDone! Processed {queries_done} queries")
    print(f"Total attention edges created: {total_edges}")
    print(f"Average relevant paragraphs per query: {total_relevant/max(queries_done,1):.2f}")
    print(f"Time: {elapsed:.1f}s ({queries_done/elapsed:.2f} queries/s)")


if __name__ == "__main__":
    main()
