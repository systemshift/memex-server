#!/usr/bin/env python3
"""
Knowledge Graph Retrieval Benchmark V2 - Multi-hop + Higher Limits

Improvements over V1:
- Multi-hop traversal (2 hops instead of 1)
- Higher limits for entity matching
- Also follows explicit relationship edges, not just ATTENDED

Usage:
    python benchmark_kg_v2.py --limit 100
"""

import argparse
import hashlib
import json
import os
import re
import time
from pathlib import Path

from datasets import load_from_disk
from dotenv import load_dotenv
from neo4j import GraphDatabase
from openai import OpenAI

load_dotenv()

DATA_DIR = Path(__file__).parent / "data"
RESULTS_FILE = Path(__file__).parent / "results_kg_v2.json"

NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "password"

MODEL = "gpt-4o-mini"


def make_source_id(title: str, content: str) -> str:
    """Create source ID matching ingest_ai.py"""
    return "source:" + hashlib.sha256((title + content).encode()).hexdigest()[:16]


def get_ground_truth(example: dict) -> set[str]:
    """Get ground truth source IDs from HotpotQA."""
    relevant_titles = set(example["supporting_facts"]["title"])
    relevant_ids = set()

    for title, sents in zip(example["context"]["title"], example["context"]["sentences"]):
        if title in relevant_titles:
            content = " ".join(sents)
            relevant_ids.add(make_source_id(title, content))

    return relevant_ids


class KGRetrieverV2:
    """Improved KG retrieval with multi-hop and higher limits."""

    def __init__(self, driver, openai_client, entity_limit=20, hop1_limit=50, hop2_limit=100):
        self.driver = driver
        self.openai = openai_client
        self.entity_limit = entity_limit  # Max entities per name match
        self.hop1_limit = hop1_limit      # Max entities from first hop
        self.hop2_limit = hop2_limit      # Max entities from second hop

    def extract_entities(self, question: str) -> list[str]:
        """Extract entity names from question using LLM."""
        try:
            response = self.openai.chat.completions.create(
                model=MODEL,
                messages=[{
                    "role": "user",
                    "content": f"""Extract key named entities from this question.
Return a JSON array of entity names only. Example: ["Einstein", "Nobel Prize"]

Question: {question}"""
                }],
                max_tokens=150,
                temperature=0,
            )
            text = response.choices[0].message.content.strip()
            match = re.search(r'\[.*\]', text, re.DOTALL)
            if match:
                return json.loads(match.group())
        except:
            pass
        return []

    def find_entities(self, entity_name: str) -> list[str]:
        """Find entity nodes by name - higher limit."""
        with self.driver.session() as s:
            result = s.run("""
                MATCH (n:Node)
                WHERE n.type <> 'Source'
                AND toLower(n.properties) CONTAINS toLower($name)
                RETURN n.id as id
                LIMIT $limit
            """, name=entity_name, limit=self.entity_limit)
            return [r["id"] for r in result]

    def get_connected_entities_hop1(self, entity_ids: list[str]) -> list[str]:
        """First hop: Get entities via ANY edge type (ATTENDED + relationships)."""
        if not entity_ids:
            return []

        with self.driver.session() as s:
            # Follow both ATTENDED edges and explicit relationships
            result = s.run("""
                MATCH (e:Node)-[:LINK]-(related:Node)
                WHERE e.id IN $ids AND related.type <> 'Source'
                RETURN DISTINCT related.id as id
                LIMIT $limit
            """, ids=entity_ids, limit=self.hop1_limit)
            return [r["id"] for r in result]

    def get_connected_entities_hop2(self, entity_ids: list[str], exclude_ids: set[str]) -> list[str]:
        """Second hop: Get entities connected to hop1 entities."""
        if not entity_ids:
            return []

        with self.driver.session() as s:
            result = s.run("""
                MATCH (e:Node)-[:LINK]-(related:Node)
                WHERE e.id IN $ids
                AND related.type <> 'Source'
                AND NOT related.id IN $exclude
                RETURN DISTINCT related.id as id
                LIMIT $limit
            """, ids=entity_ids, exclude=list(exclude_ids), limit=self.hop2_limit)
            return [r["id"] for r in result]

    def get_sources(self, entity_ids: list[str]) -> set[str]:
        """Get source nodes linked to entities."""
        if not entity_ids:
            return set()

        with self.driver.session() as s:
            result = s.run("""
                MATCH (e:Node)-[:LINK {type: 'EXTRACTED_FROM'}]->(s:Node {type: 'Source'})
                WHERE e.id IN $ids
                RETURN DISTINCT s.id as id
            """, ids=entity_ids)
            return {r["id"] for r in result}

    def retrieve(self, question: str) -> set[str]:
        """Full retrieval pipeline with multi-hop."""
        # 1. Extract entities from question
        entities = self.extract_entities(question)
        if not entities:
            return set()

        # 2. Find matching nodes (seed entities)
        seed_ids = []
        for name in entities:
            seed_ids.extend(self.find_entities(name))

        if not seed_ids:
            return set()

        seed_set = set(seed_ids)

        # 3. First hop expansion
        hop1_ids = self.get_connected_entities_hop1(seed_ids)
        hop1_set = set(hop1_ids)

        # 4. Second hop expansion
        hop2_ids = self.get_connected_entities_hop2(hop1_ids, seed_set | hop1_set)

        # 5. Combine all entity IDs
        all_entity_ids = list(seed_set | hop1_set | set(hop2_ids))

        # 6. Get sources from all entities
        return self.get_sources(all_entity_ids)


def calc_metrics(retrieved: set, truth: set) -> dict:
    """Calculate P/R/F1."""
    if not retrieved or not truth:
        return {"precision": 0, "recall": 0, "f1": 0}

    tp = len(retrieved & truth)
    p = tp / len(retrieved)
    r = tp / len(truth)
    f1 = 2 * p * r / (p + r) if (p + r) > 0 else 0
    return {"precision": p, "recall": r, "f1": f1}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=100)
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--split", default="validation")
    parser.add_argument("--entity-limit", type=int, default=20, help="Max entities per name match")
    parser.add_argument("--hop1-limit", type=int, default=50, help="Max entities from hop 1")
    parser.add_argument("--hop2-limit", type=int, default=100, help="Max entities from hop 2")
    args = parser.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY not set")
        return

    print("Loading dataset...")
    ds = load_from_disk(str(DATA_DIR))
    dataset = ds[args.split]

    driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
    retriever = KGRetrieverV2(
        driver, OpenAI(),
        entity_limit=args.entity_limit,
        hop1_limit=args.hop1_limit,
        hop2_limit=args.hop2_limit
    )

    print(f"Config: entity_limit={args.entity_limit}, hop1_limit={args.hop1_limit}, hop2_limit={args.hop2_limit}")

    metrics = {"precision": 0, "recall": 0, "f1": 0, "count": 0}
    start_time = time.time()

    end_idx = min(args.start + args.limit, len(dataset))
    print(f"Testing {args.start} to {end_idx} ({end_idx - args.start} questions)")

    for i in range(args.start, end_idx):
        ex = dataset[i]
        truth = get_ground_truth(ex)
        if not truth:
            continue

        retrieved = retriever.retrieve(ex["question"])
        m = calc_metrics(retrieved, truth)

        metrics["precision"] += m["precision"]
        metrics["recall"] += m["recall"]
        metrics["f1"] += m["f1"]
        metrics["count"] += 1

        if (i - args.start + 1) % 20 == 0:
            n = metrics["count"]
            print(f"Progress: {i - args.start + 1}/{end_idx - args.start} | "
                  f"P: {metrics['precision']/n:.3f} R: {metrics['recall']/n:.3f} F1: {metrics['f1']/n:.3f}")

    driver.close()

    n = max(metrics["count"], 1)
    results = {
        "precision": metrics["precision"] / n,
        "recall": metrics["recall"] / n,
        "f1": metrics["f1"] / n,
        "questions": metrics["count"],
        "config": {
            "start": args.start,
            "limit": args.limit,
            "entity_limit": args.entity_limit,
            "hop1_limit": args.hop1_limit,
            "hop2_limit": args.hop2_limit,
        }
    }

    with open(RESULTS_FILE, "w") as f:
        json.dump(results, f, indent=2)

    elapsed = time.time() - start_time
    print(f"\n{'='*50}")
    print(f"KG V2 RESULTS ({results['questions']} questions)")
    print(f"{'='*50}")
    print(f"Entity limit: {args.entity_limit}, Hop1: {args.hop1_limit}, Hop2: {args.hop2_limit}")
    print(f"Precision: {results['precision']:.3f}")
    print(f"Recall:    {results['recall']:.3f}")
    print(f"F1:        {results['f1']:.3f}")
    print(f"Time:      {elapsed:.1f}s")
    print(f"Saved:     {RESULTS_FILE}")


if __name__ == "__main__":
    main()
