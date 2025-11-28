# Memex Benchmark

Benchmarks Memex attention-weighted retrieval against standard RAG on HotpotQA.

## Setup

```bash
# Create venv
python3 -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Download dataset (if not already done)
python -c "from datasets import load_dataset; ds = load_dataset('hotpotqa/hotpot_qa', 'distractor'); ds.save_to_disk('./data')"

# Set API key
export OPENAI_API_KEY=your-key
```

## Running the Benchmark

### 1. Ingest paragraphs into Memex

```bash
# Make sure memex-server is running on :8080
python ingest.py --split validation
```

### 2. Build attention DAG (learning phase)

```bash
# Run first 3000 queries to learn attention patterns
python learn.py --limit 3000 --start 0
```

### 3. Benchmark retrieval

```bash
# Test on remaining queries (3000-7405)
python benchmark.py --limit 500 --start 3000
```

### 4. Compare to RAG baseline

```bash
python baseline_rag.py --limit 500 --start 3000
```

## Expected Results

After learning from ~3000 queries:

| Method | Precision | Recall | F1 |
|--------|-----------|--------|-----|
| Basic Search | ~0.15 | ~0.20 | ~0.17 |
| RAG (vector) | ~0.35 | ~0.45 | ~0.39 |
| **Memex Attention** | ~0.50+ | ~0.55+ | ~0.52+ |

The attention DAG learns which paragraphs are co-relevant, improving retrieval over time.

## Files

- `ingest.py` - Load HotpotQA paragraphs into Memex
- `learn.py` - Run queries through LLM to build attention DAG
- `benchmark.py` - Measure Memex retrieval precision/recall
- `baseline_rag.py` - Standard vector search baseline
- `results.json` - Memex benchmark results
- `results_rag.json` - RAG baseline results
