#!/usr/bin/env python3
"""
Demo: Attention-Aware DAG for Sparse Retrieval

This demonstrates the attention edge API:
1. ML pipeline computes attention weights
2. Persists attention patterns to Memex DAG
3. Future queries use DAG to retrieve sparse subgraphs
4. DAG learns which nodes are frequently co-attended
"""

import requests
import hashlib
from typing import List, Tuple

MEMEX_URL = "http://localhost:8080"


def simulate_attention_computation(query: str, nodes: List[str]) -> List[Tuple[str, str, float]]:
    """
    Simulate ML model computing attention between nodes

    In reality, this would be:
    - Get node content
    - Embed with transformer
    - Compute attention logits
    - Return high-weight pairs
    """
    # Fake attention weights for demo
    return [
        ("wiki:Python_(programming_language)", "python", 0.92),
        ("wiki:Python_(programming_language)", "guido", 0.88),
        ("python", "guido", 0.75),
    ]


def update_attention_dag(query_id: str, attention_pairs: List[Tuple[str, str, float]]):
    """
    Persist attention patterns to Memex DAG

    This makes the DAG "learn" which nodes are frequently co-attended.
    """
    print(f"\\nUpdating DAG with {len(attention_pairs)} attention edges...")

    for source, target, weight in attention_pairs:
        response = requests.post(
            f"{MEMEX_URL}/api/edges/attention",
            json={
                "source": source,
                "target": target,
                "query_id": query_id,
                "weight": weight
            }
        )

        if response.status_code == 200:
            print(f"  ✓ {source} → {target} (weight: {weight:.2f})")
        else:
            print(f"  ✗ Failed: {response.text}")


def query_with_attention_dag(query: str, start_node: str, min_weight: float = 0.7) -> dict:
    """
    Query using learned attention patterns from DAG

    Instead of full O(n²) attention, use sparse DAG structure
    to retrieve only high-attention nodes.
    """
    print(f"\\nQuerying attention DAG from '{start_node}' (min_weight={min_weight})...")

    response = requests.get(
        f"{MEMEX_URL}/api/query/attention_subgraph",
        params={
            "start": start_node,
            "min_weight": min_weight,
            "max_nodes": 50
        }
    )

    if response.status_code == 200:
        subgraph = response.json()
        print(f"  Retrieved {subgraph['stats']['node_count']} nodes, {subgraph['stats']['edge_count']} edges")
        return subgraph
    else:
        print(f"  Error: {response.text}")
        return {}


def demonstrate_dag_learning():
    """
    Show how DAG learns from multiple queries
    """
    print("=" * 60)
    print("Attention-Aware DAG Demo")
    print("=" * 60)

    # Simulate 3 queries with similar attention patterns
    queries = [
        "Who created Python?",
        "Tell me about Python's creator",
        "Python design philosophy"
    ]

    for i, query in enumerate(queries, 1):
        query_id = hashlib.sha256(query.encode()).hexdigest()[:16]

        print(f"\\n--- Query {i}: {query} ---")

        # Step 1: ML model computes attention (simulated)
        attention_pairs = simulate_attention_computation(query, ["wiki:Python_(programming_language)", "python", "guido"])

        # Step 2: Persist to DAG
        update_attention_dag(query_id, attention_pairs)

    # Step 3: Show how DAG has learned the pattern
    print("\\n" + "=" * 60)
    print("DAG Learning Summary")
    print("=" * 60)

    # Query the attention subgraph
    subgraph = query_with_attention_dag(
        "New query about Python",
        start_node="wiki:Python_(programming_language)",
        min_weight=0.7
    )

    if subgraph and subgraph.get('edges'):
        print("\\nLearned attention patterns:")
        for edge in subgraph['edges']:
            weight = edge['meta'].get('weight', 0)
            count = edge['meta'].get('query_count', 0)
            print(f"  {edge['source']} → {edge['target']}")
            print(f"    weight: {weight:.3f}, reinforced {count} times")

    print("\\n" + "=" * 60)
    print("Benefits:")
    print("=" * 60)
    print("  ✓ DAG learns which nodes co-occur in attention")
    print("  ✓ Future queries use sparse retrieval (not full O(n²))")
    print("  ✓ Patterns persist across model changes")
    print("  ✓ Reinforcement from repeated queries")


def demonstrate_pruning():
    """
    Show how to prune weak attention edges
    """
    print("\\n" + "=" * 60)
    print("Pruning Weak Edges")
    print("=" * 60)

    response = requests.post(
        f"{MEMEX_URL}/api/edges/attention/prune",
        params={
            "min_weight": 0.3,      # Remove edges with weight < 0.3
            "min_query_count": 2     # Remove edges seen only once
        }
    )

    if response.status_code == 200:
        result = response.json()
        print(f"  Pruned {result['deleted_count']} weak edges")
    else:
        print(f"  Error: {response.text}")


if __name__ == "__main__":
    demonstrate_dag_learning()
    # demonstrate_pruning()  # Uncomment to test pruning
