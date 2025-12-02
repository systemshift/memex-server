#!/usr/bin/env python3
"""
RAG Baseline - Full Corpus Search

Searches across ALL source documents in the corpus (not just per-question context).
Fair comparison against Memex graph retrieval.

Usage:
    python baseline_rag_full.py --limit 100
"""

import argparse
import hashlib
import json
import os
import time
from pathlib import Path

import numpy as np
from datasets import load_from_disk
from dotenv import load_dotenv
from neo4j import GraphDatabase
from openai import OpenAI

load_dotenv()

DATA_DIR = Path(__file__).parent / "data"
EMBEDDINGS_CACHE = Path(__file__).parent / ".corpus_embeddings.json"
RESULTS_FILE = Path(__file__).parent / "results_rag_full.json"

EMBEDDING_MODEL = "text-embedding-3-small"
NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "password"


def make_source_id(title: str, content: str) -> str:
    """Create source ID matching ingest_ai.py"""
    return "source:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def get_embedding(client: OpenAI, text: str, cache: dict) -> list[float]:
    """Get embedding with caching."""
    cache_key = hashlib.sha256(text[:1000].encode()).hexdigest()[:16]
    if cache_key in cache:
        return cache[cache_key]

    response = client.embeddings.create(
        model=EMBEDDING_MODEL,
        input=text[:8000]
    )
    embedding = response.data[0].embedding
    cache[cache_key] = embedding
    return embedding


def cosine_similarity(a: list[float], b: list[float]) -> float:
    a = np.array(a)
    b = np.array(b)
    return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))


def load_corpus_from_dataset(dataset) -> dict[str, str]:
    """Load all source documents from HotpotQA dataset."""
    sources = {}
    seen_ids = set()

    print(f"Building corpus from {len(dataset)} questions...")
    for i, ex in enumerate(dataset):
        titles = ex["context"]["title"]
        sentences = ex["context"]["sentences"]

        for title, sents in zip(titles, sentences):
            content = " ".join(sents)
            source_id = make_source_id(title, content)
            if source_id not in seen_ids:
                sources[source_id] = content
                seen_ids.add(source_id)

        if (i + 1) % 5000 == 0:
            print(f"  Processed {i + 1} questions, {len(sources)} unique sources")

    return sources


def get_ground_truth(example: dict) -> set[str]:
    """Get ground truth source IDs from HotpotQA."""
    relevant_titles = set(example["supporting_facts"]["title"])
    relevant_ids = set()

    for title, sents in zip(example["context"]["title"], example["context"]["sentences"]):
        if title in relevant_titles:
            content = " ".join(sents)
            relevant_ids.add(make_source_id(title, content))

    return relevant_ids


def calculate_metrics(retrieved: set, ground_truth: set) -> dict:
    if not retrieved or not ground_truth:
        return {"precision": 0, "recall": 0, "f1": 0}

    tp = len(retrieved & ground_truth)
    p = tp / len(retrieved)
    r = tp / len(ground_truth)
    f1 = 2 * p * r / (p + r) if (p + r) > 0 else 0
    return {"precision": p, "recall": r, "f1": f1}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=100)
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--top-k", type=int, default=20, help="Retrieve more since corpus is larger")
    parser.add_argument("--split", default="validation")
    args = parser.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY not set")
        return

    client = OpenAI()

    # Load corpus embeddings cache
    if EMBEDDINGS_CACHE.exists():
        print("Loading embeddings cache...")
        with open(EMBEDDINGS_CACHE) as f:
            cache = json.load(f)
        corpus_embeddings = cache.get("corpus", {})
        query_cache = cache.get("queries", {})
    else:
        corpus_embeddings = {}
        query_cache = {}

    # Load dataset first
    print("Loading HotpotQA dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    # Build corpus from dataset
    corpus = load_corpus_from_dataset(dataset)
    print(f"Loaded {len(corpus)} unique source documents")

    # Embed corpus (with progress)
    print("Embedding corpus (this may take a while on first run)...")
    corpus_ids = list(corpus.keys())
    needs_embedding = [sid for sid in corpus_ids if sid not in corpus_embeddings]

    if needs_embedding:
        print(f"Need to embed {len(needs_embedding)} documents...")
        for i, source_id in enumerate(needs_embedding):
            content = corpus[source_id]
            emb = get_embedding(client, content, {})  # Don't use cache for corpus
            corpus_embeddings[source_id] = emb

            if (i + 1) % 500 == 0:
                print(f"  Embedded {i + 1}/{len(needs_embedding)}")
                # Save checkpoint
                with open(EMBEDDINGS_CACHE, "w") as f:
                    json.dump({"corpus": corpus_embeddings, "queries": query_cache}, f)

        # Final save
        with open(EMBEDDINGS_CACHE, "w") as f:
            json.dump({"corpus": corpus_embeddings, "queries": query_cache}, f)
        print(f"Corpus embedding complete")
    else:
        print("Using cached corpus embeddings")

    # Build numpy matrix for fast search
    print("Building search index...")
    corpus_matrix = np.array([corpus_embeddings[sid] for sid in corpus_ids])

    end_idx = min(args.start + args.limit, len(dataset))
    print(f"Testing {args.start} to {end_idx} ({end_idx - args.start} questions)")

    metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}
    start_time = time.time()

    for i in range(args.start, end_idx):
        ex = dataset[i]
        question = ex["question"]
        ground_truth = get_ground_truth(ex)

        if not ground_truth:
            continue

        # Get question embedding
        q_emb = get_embedding(client, question, query_cache)
        q_vec = np.array(q_emb)

        # Search across full corpus
        similarities = corpus_matrix @ q_vec / (
            np.linalg.norm(corpus_matrix, axis=1) * np.linalg.norm(q_vec)
        )
        top_indices = np.argsort(similarities)[-args.top_k:][::-1]
        retrieved = {corpus_ids[idx] for idx in top_indices}

        # Calculate metrics
        m = calculate_metrics(retrieved, ground_truth)
        metrics["precision"] += m["precision"]
        metrics["recall"] += m["recall"]
        metrics["f1"] += m["f1"]
        metrics["count"] += 1

        if (i - args.start + 1) % 20 == 0:
            n = metrics["count"]
            print(f"Progress: {i - args.start + 1}/{end_idx - args.start} | "
                  f"P: {metrics['precision']/n:.3f} R: {metrics['recall']/n:.3f} F1: {metrics['f1']/n:.3f}")

    # Save query cache
    with open(EMBEDDINGS_CACHE, "w") as f:
        json.dump({"corpus": corpus_embeddings, "queries": query_cache}, f)

    n = max(metrics["count"], 1)
    results = {
        "precision": metrics["precision"] / n,
        "recall": metrics["recall"] / n,
        "f1": metrics["f1"] / n,
        "questions": metrics["count"],
        "corpus_size": len(corpus),
        "top_k": args.top_k,
        "config": {"start": args.start, "limit": args.limit}
    }

    with open(RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)

    elapsed = time.time() - start_time
    print(f"\n{'='*50}")
    print(f"RAG FULL CORPUS RESULTS ({results['questions']} questions)")
    print(f"{'='*50}")
    print(f"Corpus size: {results['corpus_size']} documents")
    print(f"Top-k: {results['top_k']}")
    print(f"Precision: {results['precision']:.3f}")
    print(f"Recall:    {results['recall']:.3f}")
    print(f"F1:        {results['f1']:.3f}")
    print(f"Time:      {elapsed:.1f}s")
    print(f"Saved:     {RESULTS_FILE}")


if __name__ == "__main__":
    main()
