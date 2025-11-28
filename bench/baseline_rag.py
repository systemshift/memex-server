#!/usr/bin/env python3
"""
Baseline RAG using vector search (no learning).

Uses OpenAI embeddings + cosine similarity for fair comparison.
This is a standard RAG setup that doesn't learn from queries.

Usage:
    python baseline_rag.py [--limit N] [--start N]
"""

import argparse
import hashlib
import json
import os
import time
from pathlib import Path

import numpy as np
from datasets import load_from_disk
from openai import OpenAI

DATA_DIR = Path(__file__).parent / "data"
EMBEDDINGS_FILE = Path(__file__).parent / ".embeddings_cache.json"
RESULTS_FILE = Path(__file__).parent / "results_rag.json"

# Use a cheap embedding model
EMBEDDING_MODEL = "text-embedding-3-small"


def make_node_id(title: str, content: str) -> str:
    """Create content-addressed node ID."""
    return "hotpot:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def load_embeddings_cache() -> dict:
    """Load cached embeddings."""
    if EMBEDDINGS_FILE.exists():
        with open(EMBEDDINGS_FILE) as f:
            return json.load(f)
    return {}


def save_embeddings_cache(cache: dict):
    """Save embeddings cache."""
    with open(EMBEDDINGS_FILE, "w") as f:
        json.dump(cache, f)


def get_embedding(client: OpenAI, text: str, cache: dict) -> list[float]:
    """Get embedding for text, using cache."""
    cache_key = hashlib.sha256(text.encode()).hexdigest()[:16]

    if cache_key in cache:
        return cache[cache_key]

    response = client.embeddings.create(
        model=EMBEDDING_MODEL,
        input=text[:8000]  # Truncate if too long
    )
    embedding = response.data[0].embedding
    cache[cache_key] = embedding
    return embedding


def cosine_similarity(a: list[float], b: list[float]) -> float:
    """Calculate cosine similarity between two vectors."""
    a = np.array(a)
    b = np.array(b)
    return np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b))


def get_ground_truth(example: dict) -> set[str]:
    """Extract ground truth relevant paragraph IDs."""
    relevant_titles = set(example["supporting_facts"]["title"])
    relevant_ids = set()

    titles = example["context"]["title"]
    sentences = example["context"]["sentences"]

    for title, sents in zip(titles, sentences):
        if title in relevant_titles:
            content = " ".join(sents)
            relevant_ids.add(make_node_id(title, content))

    return relevant_ids


def calculate_metrics(retrieved: set[str], ground_truth: set[str]) -> dict:
    """Calculate precision, recall, F1."""
    if not retrieved:
        return {"precision": 0, "recall": 0, "f1": 0}

    true_positives = len(retrieved & ground_truth)
    precision = true_positives / len(retrieved) if retrieved else 0
    recall = true_positives / len(ground_truth) if ground_truth else 0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0

    return {"precision": precision, "recall": recall, "f1": f1}


def main():
    parser = argparse.ArgumentParser(description="Baseline RAG benchmark")
    parser.add_argument("--limit", type=int, default=500, help="Number of queries")
    parser.add_argument("--start", type=int, default=3000, help="Starting index")
    parser.add_argument("--top-k", type=int, default=5, help="Number of documents to retrieve")
    parser.add_argument("--split", default="validation")
    args = parser.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY not set")
        return

    print("Loading dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    end_idx = min(args.start + args.limit, len(dataset))
    print(f"Benchmarking queries {args.start} to {end_idx}")

    client = OpenAI()
    embeddings_cache = load_embeddings_cache()

    metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}
    start_time = time.time()

    try:
        for i in range(args.start, end_idx):
            ex = dataset[i]
            question = ex["question"]
            ground_truth = get_ground_truth(ex)

            if not ground_truth:
                continue

            # Get question embedding
            q_emb = get_embedding(client, question, embeddings_cache)

            # Get paragraph embeddings and find top-k
            paragraphs = []
            titles = ex["context"]["title"]
            sentences = ex["context"]["sentences"]

            for title, sents in zip(titles, sentences):
                content = " ".join(sents)
                node_id = make_node_id(title, content)
                p_emb = get_embedding(client, content, embeddings_cache)
                similarity = cosine_similarity(q_emb, p_emb)
                paragraphs.append((node_id, similarity))

            # Sort by similarity, take top-k
            paragraphs.sort(key=lambda x: x[1], reverse=True)
            retrieved = set(p[0] for p in paragraphs[:args.top_k])

            # Calculate metrics
            m = calculate_metrics(retrieved, ground_truth)
            metrics["precision"] += m["precision"]
            metrics["recall"] += m["recall"]
            metrics["f1"] += m["f1"]
            metrics["count"] += 1

            # Progress
            if (i - args.start + 1) % 50 == 0:
                save_embeddings_cache(embeddings_cache)
                elapsed = time.time() - start_time
                print(f"Progress: {i - args.start + 1}/{end_idx - args.start} [{elapsed:.1f}s]")

    except KeyboardInterrupt:
        print("\nInterrupted!")
    finally:
        save_embeddings_cache(embeddings_cache)

    # Calculate averages
    results = {
        "rag_baseline": {
            "precision": metrics["precision"] / max(metrics["count"], 1),
            "recall": metrics["recall"] / max(metrics["count"], 1),
            "f1": metrics["f1"] / max(metrics["count"], 1),
            "queries": metrics["count"],
            "top_k": args.top_k,
        },
        "config": {
            "start": args.start,
            "limit": args.limit,
            "split": args.split,
            "embedding_model": EMBEDDING_MODEL,
        }
    }

    with open(RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)

    elapsed = time.time() - start_time
    print(f"\n{'='*50}")
    print(f"RAG BASELINE RESULTS ({results['rag_baseline']['queries']} queries)")
    print(f"{'='*50}")
    print(f"\nVector Search (top-{args.top_k}):")
    print(f"  Precision: {results['rag_baseline']['precision']:.3f}")
    print(f"  Recall:    {results['rag_baseline']['recall']:.3f}")
    print(f"  F1:        {results['rag_baseline']['f1']:.3f}")
    print(f"\nTime: {elapsed:.1f}s")
    print(f"Results saved to: {RESULTS_FILE}")


if __name__ == "__main__":
    main()
