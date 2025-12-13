#!/usr/bin/env python3
"""
Benchmark Memex Bias Injection

Measures whether graph-biased attention improves model performance on
multi-hop reasoning questions.

Metrics:
- Accuracy (exact match)
- F1 score (token overlap)
- Answer length
- Entity recall (does answer mention relevant entities)
"""

import argparse
import json
import re
import time
from dataclasses import dataclass, field
from typing import Optional
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer

from memex_sdpa import MemexSDPA, MemexGraph, GraphEntity, set_global_bias, clear_global_bias


@dataclass
class BenchmarkResult:
    """Result for a single question."""
    question: str
    context: str
    ground_truth: str
    baseline_answer: str
    biased_answer: str
    baseline_correct: bool
    biased_correct: bool
    baseline_f1: float
    biased_f1: float
    baseline_entity_recall: float
    biased_entity_recall: float
    entities_found: int
    bias_changed_output: bool


@dataclass
class BenchmarkStats:
    """Aggregate statistics."""
    total_questions: int = 0
    baseline_correct: int = 0
    biased_correct: int = 0
    baseline_f1_sum: float = 0.0
    biased_f1_sum: float = 0.0
    baseline_entity_recall_sum: float = 0.0
    biased_entity_recall_sum: float = 0.0
    output_changed_count: int = 0
    improved_count: int = 0
    degraded_count: int = 0

    def accuracy(self, biased=False):
        if self.total_questions == 0:
            return 0.0
        if biased:
            return self.biased_correct / self.total_questions
        return self.baseline_correct / self.total_questions

    def avg_f1(self, biased=False):
        if self.total_questions == 0:
            return 0.0
        if biased:
            return self.biased_f1_sum / self.total_questions
        return self.baseline_f1_sum / self.total_questions

    def avg_entity_recall(self, biased=False):
        if self.total_questions == 0:
            return 0.0
        if biased:
            return self.biased_entity_recall_sum / self.total_questions
        return self.baseline_entity_recall_sum / self.total_questions


def normalize_answer(text: str) -> str:
    """Normalize answer for comparison."""
    text = text.lower()
    text = re.sub(r'[^\w\s]', ' ', text)
    text = ' '.join(text.split())
    return text


def compute_f1(pred: str, truth: str) -> float:
    """Compute token-level F1 score."""
    pred_tokens = set(normalize_answer(pred).split())
    truth_tokens = set(normalize_answer(truth).split())

    if not pred_tokens or not truth_tokens:
        return 0.0

    common = pred_tokens & truth_tokens
    if not common:
        return 0.0

    precision = len(common) / len(pred_tokens)
    recall = len(common) / len(truth_tokens)
    return 2 * precision * recall / (precision + recall)


def compute_exact_match(pred: str, truth: str) -> bool:
    """Check if normalized answers match."""
    return normalize_answer(pred) == normalize_answer(truth)


def compute_entity_recall(answer: str, entities: list[str]) -> float:
    """Compute fraction of expected entities mentioned in answer."""
    if not entities:
        return 1.0
    answer_lower = answer.lower()
    found = sum(1 for e in entities if e.lower() in answer_lower)
    return found / len(entities)


def create_synthetic_benchmark() -> list[dict]:
    """Create synthetic benchmark questions."""
    return [
        {
            "context": "Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice. Carol founded DataInc, which was later acquired by TechCorp. Bob now oversees the integration of DataInc.",
            "question": "Who does Bob report to?",
            "answer": "Alice",
            "entities": ["Alice", "Bob"],
            "hops": 1
        },
        {
            "context": "Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice. Carol founded DataInc, which was later acquired by TechCorp. Bob now oversees the integration of DataInc.",
            "question": "Who is the CEO of the company that acquired DataInc?",
            "answer": "Alice",
            "entities": ["Alice", "TechCorp", "DataInc"],
            "hops": 2
        },
        {
            "context": "Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice. Carol founded DataInc, which was later acquired by TechCorp. Bob now oversees the integration of DataInc.",
            "question": "Who founded the company that Bob is now integrating?",
            "answer": "Carol",
            "entities": ["Carol", "Bob", "DataInc"],
            "hops": 2
        },
        {
            "context": "Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice. Carol founded DataInc, which was later acquired by TechCorp. Bob now oversees the integration of DataInc.",
            "question": "What company did Carol found?",
            "answer": "DataInc",
            "entities": ["Carol", "DataInc"],
            "hops": 1
        },
        {
            "context": "Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice. Carol founded DataInc, which was later acquired by TechCorp. Bob now oversees the integration of DataInc.",
            "question": "Who oversees the integration of the company founded by Carol?",
            "answer": "Bob",
            "entities": ["Bob", "Carol", "DataInc"],
            "hops": 2
        },
        {
            "context": "The Python programming language was created by Guido van Rossum. Python is maintained by the Python Software Foundation. NumPy is a popular Python library created by Travis Oliphant. NumPy is used extensively in data science.",
            "question": "Who created the language that NumPy is built on?",
            "answer": "Guido van Rossum",
            "entities": ["Guido van Rossum", "Python", "NumPy"],
            "hops": 2
        },
        {
            "context": "Microsoft was founded by Bill Gates and Paul Allen. Satya Nadella is the current CEO of Microsoft. GitHub was acquired by Microsoft. GitHub was originally founded by Tom Preston-Werner.",
            "question": "Who is the CEO of the company that acquired GitHub?",
            "answer": "Satya Nadella",
            "entities": ["Satya Nadella", "Microsoft", "GitHub"],
            "hops": 2
        },
        {
            "context": "Microsoft was founded by Bill Gates and Paul Allen. Satya Nadella is the current CEO of Microsoft. GitHub was acquired by Microsoft. GitHub was originally founded by Tom Preston-Werner.",
            "question": "Who founded GitHub?",
            "answer": "Tom Preston-Werner",
            "entities": ["Tom Preston-Werner", "GitHub"],
            "hops": 1
        },
    ]


def create_graph_for_benchmark(questions: list[dict]) -> MemexGraph:
    """Create graph with entities from benchmark questions."""
    graph = MemexGraph()

    # Extract all entities
    all_entities = set()
    for q in questions:
        all_entities.update(q.get("entities", []))

    # Add entities
    for name in all_entities:
        entity_id = name.lower().replace(" ", "_")
        graph.add_entity(GraphEntity(id=entity_id, name=name))

    # Add edges based on co-occurrence in contexts
    for q in questions:
        entities = q.get("entities", [])
        for i, e1 in enumerate(entities):
            for e2 in entities[i+1:]:
                id1 = e1.lower().replace(" ", "_")
                id2 = e2.lower().replace(" ", "_")
                # Stronger weight for multi-hop questions
                weight = 1.5 if q.get("hops", 1) > 1 else 1.0
                # Add or strengthen edge
                current = graph.get_weight(id1, id2)
                graph.add_edge(id1, id2, max(current, weight))

    print(f"Created graph: {len(graph.entities)} entities, {len(graph.weights)//2} edges")
    return graph


def run_benchmark(
    model,
    tokenizer,
    memex: MemexSDPA,
    questions: list[dict],
    max_new_tokens: int = 50,
    verbose: bool = True
) -> tuple[list[BenchmarkResult], BenchmarkStats]:
    """Run benchmark comparing baseline vs biased outputs."""

    results = []
    stats = BenchmarkStats()

    for i, q in enumerate(questions):
        context = q["context"]
        question = q["question"]
        ground_truth = q["answer"]
        expected_entities = q.get("entities", [])

        prompt = f"Context: {context}\n\nQuestion: {question}\n\nAnswer:"
        inputs = tokenizer(prompt, return_tensors="pt", truncation=True, max_length=512)
        inputs = {k: v.to(model.device) for k, v in inputs.items()}

        # Build bias matrix
        bias_matrix = memex.build_bias_matrix(prompt, tokenizer)
        entities_found = len(memex.find_entities_in_text(prompt))

        # Baseline (no bias)
        clear_global_bias()
        with torch.no_grad():
            outputs = model.generate(
                **inputs,
                max_new_tokens=max_new_tokens,
                do_sample=False,
                pad_token_id=tokenizer.eos_token_id
            )
        baseline_full = tokenizer.decode(outputs[0], skip_special_tokens=True)
        baseline_answer = baseline_full.split("Answer:")[-1].strip().split("\n")[0]

        # With bias
        set_global_bias(bias_matrix.to(model.device))
        with torch.no_grad():
            outputs = model.generate(
                **inputs,
                max_new_tokens=max_new_tokens,
                do_sample=False,
                pad_token_id=tokenizer.eos_token_id
            )
        clear_global_bias()
        biased_full = tokenizer.decode(outputs[0], skip_special_tokens=True)
        biased_answer = biased_full.split("Answer:")[-1].strip().split("\n")[0]

        # Compute metrics
        baseline_correct = compute_exact_match(baseline_answer, ground_truth)
        biased_correct = compute_exact_match(biased_answer, ground_truth)
        baseline_f1 = compute_f1(baseline_answer, ground_truth)
        biased_f1 = compute_f1(biased_answer, ground_truth)
        baseline_entity_recall = compute_entity_recall(baseline_answer, expected_entities)
        biased_entity_recall = compute_entity_recall(biased_answer, expected_entities)
        output_changed = baseline_answer != biased_answer

        result = BenchmarkResult(
            question=question,
            context=context[:100] + "...",
            ground_truth=ground_truth,
            baseline_answer=baseline_answer[:100],
            biased_answer=biased_answer[:100],
            baseline_correct=baseline_correct,
            biased_correct=biased_correct,
            baseline_f1=baseline_f1,
            biased_f1=biased_f1,
            baseline_entity_recall=baseline_entity_recall,
            biased_entity_recall=biased_entity_recall,
            entities_found=entities_found,
            bias_changed_output=output_changed
        )
        results.append(result)

        # Update stats
        stats.total_questions += 1
        stats.baseline_correct += int(baseline_correct)
        stats.biased_correct += int(biased_correct)
        stats.baseline_f1_sum += baseline_f1
        stats.biased_f1_sum += biased_f1
        stats.baseline_entity_recall_sum += baseline_entity_recall
        stats.biased_entity_recall_sum += biased_entity_recall
        stats.output_changed_count += int(output_changed)

        if biased_f1 > baseline_f1:
            stats.improved_count += 1
        elif biased_f1 < baseline_f1:
            stats.degraded_count += 1

        if verbose:
            status = "✓" if biased_correct else ("↑" if biased_f1 > baseline_f1 else "·")
            print(f"  [{i+1}/{len(questions)}] {status} Q: {question[:50]}...")
            print(f"       Truth: {ground_truth}")
            print(f"       Base:  {baseline_answer[:60]}  (F1={baseline_f1:.2f})")
            print(f"       Bias:  {biased_answer[:60]}  (F1={biased_f1:.2f})")

    return results, stats


def print_stats(stats: BenchmarkStats):
    """Print benchmark statistics."""
    print("\n" + "=" * 60)
    print("BENCHMARK RESULTS")
    print("=" * 60)
    print(f"Total questions: {stats.total_questions}")
    print()
    print(f"{'Metric':<25} {'Baseline':>12} {'With Bias':>12} {'Delta':>10}")
    print("-" * 60)
    print(f"{'Accuracy':<25} {stats.accuracy(False)*100:>11.1f}% {stats.accuracy(True)*100:>11.1f}% {(stats.accuracy(True)-stats.accuracy(False))*100:>+9.1f}%")
    print(f"{'Avg F1 Score':<25} {stats.avg_f1(False):>12.3f} {stats.avg_f1(True):>12.3f} {stats.avg_f1(True)-stats.avg_f1(False):>+10.3f}")
    print(f"{'Avg Entity Recall':<25} {stats.avg_entity_recall(False):>12.3f} {stats.avg_entity_recall(True):>12.3f} {stats.avg_entity_recall(True)-stats.avg_entity_recall(False):>+10.3f}")
    print()
    print(f"Output changed: {stats.output_changed_count}/{stats.total_questions} ({stats.output_changed_count/stats.total_questions*100:.1f}%)")
    print(f"Improved: {stats.improved_count} | Degraded: {stats.degraded_count} | Same: {stats.total_questions - stats.improved_count - stats.degraded_count}")
    print("=" * 60)


def main():
    parser = argparse.ArgumentParser(description="Benchmark Memex Bias Injection")
    parser.add_argument("--model", default="microsoft/phi-2", help="Model to use")
    parser.add_argument("--bias-scale", type=float, default=1.5, help="Bias scale factor")
    parser.add_argument("--questions", type=int, default=None, help="Number of questions (default: all)")
    parser.add_argument("--neo4j", action="store_true", help="Load graph from Neo4j")
    parser.add_argument("--graph-file", type=str, help="Load graph from JSON file")
    parser.add_argument("--output", type=str, help="Save results to JSON file")
    args = parser.parse_args()

    print("=" * 60)
    print("MEMEX BIAS INJECTION BENCHMARK")
    print("=" * 60)
    print(f"Model: {args.model}")
    print(f"Bias scale: {args.bias_scale}")

    # Load model
    print("\nLoading model...")
    tokenizer = AutoTokenizer.from_pretrained(args.model, trust_remote_code=True)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        args.model,
        torch_dtype=torch.float16,
        device_map="auto",
        trust_remote_code=True
    )
    model.eval()
    print(f"Model loaded on {model.device}")

    # Load or create graph
    if args.neo4j:
        from memex_sdpa import load_graph_from_neo4j
        graph = load_graph_from_neo4j()
    elif args.graph_file:
        from memex_sdpa import load_graph_from_file
        graph = load_graph_from_file(args.graph_file)
    else:
        print("\nUsing synthetic benchmark...")
        questions = create_synthetic_benchmark()
        graph = create_graph_for_benchmark(questions)

    # Create memex
    memex = MemexSDPA(graph, bias_scale=args.bias_scale)

    # Get questions
    if not args.neo4j and not args.graph_file:
        questions = create_synthetic_benchmark()
    else:
        # TODO: Load questions from HotpotQA or similar
        questions = create_synthetic_benchmark()

    if args.questions:
        questions = questions[:args.questions]

    # Run benchmark
    print(f"\nRunning benchmark on {len(questions)} questions...")
    start_time = time.time()
    results, stats = run_benchmark(model, tokenizer, memex, questions)
    elapsed = time.time() - start_time

    # Print results
    print_stats(stats)
    print(f"\nTime: {elapsed:.1f}s ({elapsed/len(questions):.1f}s per question)")

    # Save results
    if args.output:
        output_data = {
            "config": {
                "model": args.model,
                "bias_scale": args.bias_scale,
                "questions": len(questions)
            },
            "stats": {
                "total": stats.total_questions,
                "baseline_accuracy": stats.accuracy(False),
                "biased_accuracy": stats.accuracy(True),
                "baseline_f1": stats.avg_f1(False),
                "biased_f1": stats.avg_f1(True),
                "improved": stats.improved_count,
                "degraded": stats.degraded_count,
                "output_changed": stats.output_changed_count
            },
            "results": [
                {
                    "question": r.question,
                    "ground_truth": r.ground_truth,
                    "baseline": r.baseline_answer,
                    "biased": r.biased_answer,
                    "baseline_f1": r.baseline_f1,
                    "biased_f1": r.biased_f1,
                    "changed": r.bias_changed_output
                }
                for r in results
            ]
        }
        with open(args.output, "w") as f:
            json.dump(output_data, f, indent=2)
        print(f"\nResults saved to {args.output}")


if __name__ == "__main__":
    main()
