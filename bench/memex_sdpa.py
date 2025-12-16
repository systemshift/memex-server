#!/usr/bin/env python3
"""
Memex SDPA Bias Injection - Production-ready module for injecting
memex graph structure into transformer attention via SDPA patching.

Usage:
    from memex_sdpa import MemexSDPA, load_graph_from_neo4j

    # Load graph
    graph = load_graph_from_neo4j("bolt://localhost:7687")
    # or: graph = load_graph_from_file("graph.json")

    # Create patcher with adaptive bias (uses graph's own statistics)
    memex = MemexSDPA(graph, bias_mode="adaptive")

    # Use as context manager
    with memex.patch():
        output = model.generate(...)

Bias Modes:
    - "adaptive": Auto-scale based on graph statistics (recommended)
    - "confidence": Weight by query_count (higher count = more confident)
    - "raw": Use edge weights directly (0.0-1.0)
    - "fixed": Multiply all weights by bias_scale (legacy)
"""

import json
import math
import re
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional
import torch
import torch.nn.functional as F


class BiasMode(Enum):
    """Bias scaling modes."""
    RAW = "raw"              # Use edge weights directly
    FIXED = "fixed"          # Multiply by constant bias_scale
    CONFIDENCE = "confidence"  # Scale by query_count confidence
    ADAPTIVE = "adaptive"    # Auto-scale based on graph statistics


# Store original SDPA
_original_sdpa = F.scaled_dot_product_attention
_active_bias = None


def _patched_sdpa(query, key, value, attn_mask=None, dropout_p=0.0, is_causal=False, scale=None, **kwargs):
    """SDPA with memex bias injection."""
    global _active_bias

    if _active_bias is not None:
        batch, heads, q_len, head_dim = query.shape
        _, _, k_len, _ = key.shape

        # Create fresh mask tensor
        new_mask = torch.zeros(batch, heads, q_len, k_len,
                               device=query.device, dtype=query.dtype)

        # If causal, create causal mask (lower triangular)
        if is_causal:
            # Create causal mask: -inf for positions that should be masked
            causal_mask = torch.triu(
                torch.ones(q_len, k_len, device=query.device, dtype=query.dtype) * float('-inf'),
                diagonal=1
            )
            new_mask += causal_mask.unsqueeze(0).unsqueeze(0)
            is_causal = False  # We handle causality manually now

        # Copy existing mask if present
        if attn_mask is not None:
            if attn_mask.dim() == 2:
                new_mask += attn_mask.unsqueeze(0).unsqueeze(0)
            elif attn_mask.dim() == 3:
                new_mask += attn_mask.unsqueeze(1)
            elif attn_mask.shape[-2:] == (q_len, k_len):
                new_mask += attn_mask.expand(batch, heads, q_len, k_len)

        # Add memex bias
        bias_q = min(q_len, _active_bias.shape[0])
        bias_k = min(k_len, _active_bias.shape[1])
        new_mask[:, :, :bias_q, :bias_k] += _active_bias[:bias_q, :bias_k].to(query.dtype)

        attn_mask = new_mask

    return _original_sdpa(query, key, value, attn_mask=attn_mask, dropout_p=dropout_p,
                          is_causal=is_causal, scale=scale, **kwargs)


@dataclass
class GraphEntity:
    """Entity from memex graph."""
    id: str
    name: str
    type: str = "Entity"
    aliases: list = None

    def __post_init__(self):
        if self.aliases is None:
            self.aliases = [self.name.lower()]


@dataclass
class EdgeData:
    """Edge data with weight and confidence."""
    weight: float
    query_count: int = 1

    def confidence_score(self) -> float:
        """Confidence increases logarithmically with query_count."""
        return math.log(self.query_count + 1)

    def weighted_score(self) -> float:
        """Weight scaled by confidence."""
        return self.weight * self.confidence_score()


class MemexGraph:
    """Loaded memex graph with entity relationships."""

    def __init__(self):
        self.entities: dict[str, GraphEntity] = {}
        self.edges: dict[str, EdgeData] = {}  # "id1|id2" -> EdgeData
        self._name_to_id: dict[str, str] = {}  # lowercase name -> entity id
        self._stats_cache: Optional[dict] = None

    def add_entity(self, entity: GraphEntity):
        self.entities[entity.id] = entity
        for alias in entity.aliases:
            self._name_to_id[alias.lower()] = entity.id

    def add_edge(self, source: str, target: str, weight: float, query_count: int = 1):
        """Add bidirectional edge with weight and confidence."""
        edge_data = EdgeData(weight=weight, query_count=query_count)
        self.edges[f"{source}|{target}"] = edge_data
        self.edges[f"{target}|{source}"] = edge_data
        self._stats_cache = None  # Invalidate cache

    def get_edge(self, id1: str, id2: str) -> Optional[EdgeData]:
        return self.edges.get(f"{id1}|{id2}")

    def get_weight(self, id1: str, id2: str) -> float:
        edge = self.get_edge(id1, id2)
        return edge.weight if edge else 0.0

    def find_entity_by_name(self, name: str) -> Optional[str]:
        return self._name_to_id.get(name.lower())

    def get_all_names(self) -> list[str]:
        return list(self._name_to_id.keys())

    def compute_stats(self) -> dict:
        """Compute graph statistics for adaptive scaling."""
        if self._stats_cache:
            return self._stats_cache

        if not self.edges:
            return {"mean": 0.5, "std": 0.2, "min": 0, "max": 1, "count": 0}

        weights = [e.weight for e in self.edges.values()]
        query_counts = [e.query_count for e in self.edges.values()]

        mean_w = sum(weights) / len(weights)
        var_w = sum((w - mean_w) ** 2 for w in weights) / len(weights)
        std_w = math.sqrt(var_w) if var_w > 0 else 0.1

        self._stats_cache = {
            "mean": mean_w,
            "std": std_w,
            "min": min(weights),
            "max": max(weights),
            "count": len(weights) // 2,  # Bidirectional edges counted twice
            "mean_query_count": sum(query_counts) / len(query_counts),
            "max_query_count": max(query_counts),
        }
        return self._stats_cache

    def get_adaptive_scale(self) -> float:
        """
        Compute optimal bias scale based on graph statistics.

        The idea: we want edge weights to produce meaningful attention bias
        without overwhelming the model's natural attention patterns.

        Based on empirical testing:
        - bias ~0.5 works well for strong edges
        - bias >2.0 tends to hurt performance
        """
        stats = self.compute_stats()

        if stats["count"] == 0:
            return 0.5

        # Target: strongest edges should have effective bias ~0.5
        # This is based on empirical results showing scale=0.5 performs best
        target_bias = 0.5
        max_weight = stats["max"]

        if max_weight > 0:
            scale = target_bias / max_weight
        else:
            scale = 0.5

        # Clamp to reasonable range (0.3 to 1.5)
        return max(0.3, min(scale, 1.5))


def load_graph_from_neo4j(uri: str = "bolt://localhost:7687",
                          user: str = "neo4j",
                          password: str = "password",
                          min_weight: float = 0.1) -> MemexGraph:
    """Load graph from Neo4j including ATTENDED edges with query_count."""
    from neo4j import GraphDatabase

    driver = GraphDatabase.driver(uri, auth=(user, password))
    graph = MemexGraph()

    with driver.session() as session:
        # Load entities
        result = session.run("""
            MATCH (n:Node)
            WHERE n.type <> 'Source'
            RETURN n.id as id, n.type as type, n.properties as props
        """)
        for record in result:
            props = json.loads(record["props"]) if record["props"] else {}
            name = props.get("name", record["id"])
            entity = GraphEntity(
                id=record["id"],
                name=name,
                type=record["type"] or "Entity"
            )
            graph.add_entity(entity)

        # Load ATTENDED edges with weight and query_count
        result = session.run("""
            MATCH (a:Node)-[r:LINK]->(b:Node)
            WHERE r.type = 'ATTENDED' AND r.properties IS NOT NULL
            RETURN a.id as source, b.id as target, r.properties as props
        """)
        for record in result:
            props = json.loads(record["props"]) if record["props"] else {}
            weight = props.get("weight", 1.0)
            query_count = props.get("query_count", 1)
            if weight >= min_weight:
                graph.add_edge(record["source"], record["target"], weight, query_count)

        # Also load regular relationship edges as weaker connections (low confidence)
        result = session.run("""
            MATCH (a:Node)-[r:LINK]->(b:Node)
            WHERE r.type <> 'ATTENDED' AND a.type <> 'Source' AND b.type <> 'Source'
            RETURN DISTINCT a.id as source, b.id as target
        """)
        for record in result:
            key = f"{record['source']}|{record['target']}"
            if key not in graph.edges:
                # Default weight 0.5, query_count 1 (low confidence)
                graph.add_edge(record["source"], record["target"], 0.5, 1)

    driver.close()
    stats = graph.compute_stats()
    print(f"Loaded {len(graph.entities)} entities, {stats['count']} edges from Neo4j")
    print(f"  Weight stats: mean={stats['mean']:.2f}, max={stats['max']:.2f}")
    print(f"  Query count: mean={stats['mean_query_count']:.1f}, max={stats['max_query_count']}")
    print(f"  Adaptive scale: {graph.get_adaptive_scale():.2f}")
    return graph


def load_graph_from_file(path: str) -> MemexGraph:
    """Load graph from JSON file (memex subgraph export format)."""
    with open(path) as f:
        data = json.load(f)

    graph = MemexGraph()

    for node in data.get("nodes", []):
        entity_id = node.get("ID") or node.get("id")
        props = node.get("properties", {})
        if isinstance(props, str):
            props = json.loads(props)

        name = props.get("name") or node.get("name") or entity_id
        entity = GraphEntity(
            id=entity_id,
            name=name,
            type=node.get("Type") or node.get("type", "Entity")
        )
        graph.add_entity(entity)

    for edge in data.get("edges", []):
        source = edge.get("source")
        target = edge.get("target")
        meta = edge.get("meta", {})
        weight = edge.get("weight") or meta.get("weight", 1.0)
        query_count = meta.get("query_count", 1)
        graph.add_edge(source, target, weight, query_count)

    stats = graph.compute_stats()
    print(f"Loaded {len(graph.entities)} entities, {stats['count']} edges from file")
    if stats['count'] > 0:
        print(f"  Adaptive scale: {graph.get_adaptive_scale():.2f}")
    return graph


class MemexSDPA:
    """Main class for SDPA-based memex bias injection."""

    def __init__(self, graph: MemexGraph,
                 bias_mode: str = "adaptive",
                 bias_scale: float = 1.0):
        """
        Initialize memex SDPA bias injector.

        Args:
            graph: Loaded MemexGraph with entities and edges
            bias_mode: How to scale bias values:
                - "adaptive": Auto-scale based on graph statistics (recommended)
                - "confidence": Scale by query_count (more queries = stronger bias)
                - "raw": Use edge weights directly (0.0-1.0)
                - "fixed": Multiply all weights by bias_scale
            bias_scale: Only used when bias_mode="fixed"
        """
        self.graph = graph
        self.bias_mode = BiasMode(bias_mode)
        self.bias_scale = bias_scale
        self._bias_matrix: Optional[torch.Tensor] = None
        self._entity_pattern: Optional[re.Pattern] = None
        self._build_entity_pattern()

        # Print mode info
        if self.bias_mode == BiasMode.ADAPTIVE:
            print(f"Bias mode: adaptive (auto-scale={graph.get_adaptive_scale():.2f})")
        elif self.bias_mode == BiasMode.CONFIDENCE:
            print("Bias mode: confidence (weight * log(query_count + 1))")
        elif self.bias_mode == BiasMode.RAW:
            print("Bias mode: raw (using edge weights directly)")
        else:
            print(f"Bias mode: fixed (scale={bias_scale})")

    def _build_entity_pattern(self):
        """Build regex pattern for entity detection."""
        names = self.graph.get_all_names()
        if names:
            names.sort(key=len, reverse=True)
            escaped = [re.escape(n) for n in names]
            self._entity_pattern = re.compile(
                r'\b(' + '|'.join(escaped) + r')\b',
                re.IGNORECASE
            )

    def find_entities_in_text(self, text: str) -> dict[str, list[tuple[int, int]]]:
        """Find entity mentions and their character positions."""
        if not self._entity_pattern:
            return {}

        entities = {}
        for match in self._entity_pattern.finditer(text):
            name = match.group(1).lower()
            entity_id = self.graph.find_entity_by_name(name)
            if entity_id:
                if entity_id not in entities:
                    entities[entity_id] = []
                entities[entity_id].append((match.start(), match.end()))

        return entities

    def _compute_edge_bias(self, e1: str, e2: str) -> float:
        """Compute bias value for an edge based on current mode."""
        edge = self.graph.get_edge(e1, e2)
        if not edge:
            return 0.0

        if self.bias_mode == BiasMode.RAW:
            return edge.weight

        elif self.bias_mode == BiasMode.FIXED:
            return edge.weight * self.bias_scale

        elif self.bias_mode == BiasMode.CONFIDENCE:
            # Weight scaled by confidence (log of query count)
            return edge.weighted_score()

        elif self.bias_mode == BiasMode.ADAPTIVE:
            # Use graph's computed adaptive scale
            scale = self.graph.get_adaptive_scale()
            return edge.weight * scale

        return edge.weight

    def build_bias_matrix(self, text: str, tokenizer, max_len: int = 512) -> torch.Tensor:
        """Build attention bias matrix for given text."""
        # Find entities
        entity_spans = self.find_entities_in_text(text)

        # Tokenize with offsets
        encoding = tokenizer(text, return_offsets_mapping=True, truncation=True, max_length=max_len)
        offset_mapping = encoding.get("offset_mapping", [])
        seq_len = len(encoding["input_ids"])

        # Map entities to token positions
        entity_tokens = {}
        for entity_id, char_spans in entity_spans.items():
            token_indices = []
            for char_start, char_end in char_spans:
                for tok_idx, (tok_start, tok_end) in enumerate(offset_mapping):
                    if tok_end > char_start and tok_start < char_end:
                        token_indices.append(tok_idx)
            if token_indices:
                entity_tokens[entity_id] = list(set(token_indices))

        # Build bias matrix using mode-specific computation
        bias = torch.zeros(seq_len, seq_len)
        entities = list(entity_tokens.keys())

        for i, e1 in enumerate(entities):
            for e2 in entities[i+1:]:
                bias_value = self._compute_edge_bias(e1, e2)
                if bias_value > 0:
                    for t1 in entity_tokens[e1]:
                        for t2 in entity_tokens[e2]:
                            if t1 < seq_len and t2 < seq_len:
                                bias[t1, t2] = bias_value
                                bias[t2, t1] = bias_value

        return bias

    def patch(self, text: str = None, tokenizer=None, bias_matrix: torch.Tensor = None):
        """Return context manager that patches SDPA with memex bias."""
        return _SDPAPatchContext(self, text, tokenizer, bias_matrix)


class _SDPAPatchContext:
    """Context manager for SDPA patching."""

    def __init__(self, memex: MemexSDPA, text: str, tokenizer, bias_matrix: torch.Tensor):
        self.memex = memex
        self.text = text
        self.tokenizer = tokenizer
        self.bias_matrix = bias_matrix

    def __enter__(self):
        global _active_bias

        if self.bias_matrix is not None:
            _active_bias = self.bias_matrix
        elif self.text and self.tokenizer:
            _active_bias = self.memex.build_bias_matrix(self.text, self.tokenizer)
        else:
            raise ValueError("Must provide either bias_matrix or (text, tokenizer)")

        # Patch SDPA
        F.scaled_dot_product_attention = _patched_sdpa
        return self

    def __exit__(self, *args):
        global _active_bias
        _active_bias = None
        F.scaled_dot_product_attention = _original_sdpa


def set_global_bias(bias: torch.Tensor):
    """Set global bias matrix (for manual control)."""
    global _active_bias
    _active_bias = bias
    F.scaled_dot_product_attention = _patched_sdpa


def clear_global_bias():
    """Clear global bias and restore original SDPA."""
    global _active_bias
    _active_bias = None
    F.scaled_dot_product_attention = _original_sdpa


if __name__ == "__main__":
    print("Memex SDPA Bias Injection Module")
    print()
    print("Usage:")
    print("  from memex_sdpa import MemexSDPA, load_graph_from_neo4j")
    print()
    print("  graph = load_graph_from_neo4j()")
    print()
    print("  # Recommended: adaptive mode (auto-scales based on graph)")
    print("  memex = MemexSDPA(graph, bias_mode='adaptive')")
    print()
    print("  # Or: confidence mode (weights by query_count)")
    print("  memex = MemexSDPA(graph, bias_mode='confidence')")
    print()
    print("  # Or: fixed mode with manual scale (legacy)")
    print("  memex = MemexSDPA(graph, bias_mode='fixed', bias_scale=2.0)")
    print()
    print("  with memex.patch(text, tokenizer):")
    print("      output = model.generate(...)")
    print()
    print("Bias Modes:")
    print("  adaptive   - Auto-scale based on graph statistics (recommended)")
    print("  confidence - Scale by query_count (learned edges get stronger)")
    print("  raw        - Use edge weights directly (0.0-1.0)")
    print("  fixed      - Multiply all by bias_scale constant")
