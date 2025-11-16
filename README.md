# Memex - Layered Knowledge Graphs

[![Go Report Card](https://goreportcard.com/badge/github.com/systemshift/memex)](https://goreportcard.com/report/github.com/systemshift/memex)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/systemshift/memex)](https://github.com/systemshift/memex/releases)

> **⚠️ Project Status:** Early development. Basic graph storage works, building ingest layer next. See [ARCHITECTURE_PIVOT.md](ARCHITECTURE_PIVOT.md) for full architecture.

Memex stores knowledge in layers: raw sources + interpreted ontologies. Like git for knowledge graphs - content-addressed, verifiable, with interpretation history.

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
               ↓ recorded_in
Transaction Log: Git-like history (verifiable, portable)
```

**Key features:**
- Content-addressed sources (hash-based dedup)
- Multiple ontology interpretations per source
- Transaction log for audit trails
- Export/import for graph portability

## Use Cases

### Development Memory (Primary Use Case)

```bash
# Coming soon: Ingest git history
memex ingest ./myproject/.git

# Creates:
# - Source nodes (raw git log)
# - Ontology nodes (Commits, Authors, Files)
# - Links (authored_by, modifies, fixes)
```

**Value:** Agents understand project evolution, not just current state.

### Research Papers

```bash
# Ingest papers directory
memex ingest ./papers/

# Creates:
# - Source nodes (PDF text)
# - Ontology nodes (Papers, Authors, Concepts)
# - Links (cites, introduces, builds_on)
```

**Value:** Citation graph + concept relationships, not just keyword search.

## Architecture

```
memex (CLI) → HTTP API → memex-server (Go) → Neo4j
```

**Current status:**
- ✓ Neo4j connection
- ✓ Basic CRUD (nodes/links)
- ✓ Docker deployment
- 🚧 Content-addressed sources
- 🚧 Transaction log
- 🚧 LLM ingest

See [ARCHITECTURE_PIVOT.md](ARCHITECTURE_PIVOT.md) for full details.

## Quick Start

```bash
# Start Neo4j
docker run -d \
  -p 7687:7687 -p 7474:7474 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:5.15-community

# Build and start server
go build ./cmd/memex-server
NEO4J_URI=bolt://localhost:7687 \
NEO4J_USER=neo4j \
NEO4J_PASSWORD=password \
./memex-server

# Build CLI
go build ./cmd/memex

# Test
./memex create-node test-1 TestNode
./memex list-nodes
```

## Future: AI Agent Integration

(Coming after ingest layer is built)

**Python SDK** - Thin wrapper for HTTP API
**LangChain Tool** - Query graph from agents
**Query Engine** - Natural language → graph traversal

See [ARCHITECTURE_PIVOT.md](ARCHITECTURE_PIVOT.md) for roadmap.

## Documentation

- [Architecture](ARCHITECTURE_PIVOT.md) - Layered design and implementation plan

## Why Memex?

**vs RAG/Vector DBs:**
- Access to raw sources, not just chunks
- Structured relationships, not just similarity
- Multiple interpretations of same data

**vs Traditional Graph DBs:**
- LLM extracts entities automatically
- Content-addressed sources
- Transaction log for provenance

## License

BSD 3-Clause License. See [LICENSE](LICENSE).

---

**Note:** Memex v1.x was local-first `.mx` files. We're rebuilding v2.x as layered knowledge graphs. See `v1-stable` tag for old version.
