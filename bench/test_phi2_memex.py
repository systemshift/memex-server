#!/usr/bin/env python3
"""Test MemexBiasAdapter with Phi-2 model."""

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
            {"source": "alice", "target": "techcorp", "weight": 0.9},
            {"source": "bob", "target": "techcorp", "weight": 0.85},
            {"source": "alice", "target": "bob", "weight": 0.7},
            {"source": "carol", "target": "datainc", "weight": 0.8},
            {"source": "techcorp", "target": "datainc", "weight": 0.6},
        ]
    }


def main():
    print("=" * 60)
    print("Testing MemexBiasAdapter with Phi-2")
    print("=" * 60)

    # Load model
    print("\nLoading Phi-2...")
    model_name = "microsoft/phi-2"

    tokenizer = AutoTokenizer.from_pretrained(model_name, trust_remote_code=True)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        model_name,
        torch_dtype=torch.float16,
        device_map="auto",
        trust_remote_code=True
    )
    model.eval()
    print(f"Model loaded: {model.config.model_type}")

    # Setup adapter with conservative bias
    config = MemexConfig(bias_scale=0.3)  # Conservative for testing
    adapter = MemexBiasAdapter(config=config)
    adapter.load_from_dict(create_test_graph())
    print(f"Loaded graph with {len(adapter.graph.entities)} entities")

    # Test context
    context = """Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice.
Carol founded DataInc, which was later acquired by TechCorp.
Bob now oversees the integration of DataInc into TechCorp's infrastructure."""

    questions = [
        "Who does Bob report to?",
        "Who is the CEO of TechCorp?",
        "Who founded DataInc?",
    ]

    for question in questions:
        print(f"\n{'='*60}")
        print(f"Question: {question}")
        print("=" * 60)

        prompt = f"Context: {context}\n\nQuestion: {question}\n\nAnswer:"
        inputs = tokenizer(prompt, return_tensors="pt").to(model.device)

        # Without bias
        print("\nWithout memex bias:")
        with torch.no_grad():
            outputs = model.generate(
                **inputs,
                max_new_tokens=30,
                do_sample=False,
                pad_token_id=tokenizer.eos_token_id
            )
        answer = tokenizer.decode(outputs[0], skip_special_tokens=True)
        baseline = answer.split("Answer:")[-1].strip()
        print(f"  {baseline[:100]}")

        # With bias
        print("\nWith memex bias:")
        hooks = adapter.register(model, prompt, tokenizer)
        with torch.no_grad():
            outputs = model.generate(
                **inputs,
                max_new_tokens=30,
                do_sample=False,
                pad_token_id=tokenizer.eos_token_id
            )
        hooks.remove()
        answer = tokenizer.decode(outputs[0], skip_special_tokens=True)
        biased = answer.split("Answer:")[-1].strip()
        print(f"  {biased[:100]}")

        print(f"\nDifferent: {baseline != biased}")

    print("\n" + "=" * 60)
    print("Test complete!")
    print("=" * 60)


if __name__ == "__main__":
    main()
