#!/usr/bin/env python3
"""
MemexBiasAdapter - Converts memex subgraph exports to attention bias tensors.

This is the bridge between memex knowledge graphs and transformer attention layers.
Model developers use this to apply memex attention bias to any supported model.

Usage:
    from memex_bias_adapter import MemexBiasAdapter

    adapter = MemexBiasAdapter("knowledge_graph.json")
    adapter.register(model)
    output = model.generate("Who founded Alice's company?")
"""

import json
import logging
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, Callable

import torch

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(levelname)s] %(message)s')
logger = logging.getLogger(__name__)


@dataclass
class MemexConfig:
    """Configuration for memex bias adapter."""
    # Bias computation
    default_bias: float = 0.0  # Bias for unconnected entities
    bias_scale: float = 2.0    # Multiplier for bias values
    min_weight: float = 0.1    # Ignore edges below this weight

    # Entity matching
    case_sensitive: bool = False
    match_partial: bool = True  # Match "Alice" in "Alice Smith"

    # Model settings
    apply_to_layers: str = "all"  # "all", "last", "first", or "0,1,2"
    apply_to_heads: str = "all"   # "all" or "0,1,2"


class MemexGraph:
    """
    Parsed memex subgraph with entity lookup.

    This holds the knowledge graph structure in a format
    optimized for building attention bias matrices.
    """

    def __init__(self, config: MemexConfig = None):
        self.config = config or MemexConfig()
        self.entities: dict[str, dict] = {}  # entity_id -> {name, type, ...}
        self.aliases: dict[str, str] = {}    # lowercase name -> entity_id
        self.weights: dict[str, float] = {}  # "entity_a|entity_b" -> weight

    def load_from_file(self, path: str):
        """Load memex subgraph export from JSON file."""
        with open(path) as f:
            data = json.load(f)
        self.load_from_dict(data)

    def load_from_dict(self, data: dict):
        """Load from memex subgraph response format."""
        # Parse nodes
        for node in data.get("nodes", []):
            entity_id = node.get("ID") or node.get("id")
            if not entity_id:
                continue

            self.entities[entity_id] = {
                "id": entity_id,
                "type": node.get("Type") or node.get("type"),
                "name": self._extract_name(node),
            }

            # Build aliases for matching
            name = self.entities[entity_id]["name"]
            if name:
                key = name if self.config.case_sensitive else name.lower()
                self.aliases[key] = entity_id

        # Parse edges (attention weights)
        for edge in data.get("edges", []):
            source = edge.get("source")
            target = edge.get("target")

            # Get weight from meta or direct
            weight = edge.get("weight")
            if weight is None:
                meta = edge.get("meta", {})
                weight = meta.get("weight", 1.0)

            if weight < self.config.min_weight:
                continue

            # Store bidirectionally
            key1 = f"{source}|{target}"
            key2 = f"{target}|{source}"
            self.weights[key1] = weight
            self.weights[key2] = weight

        logger.info(f"Loaded {len(self.entities)} entities, {len(self.weights)//2} edges")

    def _extract_name(self, node: dict) -> str:
        """Extract display name from node."""
        # Try common locations
        if "name" in node:
            return node["name"]
        if "Name" in node:
            return node["Name"]

        # Try properties
        props = node.get("properties") or node.get("Properties") or {}
        if isinstance(props, str):
            try:
                props = json.loads(props)
            except:
                props = {}

        if "name" in props:
            return props["name"]

        # Fall back to ID
        return node.get("ID") or node.get("id", "")

    def get_weight(self, entity_a: str, entity_b: str) -> float:
        """Get attention weight between two entities."""
        key = f"{entity_a}|{entity_b}"
        return self.weights.get(key, self.config.default_bias)

    def find_entity(self, text: str) -> Optional[str]:
        """Find entity ID matching text."""
        key = text if self.config.case_sensitive else text.lower()
        return self.aliases.get(key)


class EntityMatcher:
    """
    Finds entity mentions in text and maps to token positions.

    This is the critical bridge: text -> entities -> tokens
    """

    def __init__(self, graph: MemexGraph, tokenizer):
        self.graph = graph
        self.tokenizer = tokenizer

        # Build regex pattern for all entity names
        names = list(graph.aliases.keys())
        if names:
            # Sort by length (longest first) for greedy matching
            names.sort(key=len, reverse=True)
            # Escape special regex chars
            escaped = [re.escape(n) for n in names]
            self.pattern = re.compile(r'\b(' + '|'.join(escaped) + r')\b',
                                      re.IGNORECASE if not graph.config.case_sensitive else 0)
        else:
            self.pattern = None

    def find_entities_in_text(self, text: str) -> dict[str, list[tuple[int, int]]]:
        """
        Find all entity mentions in text.

        Returns:
            dict mapping entity_id -> list of (start_char, end_char) spans
        """
        if not self.pattern:
            return {}

        entities = {}
        for match in self.pattern.finditer(text):
            name = match.group(1)
            key = name if self.graph.config.case_sensitive else name.lower()
            entity_id = self.graph.aliases.get(key)

            if entity_id:
                if entity_id not in entities:
                    entities[entity_id] = []
                entities[entity_id].append((match.start(), match.end()))

        return entities

    def map_chars_to_tokens(self, text: str, char_spans: list[tuple[int, int]]) -> list[list[int]]:
        """
        Map character spans to token positions.

        This handles the tricky alignment between characters and BPE tokens.
        """
        # Tokenize with offset mapping if available
        encoding = self.tokenizer(text, return_offsets_mapping=True, return_tensors="pt")

        if "offset_mapping" in encoding:
            offset_mapping = encoding["offset_mapping"][0].tolist()
        else:
            # Fallback: approximate mapping
            tokens = self.tokenizer.encode(text)
            # Rough character-to-token ratio
            ratio = len(tokens) / max(len(text), 1)
            return [[int(start * ratio), int(end * ratio)] for start, end in char_spans]

        token_spans = []
        for char_start, char_end in char_spans:
            token_indices = []
            for tok_idx, (tok_start, tok_end) in enumerate(offset_mapping):
                # Token overlaps with character span
                if tok_end > char_start and tok_start < char_end:
                    token_indices.append(tok_idx)
            token_spans.append(token_indices)

        return token_spans

    def get_entity_token_positions(self, text: str) -> dict[str, list[int]]:
        """
        Get token positions for all entities in text.

        Returns:
            dict mapping entity_id -> list of token indices
        """
        # Find entities
        entity_spans = self.find_entities_in_text(text)

        # Flatten all spans for batch mapping
        all_spans = []
        span_to_entity = []
        for entity_id, spans in entity_spans.items():
            for span in spans:
                all_spans.append(span)
                span_to_entity.append(entity_id)

        if not all_spans:
            return {}

        # Map to tokens
        token_spans = self.map_chars_to_tokens(text, all_spans)

        # Rebuild entity -> tokens mapping
        entity_tokens = {}
        for entity_id, token_indices in zip(span_to_entity, token_spans):
            if entity_id not in entity_tokens:
                entity_tokens[entity_id] = []
            entity_tokens[entity_id].extend(token_indices)

        # Deduplicate
        for entity_id in entity_tokens:
            entity_tokens[entity_id] = list(set(entity_tokens[entity_id]))

        return entity_tokens


class BiasMatrixBuilder:
    """
    Builds attention bias tensors from entity positions and graph weights.
    """

    def __init__(self, graph: MemexGraph):
        self.graph = graph

    def build(self, entity_positions: dict[str, list[int]], seq_len: int) -> torch.Tensor:
        """
        Build attention bias matrix.

        Args:
            entity_positions: entity_id -> list of token positions
            seq_len: total sequence length

        Returns:
            Tensor of shape (seq_len, seq_len) with bias values
        """
        bias = torch.zeros(seq_len, seq_len)

        entities = list(entity_positions.keys())

        for i, entity_a in enumerate(entities):
            for entity_b in entities[i+1:]:
                # Get weight from graph
                weight = self.graph.get_weight(entity_a, entity_b)

                if weight <= 0:
                    continue

                # Apply to all token pairs between these entities
                scaled_weight = weight * self.graph.config.bias_scale

                for pos_a in entity_positions[entity_a]:
                    for pos_b in entity_positions[entity_b]:
                        if pos_a < seq_len and pos_b < seq_len:
                            bias[pos_a, pos_b] = scaled_weight
                            bias[pos_b, pos_a] = scaled_weight

        return bias


class ModelHooks:
    """
    Attention hooks for different model architectures.

    Supports: GPT-2, Llama, Mistral, and compatible architectures.
    """

    def __init__(self, bias_matrix: torch.Tensor, config: MemexConfig):
        self.bias_matrix = bias_matrix
        self.config = config
        self.handles = []
        self.active = True

    def _get_bias_for_seq(self, seq_len: int, device, dtype) -> torch.Tensor:
        """Get bias matrix sized and typed for current sequence."""
        if seq_len <= self.bias_matrix.shape[0]:
            bias = self.bias_matrix[:seq_len, :seq_len].clone()
        else:
            # Pad with zeros if sequence is longer
            bias = torch.zeros(seq_len, seq_len)
            orig_len = self.bias_matrix.shape[0]
            bias[:orig_len, :orig_len] = self.bias_matrix

        return bias.to(device=device, dtype=dtype).contiguous()

    def _hook_gpt2(self, module, args, kwargs):
        """Hook for GPT-2 style attention (with kwargs support)."""
        if not self.active:
            return args, kwargs

        # Get hidden states from args or kwargs
        if args:
            hidden_states = args[0]
        else:
            hidden_states = kwargs.get("hidden_states")

        if hidden_states is None:
            return args, kwargs

        seq_len = hidden_states.shape[1]

        # Get attention mask from kwargs (modern transformers style)
        attention_mask = kwargs.get("attention_mask")

        if attention_mask is None:
            attention_mask = torch.zeros(
                1, 1, seq_len, seq_len,
                device=hidden_states.device,
                dtype=hidden_states.dtype
            )

        # Add memex bias
        bias = self._get_bias_for_seq(seq_len, attention_mask.device, attention_mask.dtype)
        bias = bias.unsqueeze(0).unsqueeze(0)  # (1, 1, seq, seq)
        attention_mask = attention_mask + bias

        kwargs["attention_mask"] = attention_mask
        return args, kwargs

    def _hook_llama(self, module, args, kwargs):
        """Hook for Llama/Mistral/Phi style attention (SDPA compatible)."""
        if not self.active:
            return args, kwargs

        hidden_states = args[0] if args else kwargs.get("hidden_states")
        if hidden_states is None:
            return args, kwargs

        batch_size = hidden_states.shape[0]
        seq_len = hidden_states.shape[1]

        # Build bias - needs to be (batch, num_heads, seq, seq) for SDPA
        bias = self._get_bias_for_seq(seq_len, hidden_states.device, hidden_states.dtype)

        # Get number of attention heads from module config
        num_heads = getattr(module, 'num_heads', None) or getattr(module, 'num_attention_heads', 32)

        # Shape: (1, 1, seq, seq) - will broadcast over batch and heads
        bias = bias.unsqueeze(0).unsqueeze(0)

        # Get existing attention_mask
        attention_mask = kwargs.get("attention_mask")

        if attention_mask is not None:
            # Ensure bias matches attention_mask dimensions
            while bias.dim() < attention_mask.dim():
                bias = bias.unsqueeze(0)
            # Make sure it's contiguous and same dtype
            bias = bias.to(dtype=attention_mask.dtype).contiguous()
            attention_mask = attention_mask + bias
            attention_mask = attention_mask.contiguous()
        else:
            # Just use bias as the mask
            attention_mask = bias.contiguous()

        kwargs["attention_mask"] = attention_mask
        return args, kwargs

    def register(self, model) -> "ModelHooks":
        """Auto-detect model architecture and register hooks."""
        model_type = getattr(model.config, "model_type", "unknown")

        logger.info(f"Registering memex hooks for model type: {model_type}")

        if model_type in ["gpt2", "gpt_neo", "gpt_neox", "codegen"]:
            self._register_gpt2_hooks(model)
        elif model_type in ["llama", "mistral", "mixtral", "qwen2", "phi"]:
            self._register_llama_hooks(model)
        else:
            logger.warning(f"Unknown model type '{model_type}', trying Llama-style hooks")
            self._register_llama_hooks(model)

        logger.info(f"Registered {len(self.handles)} attention hooks")
        return self

    def _register_gpt2_hooks(self, model):
        """Register hooks for GPT-2 architecture."""
        for name, module in model.named_modules():
            if name.endswith(".attn"):
                handle = module.register_forward_pre_hook(self._hook_gpt2, with_kwargs=True)
                self.handles.append(handle)

    def _register_llama_hooks(self, model):
        """Register hooks for Llama architecture."""
        for name, module in model.named_modules():
            if "self_attn" in name and not any(x in name for x in ["self_attn.", "self_attn_"]):
                # Use forward hook with kwargs support
                handle = module.register_forward_pre_hook(self._hook_llama, with_kwargs=True)
                self.handles.append(handle)

    def remove(self):
        """Remove all hooks."""
        for handle in self.handles:
            handle.remove()
        self.handles = []

    def __enter__(self):
        self.active = True
        return self

    def __exit__(self, *args):
        self.active = False


class MemexBiasAdapter:
    """
    Main adapter class - converts memex graphs to attention bias.

    Usage:
        adapter = MemexBiasAdapter("knowledge.json")
        adapter.register(model)
        output = model.generate(...)
    """

    def __init__(self, graph_path: str = None, config: MemexConfig = None):
        self.config = config or MemexConfig()
        self.graph = MemexGraph(self.config)
        self.hooks: Optional[ModelHooks] = None
        self._tokenizer = None

        if graph_path:
            self.load(graph_path)

    def load(self, path: str):
        """Load memex graph from file."""
        logger.info(f"Loading memex graph from: {path}")
        self.graph.load_from_file(path)

    def load_from_dict(self, data: dict):
        """Load memex graph from dictionary."""
        self.graph.load_from_dict(data)

    def load_from_api(self, base_url: str, start_entity: str, min_weight: float = 0.5, max_nodes: int = 100):
        """Load subgraph from memex API."""
        import requests

        response = requests.get(
            f"{base_url}/api/query/attention_subgraph",
            params={
                "start": start_entity,
                "min_weight": min_weight,
                "max_nodes": max_nodes
            }
        )
        response.raise_for_status()
        self.graph.load_from_dict(response.json())

    def build_bias_for_text(self, text: str, tokenizer) -> torch.Tensor:
        """
        Build attention bias matrix for given text.

        Args:
            text: Input text
            tokenizer: Model tokenizer

        Returns:
            Attention bias tensor (seq_len, seq_len)
        """
        self._tokenizer = tokenizer

        # Find entities and map to tokens
        matcher = EntityMatcher(self.graph, tokenizer)
        entity_positions = matcher.get_entity_token_positions(text)

        if len(entity_positions) < 2:
            logger.debug(f"Found {len(entity_positions)} entities, need at least 2 for bias")

        # Get sequence length
        tokens = tokenizer.encode(text)
        seq_len = len(tokens)

        # Build bias matrix
        builder = BiasMatrixBuilder(self.graph)
        bias = builder.build(entity_positions, seq_len)

        logger.debug(f"Built bias matrix: {bias.shape}, {len(entity_positions)} entities")
        return bias

    def register(self, model, text: str, tokenizer) -> ModelHooks:
        """
        Register attention hooks with memex bias for given text.

        Args:
            model: HuggingFace model
            text: Input text to analyze for entities
            tokenizer: Model tokenizer

        Returns:
            ModelHooks instance (use as context manager or call .remove())
        """
        # Build bias matrix
        bias = self.build_bias_for_text(text, tokenizer)

        # Create and register hooks
        self.hooks = ModelHooks(bias, self.config)
        self.hooks.register(model)

        return self.hooks

    def unregister(self):
        """Remove all hooks."""
        if self.hooks:
            self.hooks.remove()
            self.hooks = None


# Convenience function for quick testing
def apply_memex_bias(model, tokenizer, text: str, graph_path: str) -> ModelHooks:
    """
    Quick function to apply memex bias to a model.

    Usage:
        hooks = apply_memex_bias(model, tokenizer, text, "graph.json")
        output = model.generate(...)
        hooks.remove()
    """
    adapter = MemexBiasAdapter(graph_path)
    return adapter.register(model, text, tokenizer)


if __name__ == "__main__":
    # Demo usage
    print("MemexBiasAdapter - Converts memex graphs to attention bias")
    print()
    print("Usage:")
    print("  from memex_bias_adapter import MemexBiasAdapter")
    print()
    print("  adapter = MemexBiasAdapter('knowledge.json')")
    print("  hooks = adapter.register(model, text, tokenizer)")
    print("  output = model.generate(...)")
    print("  hooks.remove()")
