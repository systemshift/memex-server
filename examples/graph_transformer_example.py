#!/usr/bin/env python3
"""
Example: Converting Memex Subgraph to Graph Transformer Input

This demonstrates how to:
1. Query a subgraph from Memex
2. Convert to adjacency matrix
3. Prepare for graph transformer attention bias
"""

import requests
import numpy as np
from typing import Dict, List, Tuple


def fetch_subgraph(start_node: str, depth: int = 2) -> Dict:
    """Fetch subgraph from Memex API"""
    url = f"http://localhost:8080/api/query/subgraph"
    params = {"start": start_node, "depth": depth}
    response = requests.get(url, params=params)
    response.raise_for_status()
    return response.json()


def build_adjacency_matrix(subgraph: Dict) -> Tuple[np.ndarray, Dict[str, int], List[str]]:
    """
    Convert subgraph to adjacency matrix

    Returns:
        - adjacency_matrix: NxN binary matrix (1 = edge exists)
        - node_id_to_idx: mapping from node ID to matrix index
        - idx_to_node_id: list mapping index to node ID
    """
    nodes = subgraph["nodes"]
    edges = subgraph["edges"]

    # Create node ID to index mapping
    node_id_to_idx = {node["ID"]: idx for idx, node in enumerate(nodes)}
    idx_to_node_id = [node["ID"] for node in nodes]

    n = len(nodes)
    adjacency_matrix = np.zeros((n, n), dtype=np.float32)

    # Fill adjacency matrix
    for edge in edges:
        source_idx = node_id_to_idx[edge["source"]]
        target_idx = node_id_to_idx[edge["target"]]
        adjacency_matrix[source_idx, target_idx] = 1.0
        # For undirected graph, uncomment:
        # adjacency_matrix[target_idx, source_idx] = 1.0

    return adjacency_matrix, node_id_to_idx, idx_to_node_id


def build_edge_type_tensor(subgraph: Dict, node_id_to_idx: Dict[str, int],
                           edge_types: List[str]) -> np.ndarray:
    """
    Build edge type tensor for typed graph transformers

    Returns:
        - edge_type_matrix: NxNxE tensor (E = number of edge types)
    """
    edges = subgraph["edges"]
    n = len(node_id_to_idx)
    num_edge_types = len(edge_types)

    # Create edge type to index mapping
    type_to_idx = {t: idx for idx, t in enumerate(edge_types)}

    # Shape: (num_nodes, num_nodes, num_edge_types)
    edge_type_tensor = np.zeros((n, n, num_edge_types), dtype=np.float32)

    for edge in edges:
        source_idx = node_id_to_idx[edge["source"]]
        target_idx = node_id_to_idx[edge["target"]]
        edge_type = edge["type"]

        if edge_type in type_to_idx:
            type_idx = type_to_idx[edge_type]
            edge_type_tensor[source_idx, target_idx, type_idx] = 1.0

    return edge_type_tensor


def compute_attention_bias(adjacency_matrix: np.ndarray,
                           bias_strength: float = 1.0) -> np.ndarray:
    """
    Convert adjacency matrix to attention bias for transformer

    Graph structure biases attention:
    - Connected nodes: bias_strength (encourage attention)
    - Unconnected nodes: 0 (no bias)
    - Self-loops: always allowed

    This can be added to transformer attention logits before softmax.
    """
    n = adjacency_matrix.shape[0]

    # Start with adjacency structure
    bias = adjacency_matrix * bias_strength

    # Allow self-attention
    bias += np.eye(n) * bias_strength

    # Convert to attention mask format (can be added to logits)
    # For disallowing non-edges, use large negative value:
    # bias = np.where(adjacency_matrix > 0, 0.0, -1e9)

    return bias


def main():
    # Example: Fetch Python WikiPage neighborhood
    print("Fetching subgraph for Python WikiPage...")
    subgraph = fetch_subgraph("wiki:Python_(programming_language)", depth=2)

    print(f"Subgraph stats:")
    print(f"  Nodes: {subgraph['stats']['node_count']}")
    print(f"  Edges: {subgraph['stats']['edge_count']}")
    print()

    # Build adjacency matrix
    adj_matrix, node_map, node_list = build_adjacency_matrix(subgraph)
    print(f"Adjacency matrix shape: {adj_matrix.shape}")
    print(f"Density: {adj_matrix.sum() / (adj_matrix.shape[0] ** 2):.3f}")
    print()

    # Get unique edge types
    edge_types = sorted(set(edge["type"] for edge in subgraph["edges"]))
    print(f"Edge types: {edge_types}")
    print()

    # Build typed edge tensor
    edge_tensor = build_edge_type_tensor(subgraph, node_map, edge_types)
    print(f"Edge type tensor shape: {edge_tensor.shape}")
    print()

    # Compute attention bias
    attention_bias = compute_attention_bias(adj_matrix, bias_strength=2.0)
    print(f"Attention bias shape: {attention_bias.shape}")
    print(f"Bias range: [{attention_bias.min():.2f}, {attention_bias.max():.2f}]")
    print()

    # Example: Using with PyTorch transformer
    print("=" * 60)
    print("Usage with PyTorch Transformer:")
    print("=" * 60)
    print("""
import torch
import torch.nn as nn

# Convert to PyTorch tensors
adj_tensor = torch.from_numpy(adj_matrix)
bias_tensor = torch.from_numpy(attention_bias)

# In transformer attention layer:
class GraphBiasedAttention(nn.Module):
    def forward(self, query, key, value, graph_bias):
        # Standard attention logits
        attn_logits = torch.matmul(query, key.transpose(-2, -1))
        attn_logits = attn_logits / math.sqrt(query.size(-1))

        # Add graph structure bias
        attn_logits = attn_logits + graph_bias

        # Softmax + apply to values
        attn_weights = torch.softmax(attn_logits, dim=-1)
        return torch.matmul(attn_weights, value)
    """)

    # Sample node features (in real use, extract from node content)
    print("\nNode feature extraction (TODO):")
    for i, node in enumerate(subgraph["nodes"][:3]):
        print(f"  Node {i}: {node['ID']} (type: {node['Type']})")
        # In practice: embed node content/metadata
        # features[i] = embed_node(node)


if __name__ == "__main__":
    main()
