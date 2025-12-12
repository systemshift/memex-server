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
    python graph_attention_bias.py --analyze

    # Run with graph bias injection (experimental)
    python graph_attention_bias.py --inject

    # Benchmark comparison
    python graph_attention_bias.py --benchmark --questions 100
"""

import argparse
import json
import logging
import os
from dataclasses import dataclass, field
from typing import Optional
from collections import defaultdict

import torch
import torch.nn.functional as F
from neo4j import GraphDatabase

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(levelname)s] %(message)s')
logger = logging.getLogger(__name__)

# Models that work well on CPU for research
CPU_FRIENDLY_MODELS = {
    "gpt2": "gpt2",  # 124M params, fast
    "gpt2-medium": "gpt2-medium",  # 355M params
    "distilgpt2": "distilgpt2",  # 82M params, fastest
}


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

    # Model settings - default to CPU-friendly model
    model_name: str = "gpt2"
    device: str = "cuda" if torch.cuda.is_available() else "cpu"

    # Analysis settings
    use_layer_avg: bool = True  # Average across layers
    use_head_avg: bool = True   # Average across attention heads


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


class GraphBiasInjector:
    """
    Injects graph bias into transformer attention using forward hooks.

    This is the core mechanism: we intercept attention input and modify
    the attention mask to include graph structure bias.

    Standard: attn = softmax(Q @ K.T / sqrt(d) + mask)
    Modified: attn = softmax(Q @ K.T / sqrt(d) + mask + graph_bias)

    The attention mask is additive (applied before softmax), so we can
    simply add our graph bias to it.
    """

    def __init__(self, graph_bias: torch.Tensor, bias_scale: float = 1.0):
        """
        Args:
            graph_bias: (seq_len, seq_len) tensor of bias values
            bias_scale: Multiplier for bias values
        """
        self.graph_bias = graph_bias * bias_scale
        self.handles = []
        self.active = True

    def _pre_hook_gpt2(self, module, args):
        """
        Pre-hook for GPT-2 style attention.

        GPT2Attention.forward signature:
            hidden_states, layer_past, attention_mask, head_mask, ...

        We modify attention_mask (index 2) to include graph bias.
        """
        if not self.active:
            return args

        args = list(args)

        # Get sequence length from hidden states
        hidden_states = args[0]
        seq_len = hidden_states.shape[1]

        # Get or create attention mask
        if len(args) > 2 and args[2] is not None:
            attention_mask = args[2]
        else:
            # Create default mask (all zeros = no masking)
            attention_mask = torch.zeros(
                1, 1, seq_len, seq_len,
                device=hidden_states.device,
                dtype=hidden_states.dtype
            )

        # Add graph bias to attention mask
        if seq_len <= self.graph_bias.shape[0]:
            bias = self.graph_bias[:seq_len, :seq_len].to(attention_mask.device).to(attention_mask.dtype)
            # Reshape for broadcasting: (1, 1, seq, seq)
            bias = bias.unsqueeze(0).unsqueeze(0)
            attention_mask = attention_mask + bias

        args[2] = attention_mask
        return tuple(args)

    def register_gpt2(self, model):
        """Register hooks for GPT-2 architecture."""
        for name, module in model.named_modules():
            if name.endswith('.attn'):  # GPT2Attention modules
                handle = module.register_forward_pre_hook(self._pre_hook_gpt2)
                self.handles.append(handle)
                logger.debug(f"Registered bias hook on: {name}")

        logger.info(f"Registered {len(self.handles)} attention hooks")
        return self

    def register(self, model):
        """Auto-detect architecture and register appropriate hooks."""
        model_type = getattr(model.config, 'model_type', 'unknown')

        if model_type in ['gpt2', 'gpt_neo', 'gpt_neox']:
            return self.register_gpt2(model)
        else:
            logger.warning(f"Unknown model type: {model_type}, trying GPT-2 hooks")
            return self.register_gpt2(model)

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


class GraphBiasedModel:
    """
    Wrapper that applies graph bias to any HuggingFace causal LM.

    This is the production-ready implementation for graph-biased inference.
    Uses forward hooks to inject bias BEFORE softmax in attention layers.
    """

    def __init__(self, model, tokenizer, graph_computer: GraphBiasComputer,
                 entity_aligner: EntityAligner, bias_scale: float = 2.0):
        self.model = model
        self.tokenizer = tokenizer
        self.graph_computer = graph_computer
        self.entity_aligner = entity_aligner
        self.bias_scale = bias_scale
        self.injector = None

    def generate_with_bias(self, text: str, question: str, max_new_tokens: int = 100) -> str:
        """
        Generate answer with graph-biased attention.

        This is the core experiment: we inject graph structure into attention
        by modifying the attention mask before softmax.
        """
        full_text = f"Context: {text}\n\nQuestion: {question}\n\nAnswer:"

        # Tokenize
        inputs = self.tokenizer(full_text, return_tensors="pt", truncation=True, max_length=512)
        inputs = {k: v.to(self.model.device) for k, v in inputs.items()}
        seq_len = inputs['input_ids'].shape[1]

        # Find entities and compute bias
        entity_positions = self.entity_aligner.find_entities_in_text(full_text, self.tokenizer)

        if len(entity_positions) < 2:
            logger.info("Not enough entities found, running without bias")
            return self.generate_without_bias(text, question, max_new_tokens)

        # Compute graph bias matrix
        logger.info(f"Computing bias for {len(entity_positions)} entities")
        graph_bias = self.graph_computer.compute_bias_matrix(entity_positions, seq_len)

        # Create and register injector
        self.injector = GraphBiasInjector(graph_bias, bias_scale=self.bias_scale)
        self.injector.register(self.model)

        try:
            with torch.no_grad():
                outputs = self.model.generate(
                    inputs['input_ids'],
                    attention_mask=inputs.get('attention_mask'),
                    max_new_tokens=max_new_tokens,
                    do_sample=False,
                    pad_token_id=self.tokenizer.pad_token_id,
                )
        finally:
            # Always remove hooks
            self.injector.remove()

        answer = self.tokenizer.decode(outputs[0], skip_special_tokens=True)
        return answer.split("Answer:")[-1].strip()

    def generate_without_bias(self, text: str, question: str, max_new_tokens: int = 100) -> str:
        """Generate answer without graph bias (baseline)."""
        full_text = f"Context: {text}\n\nQuestion: {question}\n\nAnswer:"

        inputs = self.tokenizer(full_text, return_tensors="pt", truncation=True, max_length=512)
        inputs = {k: v.to(self.model.device) for k, v in inputs.items()}

        with torch.no_grad():
            outputs = self.model.generate(
                inputs['input_ids'],
                attention_mask=inputs.get('attention_mask'),
                max_new_tokens=max_new_tokens,
                do_sample=False,
                pad_token_id=self.tokenizer.pad_token_id,
            )

        answer = self.tokenizer.decode(outputs[0], skip_special_tokens=True)
        return answer.split("Answer:")[-1].strip()

    def compare_outputs(self, text: str, question: str) -> dict:
        """Compare biased vs unbiased outputs for analysis."""
        without_bias = self.generate_without_bias(text, question)
        with_bias = self.generate_with_bias(text, question)

        return {
            "question": question,
            "without_bias": without_bias,
            "with_bias": with_bias,
            "different": without_bias != with_bias
        }


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
    biased_model = GraphBiasedModel(
        model, tokenizer, graph_bias_computer, entity_aligner
    )
    return biased_model.generate_with_bias(text, question)


def load_model_and_tokenizer(config: GraphBiasConfig):
    """Load a transformer model and tokenizer."""
    from transformers import AutoModelForCausalLM, AutoTokenizer

    logger.info(f"Loading model: {config.model_name}")
    logger.info(f"Device: {config.device}")

    tokenizer = AutoTokenizer.from_pretrained(config.model_name)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        config.model_name,
        output_attentions=True,
        torch_dtype=torch.float32,  # CPU-friendly
    )
    model = model.to(config.device)
    model.eval()

    logger.info(f"Model loaded: {model.config.n_layer} layers, {model.config.n_head} heads")
    return model, tokenizer


def run_attention_analysis(config: GraphBiasConfig):
    """
    Main analysis: compare natural attention patterns to graph distance.

    This answers the research question: Do transformers naturally attend
    more to graph-connected entities, or is there room for bias injection?
    """
    from scipy import stats

    # Initialize graph connection
    logger.info("Connecting to Neo4j...")
    graph_computer = GraphBiasComputer(config)
    entity_aligner = EntityAligner(graph_computer.driver)

    logger.info("Loading entities from graph...")
    entity_aligner.load_entities()
    logger.info(f"Loaded {len(entity_aligner._entity_cache)} entities")

    # Load model
    model, tokenizer = load_model_and_tokenizer(config)

    # Get sample texts from the graph (using sources)
    sample_texts = []
    with graph_computer.driver.session() as session:
        result = session.run("""
            MATCH (s:Node {type: 'Source'})
            WHERE s.properties IS NOT NULL
            RETURN s.properties as props
            LIMIT 5
        """)
        for record in result:
            props = json.loads(record["props"]) if record["props"] else {}
            content = props.get("content", "")
            if len(content) > 100:
                # Truncate for analysis (GPT-2 max 1024 tokens)
                sample_texts.append(content[:2000])

    if not sample_texts:
        logger.warning("No source texts found in graph. Using synthetic example.")
        sample_texts = [
            "Alice works at TechCorp where she manages the AI team. "
            "Bob is the CTO of TechCorp and reports to the CEO Carol. "
            "The company is developing a new product called SmartBot. "
            "Alice met with Bob yesterday to discuss the SmartBot roadmap."
        ]

    # Collect correlation data
    all_correlations = []

    for i, text in enumerate(sample_texts):
        logger.info(f"\n--- Analyzing text {i+1}/{len(sample_texts)} ---")
        logger.info(f"Text preview: {text[:100]}...")

        # Find entities in text
        entity_positions = entity_aligner.find_entities_in_text(text, tokenizer)
        if len(entity_positions) < 2:
            logger.info("Not enough entities found, skipping")
            continue

        logger.info(f"Found {len(entity_positions)} entities: {list(entity_positions.keys())[:5]}...")

        # Get model attention
        inputs = tokenizer(text, return_tensors="pt", truncation=True, max_length=512)
        inputs = {k: v.to(config.device) for k, v in inputs.items()}

        with torch.no_grad():
            outputs = model(**inputs, output_attentions=True)

        # Stack and average attentions: (layers, batch, heads, seq, seq)
        attentions = torch.stack(outputs.attentions)

        if config.use_layer_avg:
            attentions = attentions.mean(dim=0)  # Average layers
        else:
            attentions = attentions[-1]  # Use last layer

        if config.use_head_avg:
            avg_attention = attentions.mean(dim=1).squeeze(0)  # Average heads
        else:
            avg_attention = attentions[:, 0, :, :].squeeze(0)  # Use first head

        # Compare attention to graph distance
        entities = list(entity_positions.keys())
        graph_distances = []
        attention_values = []

        for j, e1 in enumerate(entities):
            for e2 in entities[j+1:]:
                graph_dist = graph_computer.get_shortest_path(e1, e2)
                if graph_dist > config.max_graph_distance:
                    continue

                # Get attention between entity token positions
                attn_sum = 0.0
                count = 0
                for pos1 in entity_positions[e1]:
                    for pos2 in entity_positions[e2]:
                        if pos1 < avg_attention.shape[0] and pos2 < avg_attention.shape[1]:
                            attn_sum += avg_attention[pos1, pos2].item()
                            attn_sum += avg_attention[pos2, pos1].item()  # Bidirectional
                            count += 2

                if count > 0:
                    avg_attn = attn_sum / count
                    graph_distances.append(graph_dist)
                    attention_values.append(avg_attn)
                    all_correlations.append((graph_dist, avg_attn))

        if len(graph_distances) >= 3:
            corr, p_value = stats.spearmanr(graph_distances, attention_values)
            logger.info(f"Spearman correlation: {corr:.3f} (p={p_value:.4f})")
            logger.info(f"  Hypothesis: negative correlation = model attends more to closer entities")

    # Overall analysis
    if all_correlations:
        all_dists = [c[0] for c in all_correlations]
        all_attns = [c[1] for c in all_correlations]

        overall_corr, overall_p = stats.spearmanr(all_dists, all_attns)

        logger.info("\n" + "="*60)
        logger.info("OVERALL RESULTS")
        logger.info("="*60)
        logger.info(f"Total entity pairs analyzed: {len(all_correlations)}")
        logger.info(f"Spearman correlation: {overall_corr:.4f}")
        logger.info(f"P-value: {overall_p:.6f}")

        if overall_corr < -0.1 and overall_p < 0.05:
            logger.info("FINDING: Model naturally attends more to graph-close entities")
            logger.info("         Graph bias injection may have limited benefit")
        elif overall_corr > 0.1 and overall_p < 0.05:
            logger.info("FINDING: Model attends LESS to graph-close entities (unexpected)")
            logger.info("         Graph bias injection could significantly help")
        else:
            logger.info("FINDING: No strong correlation between graph distance and attention")
            logger.info("         Graph bias injection could provide new signal")

        # Group by distance
        dist_groups = defaultdict(list)
        for d, a in all_correlations:
            dist_groups[d].append(a)

        logger.info("\nAverage attention by graph distance:")
        for d in sorted(dist_groups.keys()):
            avg = sum(dist_groups[d]) / len(dist_groups[d])
            logger.info(f"  Distance {d}: avg_attention = {avg:.6f} (n={len(dist_groups[d])})")

        # Save results
        results = {
            "correlation": overall_corr,
            "p_value": overall_p,
            "n_pairs": len(all_correlations),
            "model": config.model_name,
            "by_distance": {d: sum(v)/len(v) for d, v in dist_groups.items()}
        }
        with open("/home/deocy/memex/bench/attention_analysis_results.json", "w") as f:
            json.dump(results, f, indent=2)
        logger.info("\nResults saved to attention_analysis_results.json")

    graph_computer.close()
    return all_correlations


def main():
    parser = argparse.ArgumentParser(description="Graph Attention Bias Experiments")
    parser.add_argument("--analyze", action="store_true", help="Analyze attention vs graph structure")
    parser.add_argument("--inject", action="store_true", help="Run with graph bias injection")
    parser.add_argument("--benchmark", action="store_true", help="Run benchmark comparison")
    parser.add_argument("--model", default="gpt2", help="Model to use (gpt2, distilgpt2, gpt2-medium)")
    parser.add_argument("--questions", type=int, default=20, help="Number of questions for benchmark")
    args = parser.parse_args()

    config = GraphBiasConfig(model_name=args.model)

    logger.info("="*60)
    logger.info("GRAPH ATTENTION BIAS EXPERIMENT")
    logger.info("="*60)
    logger.info(f"Model: {config.model_name}")
    logger.info(f"Device: {config.device}")
    logger.info(f"Bias function: {config.bias_function}")

    if not any([args.analyze, args.inject, args.benchmark]):
        logger.info("\nNo action specified. Available modes:")
        logger.info("  --analyze   : Compare natural attention to graph structure")
        logger.info("  --inject    : Run with graph bias injection (experimental)")
        logger.info("  --benchmark : Run QA benchmark comparison")
        logger.info("\nThis module tests the hypothesis that knowledge graph structure")
        logger.info("can serve as attention bias to improve context understanding.")
        return

    if args.analyze:
        logger.info("\n--- ANALYSIS MODE ---")
        logger.info("Comparing model's natural attention patterns to graph distances")
        run_attention_analysis(config)

    if args.inject:
        logger.info("\n--- INJECTION MODE ---")
        logger.info("Testing graph bias injection on sample text")

        # Load model
        model, tokenizer = load_model_and_tokenizer(config)

        # Initialize graph
        graph_computer = GraphBiasComputer(config)
        entity_aligner = EntityAligner(graph_computer.driver)
        entity_aligner.load_entities()

        # Create biased model wrapper
        biased_model = GraphBiasedModel(
            model, tokenizer, graph_computer, entity_aligner,
            bias_scale=2.0  # Experiment with this value
        )

        # Test text (use synthetic if no real data)
        test_text = """
        Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice.
        Carol is the head of the AI research division. She collaborates closely with Bob.
        TechCorp recently acquired DataInc, a company founded by David.
        David now leads the data infrastructure team at TechCorp.
        """

        test_questions = [
            "Who is the CEO of TechCorp?",
            "Who does Bob report to?",
            "What company did TechCorp acquire?",
            "Who founded DataInc?",
        ]

        logger.info("\nComparing biased vs unbiased outputs:")
        for q in test_questions:
            result = biased_model.compare_outputs(test_text, q)
            logger.info(f"\nQ: {q}")
            logger.info(f"  Without bias: {result['without_bias'][:100]}...")
            logger.info(f"  With bias:    {result['with_bias'][:100]}...")
            logger.info(f"  Different: {result['different']}")

        graph_computer.close()

    if args.benchmark:
        logger.info(f"\n--- BENCHMARK MODE ---")
        logger.info(f"Would compare {args.questions} questions with/without graph bias")
        logger.info("Not yet implemented - requires attention hook integration")


if __name__ == "__main__":
    main()
