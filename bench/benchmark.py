#!/usr/bin/env python3
"""
Benchmark Memex retrieval against ground truth.

Measures precision and recall of retrieval using:
1. Memex search (baseline)
2. Memex attention-weighted retrieval (after learning)

Usage:
    python benchmark.py [--limit N] [--start N]
"""

import argparse
import hashlib
import json
import time
from pathlib import Path

import httpx
from datasets import load_from_disk

MEMEX_URL = "http://localhost:8080"
DATA_DIR = Path(__file__).parent / "data"
RESULTS_FILE = Path(__file__).parent / "results.json"


def make_node_id(title: str, content: str) -> str:
    """Create content-addressed node ID."""
    return "hotpot:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def get_ground_truth(example: dict) -> set[str]:
    """Extract ground truth relevant paragraph IDs from supporting_facts."""
    relevant_titles = set(example["supporting_facts"]["title"])

    # Map titles to node IDs
    relevant_ids = set()
    titles = example["context"]["title"]
    sentences = example["context"]["sentences"]

    for title, sents in zip(titles, sentences):
        if title in relevant_titles:
            content = " ".join(sents)
            relevant_ids.add(make_node_id(title, content))

    return relevant_ids


def search_memex(client: httpx.Client, question: str, limit: int = 10) -> list[str]:
    """Search Memex using text search."""
    try:
        # Extract key terms from question
        resp = client.get("/api/query/search", params={"q": question, "limit": limit})
        if resp.status_code == 200:
            data = resp.json()
            return [node["ID"] for node in data.get("nodes", [])]
    except Exception as e:
        print(f"Search error: {e}")
    return []


def search_with_attention(client: httpx.Client, question: str, start_node: str,
                          min_weight: float = 0.3, max_nodes: int = 20) -> list[str]:
    """Search using attention-weighted subgraph."""
    try:
        resp = client.get("/api/query/attention_subgraph", params={
            "start": start_node,
            "min_weight": min_weight,
            "max_nodes": max_nodes,
        })
        if resp.status_code == 200:
            data = resp.json()
            return [node["ID"] for node in data.get("nodes", [])]
    except Exception as e:
        pass  # Attention subgraph may not exist for all nodes
    return []


def calculate_metrics(retrieved: set[str], ground_truth: set[str]) -> dict:
    """Calculate precision, recall, F1."""
    if not retrieved:
        return {"precision": 0, "recall": 0, "f1": 0, "retrieved": 0, "relevant": len(ground_truth)}

    true_positives = len(retrieved & ground_truth)
    precision = true_positives / len(retrieved) if retrieved else 0
    recall = true_positives / len(ground_truth) if ground_truth else 0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0

    return {
        "precision": precision,
        "recall": recall,
        "f1": f1,
        "retrieved": len(retrieved),
        "relevant": len(ground_truth),
        "true_positives": true_positives,
    }


def main():
    parser = argparse.ArgumentParser(description="Benchmark Memex retrieval")
    parser.add_argument("--limit", type=int, default=500, help="Number of queries")
    parser.add_argument("--start", type=int, default=3000, help="Starting index (use later half for testing)")
    parser.add_argument("--split", default="validation")
    args = parser.parse_args()

    print(f"Loading dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    end_idx = min(args.start + args.limit, len(dataset))
    print(f"Benchmarking queries {args.start} to {end_idx}")

    client = httpx.Client(base_url=MEMEX_URL, timeout=30)

    # Metrics accumulators
    search_metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}
    attention_metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}

    start_time = time.time()

    for i in range(args.start, end_idx):
        ex = dataset[i]
        question = ex["question"]
        ground_truth = get_ground_truth(ex)

        if not ground_truth:
            continue

        # Method 1: Basic search
        search_results = search_memex(client, question, limit=10)
        search_m = calculate_metrics(set(search_results), ground_truth)
        search_metrics["precision"] += search_m["precision"]
        search_metrics["recall"] += search_m["recall"]
        search_metrics["f1"] += search_m["f1"]
        search_metrics["count"] += 1

        # Method 2: Attention-weighted (if we have a starting point)
        if search_results:
            attention_results = search_with_attention(
                client, question, search_results[0], min_weight=0.3, max_nodes=20
            )
            if attention_results:
                attention_m = calculate_metrics(set(attention_results), ground_truth)
                attention_metrics["precision"] += attention_m["precision"]
                attention_metrics["recall"] += attention_m["recall"]
                attention_metrics["f1"] += attention_m["f1"]
                attention_metrics["count"] += 1

        # Progress
        if (i - args.start + 1) % 50 == 0:
            elapsed = time.time() - start_time
            print(f"Progress: {i - args.start + 1}/{end_idx - args.start} [{elapsed:.1f}s]")

    # Calculate averages
    results = {
        "search": {
            "precision": search_metrics["precision"] / max(search_metrics["count"], 1),
            "recall": search_metrics["recall"] / max(search_metrics["count"], 1),
            "f1": search_metrics["f1"] / max(search_metrics["count"], 1),
            "queries": search_metrics["count"],
        },
        "attention": {
            "precision": attention_metrics["precision"] / max(attention_metrics["count"], 1),
            "recall": attention_metrics["recall"] / max(attention_metrics["count"], 1),
            "f1": attention_metrics["f1"] / max(attention_metrics["count"], 1),
            "queries": attention_metrics["count"],
        },
        "config": {
            "start": args.start,
            "limit": args.limit,
            "split": args.split,
        }
    }

    # Save results
    with open(RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)

    # Print results
    elapsed = time.time() - start_time
    print(f"\n{'='*50}")
    print(f"BENCHMARK RESULTS ({results['search']['queries']} queries)")
    print(f"{'='*50}")
    print(f"\nBasic Search:")
    print(f"  Precision: {results['search']['precision']:.3f}")
    print(f"  Recall:    {results['search']['recall']:.3f}")
    print(f"  F1:        {results['search']['f1']:.3f}")
    print(f"\nAttention-Weighted:")
    print(f"  Precision: {results['attention']['precision']:.3f}")
    print(f"  Recall:    {results['attention']['recall']:.3f}")
    print(f"  F1:        {results['attention']['f1']:.3f}")
    print(f"\nImprovement:")
    if results['search']['f1'] > 0:
        improvement = (results['attention']['f1'] - results['search']['f1']) / results['search']['f1'] * 100
        print(f"  F1: {improvement:+.1f}%")
    print(f"\nTime: {elapsed:.1f}s")
    print(f"Results saved to: {RESULTS_FILE}")


if __name__ == "__main__":
    main()
