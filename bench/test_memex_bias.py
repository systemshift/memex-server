#!/usr/bin/env python3
"""
Test script for MemexBiasAdapter.

Tests the adapter with a synthetic graph and GPT-2 model.
Run: python test_memex_bias.py
"""

import json
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer

from memex_bias_adapter import MemexBiasAdapter, MemexConfig


def create_test_graph():
    """Create a synthetic memex subgraph for testing."""
    return {
        "nodes": [
            {"id": "alice", "type": "Person", "name": "Alice"},
            {"id": "bob", "type": "Person", "name": "Bob"},
            {"id": "carol", "type": "Person", "name": "Carol"},
            {"id": "techcorp", "type": "Company", "name": "TechCorp"},
            {"id": "datainc", "type": "Company", "name": "DataInc"},
        ],
        "edges": [
            {"source": "alice", "target": "techcorp", "weight": 0.9},  # Alice works at TechCorp
            {"source": "bob", "target": "techcorp", "weight": 0.85},   # Bob is CTO of TechCorp
            {"source": "alice", "target": "bob", "weight": 0.7},       # Alice knows Bob
            {"source": "carol", "target": "datainc", "weight": 0.8},   # Carol founded DataInc
            {"source": "techcorp", "target": "datainc", "weight": 0.6}, # TechCorp acquired DataInc
        ]
    }


def test_entity_detection():
    """Test that entities are found in text."""
    print("\n=== Test: Entity Detection ===")

    adapter = MemexBiasAdapter()
    adapter.load_from_dict(create_test_graph())

    # Check entities loaded
    print(f"Loaded {len(adapter.graph.entities)} entities")
    print(f"Aliases: {list(adapter.graph.aliases.keys())}")

    # Check weights loaded
    print(f"Loaded {len(adapter.graph.weights)} edge weights")

    assert len(adapter.graph.entities) == 5
    assert adapter.graph.get_weight("alice", "techcorp") == 0.9
    print("PASS")


def test_bias_matrix_building():
    """Test bias matrix construction."""
    print("\n=== Test: Bias Matrix Building ===")

    adapter = MemexBiasAdapter()
    adapter.load_from_dict(create_test_graph())

    tokenizer = AutoTokenizer.from_pretrained("gpt2")

    text = "Alice works at TechCorp where Bob is the CTO."
    bias = adapter.build_bias_for_text(text, tokenizer)

    print(f"Text: {text}")
    print(f"Bias matrix shape: {bias.shape}")
    print(f"Non-zero entries: {(bias != 0).sum().item()}")
    print(f"Max bias value: {bias.max().item():.3f}")

    # Should have some non-zero bias (Alice-TechCorp, Alice-Bob, Bob-TechCorp)
    assert (bias != 0).sum().item() > 0
    print("PASS")


def test_model_hooks():
    """Test hook registration on GPT-2."""
    print("\n=== Test: Model Hook Registration ===")

    adapter = MemexBiasAdapter()
    adapter.load_from_dict(create_test_graph())

    print("Loading GPT-2...")
    model = AutoModelForCausalLM.from_pretrained("gpt2")
    tokenizer = AutoTokenizer.from_pretrained("gpt2")
    model.eval()

    text = "Alice works at TechCorp where Bob is the CTO."

    # Register hooks
    hooks = adapter.register(model, text, tokenizer)
    print(f"Registered {len(hooks.handles)} hooks")

    # Run inference
    inputs = tokenizer(text, return_tensors="pt")
    with torch.no_grad():
        outputs = model.generate(
            inputs["input_ids"],
            max_new_tokens=20,
            do_sample=False,
            pad_token_id=tokenizer.eos_token_id
        )

    generated = tokenizer.decode(outputs[0], skip_special_tokens=True)
    print(f"Generated: {generated[:100]}...")

    # Clean up
    hooks.remove()
    print("Hooks removed")
    print("PASS")


def test_comparison():
    """Compare generation with and without memex bias."""
    print("\n=== Test: With vs Without Bias Comparison ===")

    # Use lower bias scale for GPT-2
    config = MemexConfig(bias_scale=0.5)
    adapter = MemexBiasAdapter(config=config)
    adapter.load_from_dict(create_test_graph())

    print("Loading GPT-2...")
    model = AutoModelForCausalLM.from_pretrained("gpt2")
    tokenizer = AutoTokenizer.from_pretrained("gpt2")
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
    model.eval()

    context = """Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice.
Carol founded DataInc, which was later acquired by TechCorp.
Bob now oversees the integration of DataInc into TechCorp's infrastructure."""

    question = "Who does Bob report to?"
    full_text = f"Context: {context}\n\nQuestion: {question}\n\nAnswer:"

    inputs = tokenizer(full_text, return_tensors="pt")

    # Without bias
    print("\nGenerating WITHOUT memex bias...")
    with torch.no_grad():
        outputs_baseline = model.generate(
            inputs["input_ids"],
            max_new_tokens=30,
            do_sample=False,
            pad_token_id=tokenizer.eos_token_id
        )
    baseline = tokenizer.decode(outputs_baseline[0], skip_special_tokens=True)
    baseline_answer = baseline.split("Answer:")[-1].strip()

    # With bias
    print("Generating WITH memex bias...")
    hooks = adapter.register(model, full_text, tokenizer)
    with torch.no_grad():
        outputs_biased = model.generate(
            inputs["input_ids"],
            max_new_tokens=30,
            do_sample=False,
            pad_token_id=tokenizer.eos_token_id
        )
    hooks.remove()
    biased = tokenizer.decode(outputs_biased[0], skip_special_tokens=True)
    biased_answer = biased.split("Answer:")[-1].strip()

    print(f"\nQuestion: {question}")
    print(f"Without bias: {baseline_answer[:80]}...")
    print(f"With bias:    {biased_answer[:80]}...")
    print(f"Different: {baseline_answer != biased_answer}")

    print("PASS")


def test_save_load():
    """Test saving and loading graph to file."""
    print("\n=== Test: Save/Load Graph ===")

    import tempfile
    import os

    graph_data = create_test_graph()

    # Save to temp file
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump(graph_data, f)
        temp_path = f.name

    try:
        # Load from file
        adapter = MemexBiasAdapter(temp_path)
        print(f"Loaded {len(adapter.graph.entities)} entities from file")
        assert len(adapter.graph.entities) == 5
        print("PASS")
    finally:
        os.unlink(temp_path)


if __name__ == "__main__":
    print("=" * 60)
    print("MemexBiasAdapter Tests")
    print("=" * 60)

    test_entity_detection()
    test_bias_matrix_building()
    test_save_load()
    test_model_hooks()
    test_comparison()

    print("\n" + "=" * 60)
    print("All tests passed!")
    print("=" * 60)
