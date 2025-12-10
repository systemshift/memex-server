#!/usr/bin/env python3
"""
Graph Attention Bias - Pre-computed attention from knowledge graph structure

This module explores using knowledge graph relationships as attention bias
for transformer models. The hypothesis: graph structure can serve as a
learned prior for attention, effectively extending context window by
focusing on semantically related information.

Research questions:
1. Does graph-biased attention outperform uniform attention?
2. Can we extend effective context window via structural priors?
3. What's the optimal way to encode graph distance as attention bias?

Usage:
    # Analyze natural attention vs graph structure
    python graph_attention_bias.py --analyze --model mistral-7b

    # Run with graph bias injection
    python graph_attention_bias.py --inject --model mistral-7b

    # Benchmark comparison
    python graph_attention_bias.py --benchmark --questions 100
"""

import argparse
import json
import logging
from dataclasses import dataclass
from typing import Optional

import torch
import torch.nn.functional as F
from neo4j import GraphDatabase

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@dataclass
class GraphBiasConfig:
    """Configuration for graph attention bias."""
    neo4j_uri: str = "bolt://localhost:7687"
    neo4j_user: str = "neo4j"
    neo4j_password: str = "password"

    # Bias computation
    max_graph_distance: int = 5  # Beyond this, bias = 0
    bias_scale: float = 1.0  # Multiplier for bias values
    bias_function: str = "inverse"  # inverse, exponential, linear

    # Model settings
    model_name: str = "mistralai/Mistral-7B-v0.1"
    device: str = "cuda" if torch.cuda.is_available() else "cpu"


class GraphBiasComputer:
    """Computes attention bias matrices from knowledge graph structure."""

    def __init__(self, config: GraphBiasConfig):
        self.config = config
        self.driver = GraphDatabase.driver(
            config.neo4j_uri,
            auth=(config.neo4j_user, config.neo4j_password)
        )

    def get_shortest_path(self, entity1: str, entity2: str) -> int:
        """Get shortest path length between two entities in the graph."""
        with self.driver.session() as session:
            result = session.run("""
                MATCH (a:Node {id: $e1}), (b:Node {id: $e2})
                MATCH path = shortestPath((a)-[*..10]-(b))
                RETURN length(path) as distance
            """, e1=entity1, e2=entity2)
            record = result.single()
            if record:
                return record["distance"]
            return self.config.max_graph_distance + 1  # No path found

    def compute_bias_value(self, distance: int) -> float:
        """Convert graph distance to attention bias value."""
        if distance > self.config.max_graph_distance:
            return 0.0

        if self.config.bias_function == "inverse":
            # 1 / (1 + distance)
            return self.config.bias_scale / (1 + distance)
        elif self.config.bias_function == "exponential":
            # exp(-distance)
            return self.config.bias_scale * (0.5 ** distance)
        elif self.config.bias_function == "linear":
            # (max - distance) / max
            return self.config.bias_scale * (self.config.max_graph_distance - distance) / self.config.max_graph_distance
        else:
            raise ValueError(f"Unknown bias function: {self.config.bias_function}")

    def compute_bias_matrix(
        self,
        entity_positions: dict[str, list[int]],
        seq_len: int
    ) -> torch.Tensor:
        """
        Compute attention bias matrix from entity positions and graph structure.

        Args:
            entity_positions: Map from entity_id to list of token positions
            seq_len: Total sequence length

        Returns:
            Tensor of shape (seq_len, seq_len) with bias values
        """
        bias = torch.zeros(seq_len, seq_len)

        entities = list(entity_positions.keys())
        for i, e1 in enumerate(entities):
            for e2 in entities[i+1:]:
                distance = self.get_shortest_path(e1, e2)
                bias_value = self.compute_bias_value(distance)

                # Apply bias to all token pairs between these entities
                for pos1 in entity_positions[e1]:
                    for pos2 in entity_positions[e2]:
                        bias[pos1, pos2] = bias_value
                        bias[pos2, pos1] = bias_value

        return bias

    def close(self):
        self.driver.close()


class EntityAligner:
    """Aligns entities in text to their graph IDs."""

    def __init__(self, driver):
        self.driver = driver
        self._entity_cache = {}

    def load_entities(self):
        """Load all entity names from graph."""
        with self.driver.session() as session:
            result = session.run("""
                MATCH (n:Node)
                WHERE n.type <> 'Source'
                RETURN n.id as id, n.properties as props
            """)
            for record in result:
                props = json.loads(record["props"]) if record["props"] else {}
                name = props.get("name", "")
                if name:
                    self._entity_cache[name.lower()] = record["id"]

    def find_entities_in_text(
        self,
        text: str,
        tokenizer
    ) -> dict[str, list[int]]:
        """
        Find entity mentions in text and return their token positions.

        Returns:
            Map from entity_id to list of token positions
        """
        # Simple substring matching - could be improved with NER
        entity_positions = {}
        text_lower = text.lower()

        tokens = tokenizer.encode(text)

        for name, entity_id in self._entity_cache.items():
            if name in text_lower:
                # Find token positions (simplified)
                # In practice, need proper token alignment
                start_char = text_lower.find(name)
                # Approximate token position based on character ratio
                approx_token = int(start_char / len(text) * len(tokens))

                if entity_id not in entity_positions:
                    entity_positions[entity_id] = []
                entity_positions[entity_id].append(approx_token)

        return entity_positions


def analyze_attention_patterns(
    model,
    tokenizer,
    text: str,
    graph_bias_computer: GraphBiasComputer,
    entity_aligner: EntityAligner
):
    """
    Compare model's natural attention to graph structure.

    This helps us understand: Does the model already attend to
    graph-connected entities, or would bias help?
    """
    inputs = tokenizer(text, return_tensors="pt").to(model.device)

    with torch.no_grad():
        outputs = model(**inputs, output_attentions=True)

    # Average attention across layers and heads
    attentions = outputs.attentions  # tuple of (batch, heads, seq, seq)
    avg_attention = torch.stack(attentions).mean(dim=[0, 1, 2, 3])

    # Get entity positions
    entity_positions = entity_aligner.find_entities_in_text(text, tokenizer)

    # Compare natural attention to graph distance
    results = []
    entities = list(entity_positions.keys())

    for i, e1 in enumerate(entities):
        for e2 in entities[i+1:]:
            graph_dist = graph_bias_computer.get_shortest_path(e1, e2)

            # Get average attention between entity tokens
            natural_attn = 0.0
            count = 0
            for pos1 in entity_positions[e1]:
                for pos2 in entity_positions[e2]:
                    if pos1 < avg_attention.shape[0] and pos2 < avg_attention.shape[1]:
                        natural_attn += avg_attention[pos1, pos2].item()
                        count += 1

            if count > 0:
                natural_attn /= count

            results.append({
                "entity1": e1,
                "entity2": e2,
                "graph_distance": graph_dist,
                "natural_attention": natural_attn,
                "correlation": -1  # Placeholder
            })

    return results


def inject_graph_bias(
    model,
    tokenizer,
    text: str,
    question: str,
    graph_bias_computer: GraphBiasComputer,
    entity_aligner: EntityAligner
) -> str:
    """
    Run inference with graph-based attention bias.

    This is the core experiment: inject graph structure into attention.
    """
    full_text = f"Context: {text}\n\nQuestion: {question}\n\nAnswer:"
    inputs = tokenizer(full_text, return_tensors="pt").to(model.device)

    # Compute graph bias
    entity_positions = entity_aligner.find_entities_in_text(full_text, tokenizer)
    seq_len = inputs.input_ids.shape[1]
    graph_bias = graph_bias_computer.compute_bias_matrix(entity_positions, seq_len)
    graph_bias = graph_bias.to(model.device)

    # TODO: Actually inject the bias into the model
    # This requires modifying the attention mechanism
    # For now, this is a placeholder showing the structure

    logger.warning("Bias injection not yet implemented - running standard inference")

    with torch.no_grad():
        outputs = model.generate(
            inputs.input_ids,
            max_new_tokens=100,
            do_sample=False
        )

    answer = tokenizer.decode(outputs[0], skip_special_tokens=True)
    return answer.split("Answer:")[-1].strip()


def main():
    parser = argparse.ArgumentParser(description="Graph Attention Bias Experiments")
    parser.add_argument("--analyze", action="store_true", help="Analyze attention vs graph structure")
    parser.add_argument("--inject", action="store_true", help="Run with graph bias injection")
    parser.add_argument("--benchmark", action="store_true", help="Run benchmark comparison")
    parser.add_argument("--model", default="mistralai/Mistral-7B-v0.1", help="Model to use")
    parser.add_argument("--questions", type=int, default=20, help="Number of questions for benchmark")
    args = parser.parse_args()

    config = GraphBiasConfig(model_name=args.model)

    logger.info(f"Graph Attention Bias Experiment")
    logger.info(f"Model: {config.model_name}")
    logger.info(f"Device: {config.device}")
    logger.info(f"Bias function: {config.bias_function}")

    if not any([args.analyze, args.inject, args.benchmark]):
        logger.info("No action specified. Use --analyze, --inject, or --benchmark")
        logger.info("\nThis module implements graph-based attention bias for transformers.")
        logger.info("The hypothesis: knowledge graph structure can improve attention focus.")
        return

    # Initialize components
    graph_computer = GraphBiasComputer(config)
    entity_aligner = EntityAligner(graph_computer.driver)
    entity_aligner.load_entities()

    if args.analyze:
        logger.info("Analysis mode - comparing natural attention to graph structure")
        # TODO: Load model and run analysis
        logger.info("Model loading not implemented yet - requires GPU")

    if args.inject:
        logger.info("Injection mode - running with graph bias")
        # TODO: Load model and run with bias
        logger.info("Bias injection not implemented yet - requires model modification")

    if args.benchmark:
        logger.info(f"Benchmark mode - comparing {args.questions} questions")
        # TODO: Run benchmark
        logger.info("Benchmark not implemented yet")

    graph_computer.close()


if __name__ == "__main__":
    main()
