#!/usr/bin/env python3
"""
Evaluate whether the World Model discovers hidden patterns.

This is the key test: can the model find relationships we didn't tell it about?
"""

import json
import sys
from typing import Dict, List, Any, Tuple
from collections import defaultdict

import torch
import torch.nn.functional as F
import numpy as np

# Add parent dir for imports
sys.path.insert(0, '.')

from world_model import (
    WorldModel, WorldModelConfig, WorldModelInference,
    AttentionDAGIntegration, create_world_model
)


def load_ground_truth(path: str = "bench/synthetic_org_data.json") -> Dict[str, Any]:
    """Load the generated data with hidden patterns."""
    with open(path) as f:
        return json.load(f)


def evaluate_skill_clusters(
    inference: WorldModelInference,
    patterns: List[Dict],
    threshold: float = 0.5
) -> Dict[str, Any]:
    """
    Evaluate: Are people with same skills close in embedding space?

    This tests if the model learned skill-based similarity without being told explicitly.
    """
    results = {
        "pattern_type": "skill_cluster",
        "total_patterns": 0,
        "detected": 0,
        "avg_similarity": 0.0,
        "details": []
    }

    skill_patterns = [p for p in patterns if p["pattern_type"] == "skill_cluster"]
    results["total_patterns"] = len(skill_patterns)

    total_sim = 0.0

    for pattern in skill_patterns:
        entities = pattern["entities"]
        if len(entities) < 2:
            continue

        # Get pairwise similarities for entities in this cluster
        sims = []
        for i, e1 in enumerate(entities):
            for e2 in entities[i+1:]:
                try:
                    related = inference.find_related_entities(e1, top_k=50, min_similarity=0.0)
                    sim = 0.0
                    for r in related:
                        if r.entity_id == e2:
                            sim = r.score
                            break
                    sims.append(sim)
                except (IndexError, KeyError):
                    continue  # Skip entities not in model

        if sims:
            avg_sim = np.mean(sims)
            total_sim += avg_sim

            detected = avg_sim >= threshold
            if detected:
                results["detected"] += 1

            results["details"].append({
                "description": pattern["description"],
                "num_entities": len(entities),
                "avg_similarity": avg_sim,
                "detected": detected
            })

    if results["total_patterns"] > 0:
        results["avg_similarity"] = total_sim / results["total_patterns"]
        results["detection_rate"] = results["detected"] / results["total_patterns"]

    return results


def evaluate_project_dependencies(
    inference: WorldModelInference,
    patterns: List[Dict],
    threshold: float = 0.5
) -> Dict[str, Any]:
    """
    Evaluate: Does the model predict project dependencies from shared members?

    Projects with shared members should be predicted as linked.
    """
    results = {
        "pattern_type": "project_dependency",
        "total_patterns": 0,
        "detected": 0,
        "avg_link_prob": 0.0,
        "details": []
    }

    dep_patterns = [p for p in patterns if p["pattern_type"] == "project_dependency"]
    results["total_patterns"] = len(dep_patterns)

    total_prob = 0.0

    for pattern in dep_patterns:
        # Extract the two projects from entities
        projects = [e for e in pattern["entities"] if e.startswith("project:")]
        if len(projects) < 2:
            continue

        p1, p2 = projects[0], projects[1]

        # Get link prediction
        try:
            prob = inference.predict_link_probability(p1, p2)
        except (IndexError, KeyError):
            continue
        total_prob += prob

        detected = prob >= threshold
        if detected:
            results["detected"] += 1

        results["details"].append({
            "description": pattern["description"],
            "projects": [p1, p2],
            "link_probability": prob,
            "ground_truth_strength": pattern["strength"],
            "detected": detected
        })

    if results["total_patterns"] > 0:
        results["avg_link_prob"] = total_prob / results["total_patterns"]
        results["detection_rate"] = results["detected"] / results["total_patterns"]

    return results


def evaluate_cross_team_bridges(
    inference: WorldModelInference,
    patterns: List[Dict],
    data: Dict
) -> Dict[str, Any]:
    """
    Evaluate: Does the model identify bridge people as central?

    People who bridge teams should have high predicted relevance.
    """
    results = {
        "pattern_type": "cross_team_bridge",
        "total_bridges": 0,
        "avg_rank": 0.0,
        "in_top_10": 0,
        "details": []
    }

    bridge_patterns = [p for p in patterns if p["pattern_type"] == "cross_team_bridge"]
    if not bridge_patterns:
        return results

    bridge_people = bridge_patterns[0]["entities"]
    results["total_bridges"] = len(bridge_people)

    # Get top predicted entities
    predictions = inference.predict_next_entities(top_k=50)
    pred_ids = [p.entity_id for p in predictions]

    total_rank = 0
    for bridge in bridge_people:
        if bridge in pred_ids:
            rank = pred_ids.index(bridge) + 1
            total_rank += rank
            if rank <= 10:
                results["in_top_10"] += 1
        else:
            rank = 999
            total_rank += rank

        results["details"].append({
            "person": bridge,
            "rank_in_predictions": rank if rank < 999 else "not in top 50"
        })

    if results["total_bridges"] > 0:
        results["avg_rank"] = total_rank / results["total_bridges"]
        results["top_10_rate"] = results["in_top_10"] / results["total_bridges"]

    return results


def evaluate_topic_affinity(
    inference: WorldModelInference,
    patterns: List[Dict],
    threshold: float = 0.4
) -> Dict[str, Any]:
    """
    Evaluate: Are people who write about same topics close in embedding space?
    """
    results = {
        "pattern_type": "topic_affinity",
        "total_patterns": 0,
        "detected": 0,
        "avg_similarity": 0.0,
        "details": []
    }

    topic_patterns = [p for p in patterns if p["pattern_type"] == "topic_affinity"]
    results["total_patterns"] = len(topic_patterns)

    total_sim = 0.0

    for pattern in topic_patterns[:10]:  # Sample for speed
        entities = pattern["entities"]
        if len(entities) < 2:
            continue

        # Get pairwise similarities
        sims = []
        for i, e1 in enumerate(entities[:5]):  # Sample
            try:
                related = inference.find_related_entities(e1, top_k=20, min_similarity=0.0)
                for e2 in entities[i+1:5]:
                    sim = 0.0
                    for r in related:
                        if r.entity_id == e2:
                            sim = r.score
                            break
                    sims.append(sim)
            except (IndexError, KeyError):
                continue

        if sims:
            avg_sim = np.mean(sims)
            total_sim += avg_sim

            detected = avg_sim >= threshold
            if detected:
                results["detected"] += 1

    evaluated = min(10, results["total_patterns"])
    if evaluated > 0:
        results["avg_similarity"] = total_sim / evaluated
        results["detection_rate"] = results["detected"] / evaluated

    return results


def run_baseline_comparison(data: Dict) -> Dict[str, Any]:
    """
    Baseline: What would random/simple heuristics achieve?
    """
    patterns = data["hidden_patterns"]

    # Random baseline: randomly guess connections
    random_accuracy = 1.0 / len(data["entities"]["people"])  # Chance of guessing right

    # Heuristic baseline: same team = connected
    team_groups = defaultdict(list)
    for person in data["entities"]["people"]:
        team_groups[person["team"]].append(person["id"])

    # For skill clusters, heuristic gets 0 (no explicit skill links)

    return {
        "random_baseline": random_accuracy,
        "team_heuristic_coverage": len(team_groups) / len(data["entities"]["people"]),
        "note": "World model should beat these baselines on hidden patterns"
    }


def load_entities_from_file(integration, path: str) -> int:
    """Load entities directly from synthetic data JSON file."""
    with open(path) as f:
        file_data = json.load(f)

    i = 0
    entities = file_data.get("entities", {})

    for entity_type, entity_list in entities.items():
        type_name = entity_type.rstrip("s").title()
        for entity in entity_list:
            entity_id = entity.get("id")
            if entity_id:
                integration.entity_to_idx[entity_id] = i
                integration.idx_to_entity[i] = entity_id
                integration.entity_types[entity_id] = type_name

                if type_name not in integration.type_to_idx:
                    type_idx = len(integration.type_to_idx)
                    integration.type_to_idx[type_name] = type_idx
                    integration.idx_to_type[type_idx] = type_name
                i += 1

    return i


def main():
    print("=" * 60)
    print("WORLD MODEL EVALUATION: Hidden Pattern Discovery")
    print("=" * 60)

    # Load ground truth
    print("\nLoading ground truth data...")
    data = load_ground_truth()
    patterns = data["hidden_patterns"]
    print(f"  {len(patterns)} hidden patterns to discover")

    # Initialize model
    print("\nInitializing world model...")
    config = WorldModelConfig(
        max_entities=100000,
        hidden_dim=128,
        num_layers=2,
    )
    model = create_world_model(config)

    # Check for checkpoint
    checkpoint_path = "checkpoints/world_model/final.pt"
    try:
        inference = WorldModelInference(model, config, checkpoint_path)
        print(f"  Loaded trained model from {checkpoint_path}")
    except FileNotFoundError:
        print(f"  No checkpoint found at {checkpoint_path}")
        print("  Using untrained model (random baseline)")
        inference = WorldModelInference(model, config)

    # Load entities from synthetic data file (same as training)
    print("\nLoading entities from synthetic data file...")
    try:
        load_entities_from_file(inference.integration, "bench/synthetic_org_data.json")

        # Re-encode state with loaded entities
        entity_ids, entity_types, edge_weights = inference.integration.get_attention_dag_as_tensor(1000)
        inference._current_z = inference.model.encode(
            entity_ids.unsqueeze(0).to(inference.device),
            entity_types.unsqueeze(0).to(inference.device),
            edge_weights.unsqueeze(0).to(inference.device),
        )
        inference._entity_embeds = inference.model.encoder.entity_encoder(
            entity_ids.unsqueeze(0).to(inference.device),
            entity_types.unsqueeze(0).to(inference.device),
        ).squeeze(0)

        print(f"  Loaded {len(inference.integration.entity_to_idx)} entities")
    except Exception as e:
        print(f"  Error loading entities: {e}")
        import traceback
        traceback.print_exc()
        return


    # Run evaluations
    print("\n" + "=" * 60)
    print("EVALUATION RESULTS")
    print("=" * 60)

    # Baseline
    print("\n--- Baseline (for comparison) ---")
    baseline = run_baseline_comparison(data)
    print(f"  Random guess accuracy: {baseline['random_baseline']:.4f}")

    # Skill clusters
    print("\n--- Skill Clusters ---")
    print("  (People with same skills should be similar)")
    skill_results = evaluate_skill_clusters(inference, patterns)
    print(f"  Patterns: {skill_results['total_patterns']}")
    print(f"  Detected: {skill_results['detected']}")
    print(f"  Detection rate: {skill_results.get('detection_rate', 0):.2%}")
    print(f"  Avg similarity: {skill_results['avg_similarity']:.3f}")

    # Project dependencies
    print("\n--- Project Dependencies ---")
    print("  (Projects with shared members should be linked)")
    dep_results = evaluate_project_dependencies(inference, patterns)
    print(f"  Patterns: {dep_results['total_patterns']}")
    print(f"  Detected: {dep_results['detected']}")
    print(f"  Detection rate: {dep_results.get('detection_rate', 0):.2%}")
    print(f"  Avg link probability: {dep_results['avg_link_prob']:.3f}")

    # Cross-team bridges
    print("\n--- Cross-Team Bridges ---")
    print("  (Bridge people should be predicted as important)")
    bridge_results = evaluate_cross_team_bridges(inference, patterns, data)
    print(f"  Total bridges: {bridge_results['total_bridges']}")
    print(f"  In top 10 predictions: {bridge_results['in_top_10']}")
    print(f"  Avg rank: {bridge_results['avg_rank']:.1f}")

    # Topic affinity
    print("\n--- Topic Affinity ---")
    print("  (People writing about same topics should be similar)")
    topic_results = evaluate_topic_affinity(inference, patterns)
    print(f"  Patterns evaluated: {min(10, topic_results['total_patterns'])}")
    print(f"  Detected: {topic_results['detected']}")
    print(f"  Detection rate: {topic_results.get('detection_rate', 0):.2%}")

    # Summary
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)

    total_detected = (
        skill_results['detected'] +
        dep_results['detected'] +
        bridge_results['in_top_10'] +
        topic_results['detected']
    )
    total_patterns = (
        skill_results['total_patterns'] +
        dep_results['total_patterns'] +
        bridge_results['total_bridges'] +
        min(10, topic_results['total_patterns'])
    )

    print(f"\nOverall detection: {total_detected}/{total_patterns} patterns")
    print(f"Overall rate: {total_detected/total_patterns:.2%}" if total_patterns > 0 else "N/A")

    if total_detected / total_patterns > baseline['random_baseline'] * 10:
        print("\n✓ Model significantly beats random baseline!")
        print("  The world model IS discovering hidden patterns.")
    else:
        print("\n✗ Model not much better than random.")
        print("  Need more training data or model tuning.")

    # Save detailed results
    results = {
        "skill_clusters": skill_results,
        "project_dependencies": dep_results,
        "cross_team_bridges": bridge_results,
        "topic_affinity": topic_results,
        "baseline": baseline,
        "summary": {
            "total_detected": total_detected,
            "total_patterns": total_patterns,
            "overall_rate": total_detected / total_patterns if total_patterns > 0 else 0
        }
    }

    with open("bench/evaluation_results.json", "w") as f:
        json.dump(results, f, indent=2, default=str)

    print("\nDetailed results saved to bench/evaluation_results.json")


if __name__ == "__main__":
    main()
