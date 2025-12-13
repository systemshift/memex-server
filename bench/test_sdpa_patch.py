#!/usr/bin/env python3
"""Test direct SDPA patching for memex bias injection."""

import torch
import torch.nn.functional as F
from transformers import AutoModelForCausalLM, AutoTokenizer

# Store original SDPA
_original_sdpa = F.scaled_dot_product_attention

# Global bias matrix (set before generation)
_memex_bias = None


def patched_sdpa(query, key, value, attn_mask=None, dropout_p=0.0, is_causal=False, scale=None, **kwargs):
    """SDPA with memex bias injection."""
    global _memex_bias


    if _memex_bias is not None and not is_causal:
        # Get dimensions - key determines the sequence length for attention scores
        batch, heads, q_len, head_dim = query.shape
        _, _, k_len, _ = key.shape

        # Create mask for attention scores: (batch, heads, q_len, k_len)
        new_mask = torch.zeros(batch, heads, q_len, k_len,
                               device=query.device, dtype=query.dtype)

        # Copy existing mask if present
        if attn_mask is not None:
            if attn_mask.dim() == 2:
                new_mask += attn_mask.unsqueeze(0).unsqueeze(0)
            elif attn_mask.dim() == 3:
                new_mask += attn_mask.unsqueeze(1)
            elif attn_mask.shape[-2:] == (q_len, k_len):
                new_mask += attn_mask.expand(batch, heads, q_len, k_len)

        # Add memex bias - only for the portions that overlap
        bias_q = min(q_len, _memex_bias.shape[0])
        bias_k = min(k_len, _memex_bias.shape[1])
        new_mask[:, :, :bias_q, :bias_k] += _memex_bias[:bias_q, :bias_k].to(query.dtype)

        attn_mask = new_mask

    return _original_sdpa(query, key, value, attn_mask=attn_mask, dropout_p=dropout_p,
                          is_causal=is_causal, scale=scale, **kwargs)


def create_test_graph():
    """Create synthetic memex graph bias matrix."""
    # Entities and their approximate token positions in our test text
    # "Alice is the CEO of TechCorp. Bob works as CTO..."
    # Alice=0, TechCorp=6, Bob=8, DataInc=~20, Carol=~15

    # For simplicity, create a small bias matrix
    # In real use, this would come from memex export
    size = 100  # Max sequence length we'll handle
    bias = torch.zeros(size, size)

    # Strong bias between related entities (token position approximations)
    # Alice (tokens 0-1) <-> TechCorp (tokens 5-6): weight 0.9
    # Bob (tokens 8-9) <-> TechCorp (tokens 5-6): weight 0.85
    # Alice <-> Bob: weight 0.7

    # Stronger bias values
    relationships = [
        ((0, 2), (5, 7), 2.0),    # Alice <-> TechCorp
        ((8, 10), (5, 7), 2.0),   # Bob <-> TechCorp
        ((0, 2), (8, 10), 1.5),   # Alice <-> Bob
    ]

    for (s1, e1), (s2, e2), weight in relationships:
        for i in range(s1, e1):
            for j in range(s2, e2):
                if i < size and j < size:
                    bias[i, j] = weight
                    bias[j, i] = weight

    return bias


def main():
    global _memex_bias

    print("=" * 60)
    print("Testing SDPA Patching for Memex Bias")
    print("=" * 60)

    # Patch SDPA
    F.scaled_dot_product_attention = patched_sdpa
    print("SDPA patched")

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

    # Test context
    context = """Alice is the CEO of TechCorp. Bob works as CTO at TechCorp and reports to Alice.
Carol founded DataInc, which was later acquired by TechCorp."""

    question = "Who does Bob report to?"
    prompt = f"Context: {context}\n\nQuestion: {question}\n\nAnswer:"

    inputs = tokenizer(prompt, return_tensors="pt").to(model.device)
    print(f"\nPrompt tokens: {inputs['input_ids'].shape[1]}")

    # Without bias
    print("\n--- Without memex bias ---")
    _memex_bias = None
    with torch.no_grad():
        outputs = model.generate(
            **inputs,
            max_new_tokens=30,
            do_sample=False,
            pad_token_id=tokenizer.eos_token_id
        )
    baseline = tokenizer.decode(outputs[0], skip_special_tokens=True)
    baseline_answer = baseline.split("Answer:")[-1].strip()
    print(f"Answer: {baseline_answer[:80]}")

    # With bias
    print("\n--- With memex bias ---")
    _memex_bias = create_test_graph().to(model.device).to(torch.float16)
    with torch.no_grad():
        outputs = model.generate(
            **inputs,
            max_new_tokens=30,
            do_sample=False,
            pad_token_id=tokenizer.eos_token_id
        )
    biased = tokenizer.decode(outputs[0], skip_special_tokens=True)
    biased_answer = biased.split("Answer:")[-1].strip()
    print(f"Answer: {biased_answer[:80]}")

    print(f"\n--- Results ---")
    print(f"Different: {baseline_answer != biased_answer}")

    # Restore original SDPA
    F.scaled_dot_product_attention = _original_sdpa
    print("\nSDPA restored")


if __name__ == "__main__":
    main()
