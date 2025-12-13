#!/usr/bin/env python3
"""
Memex SDPA Bias Injection - Production-ready module for injecting
memex graph structure into transformer attention via SDPA patching.

Usage:
    from memex_sdpa import MemexSDPA, load_graph_from_neo4j

    # Load graph
    graph = load_graph_from_neo4j("bolt://localhost:7687")
    # or: graph = load_graph_from_file("graph.json")

    # Create patcher
    memex = MemexSDPA(graph)

    # Use as context manager
    with memex.patch():
        output = model.generate(...)
"""

import json
import re
from dataclasses import dataclass
from typing import Optional
import torch
import torch.nn.functional as F


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


class MemexGraph:
    """Loaded memex graph with entity relationships."""

    def __init__(self):
        self.entities: dict[str, GraphEntity] = {}
        self.weights: dict[str, float] = {}  # "id1|id2" -> weight
        self._name_to_id: dict[str, str] = {}  # lowercase name -> entity id

    def add_entity(self, entity: GraphEntity):
        self.entities[entity.id] = entity
        for alias in entity.aliases:
            self._name_to_id[alias.lower()] = entity.id

    def add_edge(self, source: str, target: str, weight: float):
        """Add bidirectional edge."""
        self.weights[f"{source}|{target}"] = weight
        self.weights[f"{target}|{source}"] = weight

    def get_weight(self, id1: str, id2: str) -> float:
        return self.weights.get(f"{id1}|{id2}", 0.0)

    def find_entity_by_name(self, name: str) -> Optional[str]:
        return self._name_to_id.get(name.lower())

    def get_all_names(self) -> list[str]:
        return list(self._name_to_id.keys())


def load_graph_from_neo4j(uri: str = "bolt://localhost:7687",
                          user: str = "neo4j",
                          password: str = "password",
                          min_weight: float = 0.1) -> MemexGraph:
    """Load graph from Neo4j including ATTENDED edges."""
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

        # Load ATTENDED edges
        result = session.run("""
            MATCH (a:Node)-[r:LINK]->(b:Node)
            WHERE r.type = 'ATTENDED' AND r.properties IS NOT NULL
            RETURN a.id as source, b.id as target, r.properties as props
        """)
        for record in result:
            props = json.loads(record["props"]) if record["props"] else {}
            weight = props.get("weight", 1.0)
            if weight >= min_weight:
                graph.add_edge(record["source"], record["target"], weight)

        # Also load regular relationship edges as weaker connections
        result = session.run("""
            MATCH (a:Node)-[r:LINK]->(b:Node)
            WHERE r.type <> 'ATTENDED' AND a.type <> 'Source' AND b.type <> 'Source'
            RETURN DISTINCT a.id as source, b.id as target
        """)
        for record in result:
            key = f"{record['source']}|{record['target']}"
            if key not in graph.weights:
                graph.add_edge(record["source"], record["target"], 0.5)

    driver.close()
    print(f"Loaded {len(graph.entities)} entities, {len(graph.weights)//2} edges from Neo4j")
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
        graph.add_edge(source, target, weight)

    print(f"Loaded {len(graph.entities)} entities, {len(graph.weights)//2} edges from file")
    return graph


class MemexSDPA:
    """Main class for SDPA-based memex bias injection."""

    def __init__(self, graph: MemexGraph, bias_scale: float = 1.0):
        self.graph = graph
        self.bias_scale = bias_scale
        self._bias_matrix: Optional[torch.Tensor] = None
        self._entity_pattern: Optional[re.Pattern] = None
        self._build_entity_pattern()

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

        # Build bias matrix
        bias = torch.zeros(seq_len, seq_len)
        entities = list(entity_tokens.keys())

        for i, e1 in enumerate(entities):
            for e2 in entities[i+1:]:
                weight = self.graph.get_weight(e1, e2) * self.bias_scale
                if weight > 0:
                    for t1 in entity_tokens[e1]:
                        for t2 in entity_tokens[e2]:
                            if t1 < seq_len and t2 < seq_len:
                                bias[t1, t2] = weight
                                bias[t2, t1] = weight

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
    print("  memex = MemexSDPA(graph, bias_scale=1.5)")
    print()
    print("  with memex.patch(text, tokenizer):")
    print("      output = model.generate(...)")
