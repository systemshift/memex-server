# Memex - Layered Knowledge Graphs

[![Go Report Card](https://goreportcard.com/badge/github.com/systemshift/memex)](https://goreportcard.com/report/github.com/systemshift/memex)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/systemshift/memex)](https://github.com/systemshift/memex/releases)

Memex stores knowledge in layers: raw sources + interpreted ontologies. Like git for knowledge graphs - content-addressed, verifiable, with interpretation history.

**[Live Demo](https://memex.systems/demo.html)** | **[Website](https://memex.systems)**

## The Problem

RAG returns similar text chunks. AI agents need:
- Access to raw sources (not just interpretations)
- Structured relationships between entities
- Multiple views of the same data
- Verifiable provenance

## The Solution

**Two-layer architecture:**

```
Source Layer:  Raw data (content-addressed, immutable)
               ↓ extracted_from
Ontology Layer: Entities + Relationships (LLM-interpreted)
               ↓ attention edges
Query Layer:   Dynamic, usage-weighted connections
```

## Quick Start

```bash
# Start Neo4j
docker run -d \
  -p 7687:7687 -p 7474:7474 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:5.15-community

# Build and start server
go build ./cmd/memex-server
./memex-server

# Server runs on http://localhost:8080
```

## API Reference

### Node Operations
```bash
# Create a node
curl -X POST http://localhost:8080/api/nodes \
  -H "Content-Type: application/json" \
  -d '{"id": "person:john-doe", "type": "Person", "content": "Software engineer", "meta": {"name": "John Doe"}}'

# Get a node
curl http://localhost:8080/api/nodes/person:john-doe

# List nodes (with pagination)
curl "http://localhost:8080/api/nodes?limit=100&offset=0"

# Delete a node
curl -X DELETE http://localhost:8080/api/nodes/person:john-doe
```

### Link Operations
```bash
# Create a link
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"source": "person:john-doe", "target": "company:acme", "type": "WORKS_AT"}'

# Get links for a node
curl http://localhost:8080/api/nodes/person:john-doe/links
```

### Query Operations
```bash
# Search by text
curl "http://localhost:8080/api/query/search?q=john&limit=10"

# Filter by type
curl "http://localhost:8080/api/query/filter?type=Person&limit=100"

# Graph traversal
curl "http://localhost:8080/api/query/traverse?start=person:john-doe&depth=2"

# Get subgraph
curl "http://localhost:8080/api/query/subgraph?node_id=person:john-doe&depth=2"

# Attention-weighted subgraph
curl "http://localhost:8080/api/query/attention_subgraph?node_id=person:john-doe&min_weight=0.5"
```

### Attention Edges
```bash
# Update attention edge (co-occurrence/relevance)
curl -X POST http://localhost:8080/api/edges/attention \
  -H "Content-Type: application/json" \
  -d '{"source": "entity1", "target": "entity2", "query_id": "q123", "weight": 0.8}'

# Prune low-weight edges
curl -X POST http://localhost:8080/api/edges/attention/prune \
  -H "Content-Type: application/json" \
  -d '{"threshold": 0.1}'
```

### Graph Overview
```bash
# Get graph statistics and type distribution
curl http://localhost:8080/api/graph/map
```

## LLM Ingestion

The `bench/` directory contains tools for LLM-powered knowledge extraction:

```bash
cd bench
pip install -r requirements.txt

# Set your API key
export OPENAI_API_KEY=your-key

# Ingest with parallel workers
python ingest_ai.py --limit 1000 --concurrency 5
```

The ingestion pipeline:
1. Takes raw text documents
2. Uses LLM to extract entities and relationships
3. Creates content-addressed source nodes
4. Links entities to sources with `EXTRACTED_FROM` edges
5. Updates attention edges for co-occurring entities

## MCP Server (AI Agent Integration)

Memex includes an MCP (Model Context Protocol) server for AI agents:

```bash
cd mcp-server
pip install -r requirements.txt
python server.py
```

Provides tools for:
- `search_graph` - Search entities by name
- `get_node` - Retrieve node details
- `get_relationships` - Explore entity connections
- `traverse_graph` - Multi-hop traversal

## Benchmarking

HotpotQA benchmark suite for evaluating retrieval:

```bash
cd bench

# Agent-based retrieval
python benchmark_kg_agent.py --limit 100

# Baseline RAG comparison
python baseline_rag.py --limit 100
```

## Architecture

```
memex (CLI) ─┐
             │
HTTP API ────┼──→ memex-server (Go) ──→ Neo4j
             │
MCP Server ──┘
```

**Components:**
- `cmd/memex-server` - Go HTTP API server
- `cmd/memex` - CLI tool
- `mcp-server/` - Python MCP server for AI agents
- `bench/` - Ingestion pipeline and benchmarks
- `internal/server/` - Server implementation

## Why Memex?

**vs RAG/Vector DBs:**
- Access to raw sources, not just chunks
- Structured relationships, not just similarity
- Multiple interpretations of same data

**vs Traditional Graph DBs:**
- LLM extracts entities automatically
- Content-addressed sources
- Attention edges for query-time relevance

## Documentation

- [Architecture](ARCHITECTURE_PIVOT.md) - Design details
- [MCP Server](mcp-server/README.md) - AI agent integration
- [Benchmarks](bench/README.md) - Evaluation tools

## License

BSD 3-Clause License. See [LICENSE](LICENSE).
