# Attention-Aware DAG Architecture

## Overview

Memex implements a novel **attention-aware DAG** that acts as persistent memory for attention patterns computed by ML models. This enables sparse, learned retrieval that improves with use and persists across model changes.

## The Problem: O(n²) Attention Doesn't Scale

Standard transformer attention:
```
Query → Attend to ALL nodes → O(n²) memory/compute
```

With 1M documents:
- Full attention: 1M × 1M = 1 trillion pairs
- Doesn't fit in memory
- Too slow for retrieval

## The Solution: Crystallized Attention in DAG

Instead of computing attention from scratch every query, **learn which nodes tend to be co-attended** and persist this structure in the DAG.

```
Query 1: ML model computes attention → Memex persists patterns
Query 2: ML model computes attention → Memex reinforces patterns
Query 3: Use DAG patterns → Sparse retrieval (no full attention needed)
```

### Key Insight

The DAG becomes a **crystallized attention map** that accumulates knowledge across:
- Multiple queries
- Multiple models
- Multiple users

## Architecture

### Three Layers

```
┌─────────────────────────────────────────┐
│  ML Pipeline (Python)                   │
│  - Computes transient attention         │
│  - Uses DAG for sparse retrieval        │
└──────────────┬──────────────────────────┘
               │ HTTP API
┌──────────────▼──────────────────────────┐
│  Memex Attention Layer                  │
│  - Persists ATTENDED edges              │
│  - Running average weights              │
│  - Sparse traversal                     │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│  Memex Data Layer                       │
│  - Content-addressed nodes (SHA256)     │
│  - Graph structure (Neo4j)              │
│  - DAG integrity (tombstones)           │
└─────────────────────────────────────────┘
```

## ATTENDED Edge Structure

Attention edges are stored as special relationships:

```cypher
(NodeA)-[r:LINK {
  type: 'ATTENDED',
  properties: {
    "weight": 0.85,              // Running average of attention weights
    "query_count": 12,           // How many times this pair co-attended
    "last_updated": "2025-11-23T...",
    "last_query_id": "abc123"
  }
}]->(NodeB)
```

### Weight Computation

When updating an existing edge:
```
new_weight = (old_weight × query_count + new_weight) / (query_count + 1)
```

This creates a **running average** that stabilizes over time while still adapting to new patterns.

## API Endpoints

### 1. Update Attention Edge

**Endpoint**: `POST /api/edges/attention`

**Purpose**: ML pipeline persists attention weights after computing them.

**Request**:
```json
{
  "source": "wiki:Python_(programming_language)",
  "target": "guido",
  "query_id": "sha256:abc123",
  "weight": 0.92
}
```

**Response**:
```json
{
  "success": true,
  "message": "Attention edge updated",
  "source": "wiki:Python_(programming_language)",
  "target": "guido",
  "weight": 0.92
}
```

**Behavior**:
- If edge exists: Updates weight (running average), increments query_count
- If edge doesn't exist: Creates new edge with weight and query_count=1

### 2. Query Attention Subgraph

**Endpoint**: `GET /api/query/attention_subgraph`

**Purpose**: Retrieve sparse subgraph following learned attention patterns.

**Parameters**:
- `start` (required): Starting node ID
- `min_weight` (default: 0.5): Minimum attention weight threshold
- `max_nodes` (default: 50): Maximum nodes to return

**Example**:
```bash
GET /api/query/attention_subgraph?start=wiki:Python&min_weight=0.7&max_nodes=50
```

**Response**:
```json
{
  "nodes": [
    {"ID": "wiki:Python", "Type": "WikiPage", ...},
    {"ID": "guido", "Type": "Person", ...}
  ],
  "edges": [
    {
      "source": "wiki:Python",
      "target": "guido",
      "type": "ATTENDED",
      "meta": {
        "weight": 0.92,
        "query_count": 15
      }
    }
  ],
  "stats": {
    "node_count": 2,
    "edge_count": 1,
    "depth": 2
  }
}
```

### 3. Prune Weak Edges

**Endpoint**: `POST /api/edges/attention/prune`

**Purpose**: Remove low-quality attention edges to maintain DAG health.

**Parameters**:
- `min_weight` (default: 0.3): Remove edges below this weight
- `min_query_count` (default: 2): Remove edges seen fewer times

**Example**:
```bash
POST /api/edges/attention/prune?min_weight=0.3&min_query_count=2
```

**Response**:
```json
{
  "success": true,
  "deleted_count": 42,
  "message": "Pruned 42 weak attention edges"
}
```

## Usage Workflow

### Step 1: ML Model Computes Attention

```python
import torch

# Your graph transformer
query_embedding = model.encode(query_text)
node_embeddings = model.encode([node.content for node in subgraph.nodes])

# Compute attention weights
attention_logits = torch.matmul(query_embedding, node_embeddings.T)
attention_weights = torch.softmax(attention_logits, dim=-1)

# Get high-attention pairs
for i, j in high_weight_pairs:
    if attention_weights[i, j] > 0.5:
        yield (nodes[i].id, nodes[j].id, attention_weights[i, j].item())
```

### Step 2: Persist to Memex

```python
import requests
import hashlib

def persist_attention_patterns(query, attention_pairs):
    query_id = hashlib.sha256(query.encode()).hexdigest()[:16]

    for source, target, weight in attention_pairs:
        requests.post("http://localhost:8080/api/edges/attention", json={
            "source": source,
            "target": target,
            "query_id": query_id,
            "weight": weight
        })
```

### Step 3: Future Queries Use Learned Patterns

```python
def query_with_sparse_attention(query, start_node):
    # Instead of full O(n²) attention, use learned patterns
    response = requests.get(
        "http://localhost:8080/api/query/attention_subgraph",
        params={"start": start_node, "min_weight": 0.7, "max_nodes": 50}
    )

    subgraph = response.json()

    # Only attend within this sparse subgraph
    # O(k) where k = avg attention neighbors, not O(n²)
    return model.generate(query, subgraph['nodes'])
```

## Benefits

### 1. Sparse Retrieval

**Without attention DAG**:
- Must consider all 1M documents
- O(n²) attention computation
- Doesn't scale

**With attention DAG**:
- Only considers ~50 high-attention nodes
- O(k × n) where k << n
- Scales to millions of documents

### 2. Model-Agnostic

Attention patterns persist across:
- Different model architectures (BERT → GPT → Llama)
- Model updates/retraining
- Fine-tuning runs

The DAG accumulates knowledge independent of the specific model.

### 3. Learns Over Time

```
Query 1:  Python → Guido (weight: 0.85, count: 1)
Query 5:  Python → Guido (weight: 0.91, count: 5)   ← Reinforced
Query 20: Python → Guido (weight: 0.94, count: 20)  ← Strong pattern
```

Frequently co-attended nodes develop high-weight edges.

### 4. Adaptive to Queries

Different query types develop different attention patterns:

```
"Who created Python?" → Python -[0.95]-> Guido
"Python performance"  → Python -[0.88]-> CPython -[0.82]-> optimization
"Python syntax"       → Python -[0.91]-> language_features
```

The DAG learns multiple attention neighborhoods per node.

## Integration with Graph Transformers

### Standard Graph Transformer

```python
# Attention computed from scratch every query
attention_matrix = compute_attention(query, all_nodes)  # O(n²)
output = transformer(nodes, attention_bias=graph_structure)
```

### Attention-DAG-Guided Transformer

```python
# Use DAG to get sparse subgraph
subgraph = memex.get_attention_subgraph(start_node, min_weight=0.7)  # O(k)

# Only compute attention within subgraph
attention_matrix = compute_attention(query, subgraph.nodes)  # O(k²), k << n
output = transformer(subgraph.nodes, attention_bias=subgraph.edges)

# Persist new patterns back to DAG
memex.update_attention_edges(query_id, attention_matrix)
```

## Adaptive Chunking (Future)

The attention DAG can guide **dynamic content chunking**:

### Problem
Fixed-size chunks (e.g., 512 tokens) don't align with semantic boundaries.

### Solution
Use attention patterns to chunk documents:

```python
# High-attention regions → fine-grained chunks
# Low-attention regions → coarse chunks

def adaptive_chunk(document, attention_history):
    # Get historical attention map for this document
    attention_map = memex.get_attention_history(document_id)

    chunks = []
    for span in document:
        if attention_map[span] > 0.8:
            # Important region: small chunks (100 tokens)
            chunks.extend(split(span, chunk_size=100))
        else:
            # Less important: large chunks (500 tokens)
            chunks.extend(split(span, chunk_size=500))

    return chunks
```

### Benefits
- Important content gets finer granularity
- Reduces total chunks (sparse representation)
- Adapts to usage patterns

## Maintenance

### Edge Pruning Strategy

Run periodically (e.g., daily):

```python
# Remove noise edges
memex.prune_attention_edges(
    min_weight=0.3,       # Below this = noise
    min_query_count=2     # Seen only once = spurious
)
```

### Monitoring

Track these metrics:
- Average attention weight per node
- Edge count growth rate
- Query latency (should decrease as DAG learns)

## Comparison to Alternatives

| Approach | Memory | Latency | Learns | Persists |
|----------|--------|---------|--------|----------|
| **Full attention** | O(n²) | High | No | No |
| **RAG (vector search)** | O(n) | Medium | No | Static |
| **Attention DAG** | O(n + e) | Low | Yes | Yes |

**Attention DAG unique advantage**: Learns which content co-occurs in attention, enabling increasingly sparse retrieval over time.

## Example: Wikipedia Query

```
Query: "Who created Python and why?"

Step 1: Traditional approach
- Search all 6M Wikipedia articles
- Embed query + all articles
- O(6M) embeddings + O(36T) attention pairs
- Doesn't fit in memory

Step 2: RAG approach
- Vector search → top 10 articles
- Embed query + 10 articles
- O(10) embeddings + O(100) attention pairs
- Works but static (no learning)

Step 3: Attention DAG approach
- Query DAG from "Python" node
- DAG learned: Python -[0.95]-> Guido -[0.88]-> design_philosophy
- Get 3 highly relevant nodes (not 10 random ones)
- O(3) embeddings + O(9) attention pairs
- AND it learned from past queries!
```

## Implementation Details

### Neo4j Storage

ATTENDED edges stored as `LINK` relationships with `type='ATTENDED'`:

```cypher
// Create/update attention edge
MATCH (s:Node {id: $source}), (t:Node {id: $target})
MERGE (s)-[r:LINK {type: 'ATTENDED'}]->(t)
SET r.properties = $metadata
```

### Weight Averaging in Go

```go
// Compute running average
currentWeight := meta["weight"].(float64)
currentCount := meta["query_count"].(float64)

newWeight := (currentWeight*currentCount + weight) / (currentCount + 1)
newCount := currentCount + 1
```

### No APOC Dependency

Implementation uses standard Cypher, no APOC plugin required.

## Future Enhancements

### 1. Typed Attention Heads

Different edge types for different attention heads:

```
Python -[ATTENDED:semantic]-> Guido      (weight: 0.9)
Python -[ATTENDED:temporal]-> Python_3.0 (weight: 0.8)
```

### 2. Contextual Attention

Store query context with edges:

```json
{
  "weight": 0.85,
  "contexts": [
    {"query_type": "creator", "weight": 0.95},
    {"query_type": "technical", "weight": 0.70}
  ]
}
```

### 3. Attention Decay

Older patterns decay over time:

```go
// Apply time-based decay
age := time.Since(lastUpdated).Hours() / 24.0  // days
decayedWeight := weight * math.Exp(-decay_rate * age)
```

## References

- Graph Transformers: [Dwivedi & Bresson, 2020](https://arxiv.org/abs/2012.09699)
- Sparse Attention: [Child et al., 2019](https://arxiv.org/abs/1904.10509)
- Content-Addressed Storage: [IPFS Whitepaper](https://ipfs.io/)

## See Also

- [deletion-policy.md](deletion-policy.md) - DAG integrity via tombstones
- [examples/attention_dag_demo.py](../examples/attention_dag_demo.py) - Working demo
- [examples/graph_transformer_example.py](../examples/graph_transformer_example.py) - Transformer integration
