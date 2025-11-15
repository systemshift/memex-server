# Memex Architecture Pivot

**Date:** November 2024
**Status:** Planning Phase

## Vision

Memex is pivoting from a local-first personal knowledge graph to **structured knowledge infrastructure for agentic AI**. The core insight: AI agents need graph-structured context with rich ontologies, not just vector search over document chunks.

## The Problem We're Solving

### Current State of RAG
- Vector databases return semantically similar chunks
- No understanding of relationships between concepts
- Context assembly is naive (just concatenate similar docs)
- Agents can't reason about structure
- Every query starts from scratch
- No learning or memory between sessions

### What Memex Provides
1. **Ontology-Driven Knowledge Graphs**: Entity types and relationships defined upfront
2. **Multi-Modal Retrieval**: Vector search + graph traversal + LLM synthesis
3. **Lens System**: Different ontology views for different domains (code, research, legal, etc.)
4. **Contextual Memory**: Track what queries activate which concepts, build session memory
5. **Agent Integration**: Native tools for LangChain, CrewAI, and other frameworks

## Architecture Evolution

### Phase 1: Current State (Local DAG)
```
memex (CLI) → .mx file → Local DAG storage
```
- Single file storage
- Content-addressable chunks
- Local operations only

### Phase 2: Hybrid (Server + Client)
```
memex (CLI) → HTTP API → memex-server → Neo4j + Vectors
```
- CLI becomes API client
- Server handles graph operations
- Neo4j for graph + vector storage
- Support both self-hosted and cloud

### Phase 3: Enhanced (ECS + Activation)
```
memex (CLI) → HTTP API → memex-server → Neo4j + ECS + Activation Tracking
```
- ECS representations for model-agnostic embeddings
- Activation tracking for contextual memory
- Query-induced ontology evolution
- Multi-agent shared memory

## Key Components

### Memex Server (Go)
**Core Responsibilities:**
- Graph database operations (Neo4j)
- Ontology management and validation
- Entity extraction pipeline (LLM-powered)
- Query engine (vector + graph hybrid)
- Transaction logging (audit trail)
- Batch processing for large datasets

**Will Keep from Current Codebase:**
- `internal/memex/core/types.go` - Node/Link types (adapt)
- `internal/memex/transaction/*` - Transaction system (keep!)
- `internal/memex/core/version.go` - Version management (keep!)
- `pkg/module/*` - Module system (adapt for extractors)

**Will Remove:**
- `internal/memex/storage/store/*` - Replace with Neo4j driver
- `internal/memex/storage/rabin/*` - No longer needed for graph-first
- `internal/memex/editor.go` - CLI won't have editor
- `.mx` file format code - Using Neo4j

**Will Transform:**
- `cmd/memex/*` - Becomes thin API client
- `cmd/memexd/*` - Becomes main server with graph backend
- Repository interface - Graph operations API

### Memex CLI (Go → API Client)
**Responsibilities:**
- Connect to server
- Ingest documents/repos
- Define ontologies
- Run queries
- Configure lenses

**Commands:**
```bash
memex server start           # Start local server
memex connect <url>          # Connect to server
memex ontology create <yaml> # Define ontology
memex ingest <path>          # Ingest data
memex query <query>          # Query graph
memex lens list              # List available lenses
```

### Memex Python SDK (New)
**Responsibilities:**
- Thin wrapper around HTTP API
- LangChain integration
- Pydantic models for type safety
- Async support
- Streaming responses

**Example:**
```python
from memex import MemexClient
from langchain.tools import Tool

memex = MemexClient("http://localhost:8080")

# Use as LangChain tool
tool = Tool(
    name="knowledge_graph",
    func=memex.query,
    description="Query structured knowledge graph"
)
```

## Design Decisions

### Why Client-Server Split?
1. **Scale**: Large datasets can't fit in .mx files
2. **Performance**: Graph operations need server-side optimization
3. **Multi-user**: Teams need shared knowledge base
4. **LLM Integration**: Expensive operations should be batched
5. **Extensibility**: Modules can run server-side

### Why Go for Server?
1. **Current codebase**: Already in Go
2. **Performance**: Excellent for concurrent operations
3. **Deployment**: Single binary, no dependencies
4. **Ecosystem**: Good database drivers

### Why Python SDK?
1. **AI Ecosystem**: LangChain, CrewAI, AutoGPT all in Python
2. **User Base**: AI developers primarily use Python
3. **Integration**: Easy to wrap Go API

### Why Neo4j?
1. **Graph Native**: Purpose-built for graph operations
2. **Vector Support**: Native vector indexing (5.11+)
3. **Cypher**: Powerful query language
4. **Mature**: Production-ready, well-documented
5. **Flexible**: Can self-host or use cloud

### Why ECS Representation?
1. **Model Agnostic**: Not tied to specific embedding models
2. **Interpretable**: Structured components are human-readable
3. **Composable**: Can generate embeddings on-demand
4. **Future-Proof**: No re-embedding when models change

## Use Cases

### 1. Development Memory (Primary Launch Use Case)
```
Problem: Coding agents don't remember project history
Solution: Memex ingests git, terminal, LLM traces
Result: Agents learn from past attempts, don't repeat mistakes
```

**Entities:** Commit, Error, Edit, Terminal Session, LLM Prompt, Function, Test
**Relationships:** fixes, caused_by, attempts_to_fix, suggested_by, tests

### 2. Research Assistant
```
Problem: RAG returns disconnected paper chunks
Solution: Memex builds citation graph + concept ontology
Result: Agents understand research lineage and influence
```

**Entities:** Paper, Author, Concept, Method, Dataset
**Relationships:** cites, introduces, applies, authored_by

### 3. Legal/Compliance
```
Problem: Finding relevant case law requires expert knowledge
Solution: Memex maps legal precedents and statutes
Result: Agents navigate legal relationships accurately
```

**Entities:** Case, Statute, Regulation, Citation, Jurisdiction
**Relationships:** cites, overturns, distinguishes, applies

## Migration Path

### Step 1: Minimal Server (4 weeks)
- Keep core types
- Keep transaction system
- Add Neo4j integration
- Basic HTTP API
- CLI as API client

### Step 2: Ontology Layer (2 weeks)
- Define ontology schema
- Add validation
- Entity extraction with LLM
- Basic lens system

### Step 3: Python SDK (3 weeks)
- HTTP client wrapper
- LangChain integration
- Example notebooks
- Documentation

### Step 4: ECS Enhancement (3 weeks)
- ECS extractor
- Model-agnostic storage
- Multi-model support

### Step 5: Activation Tracking (4 weeks)
- Query tracking
- Activation profiles
- Contextual retrieval
- Session memory

## Backward Compatibility

### For Existing Users
- Provide migration tool: `.mx` → Neo4j export
- Document migration process
- Support old CLI commands during transition
- Clear deprecation timeline

### For Code
- Keep core interfaces stable
- Use feature flags for new features
- Semantic versioning (2.0.0 for major changes)

## Success Metrics

### Technical
- Query latency < 200ms (p95)
- Support 10M+ node graphs
- Handle 100+ concurrent users
- 99.9% uptime

### Product
- 1,000 MAU by Month 6
- 100 paying customers by Month 12
- 10 LangChain integrations in examples
- 5 community-built lenses

### Business
- $100K ARR by Year 1
- $1M ARR by Year 2
- Raise Series A by Year 2

## Open Questions

1. How to price? Per-user? Per-node? Per-query?
2. Should we support multiple graph DBs (Postgres, Neptune)?
3. How aggressive should activation tracking be?
4. What's the minimal viable lens system?
5. Self-hosted vs cloud-first strategy?

## References

- [Graph RAG Paper](https://arxiv.org/abs/2404.16130)
- [LangGraph Documentation](https://langchain-ai.github.io/langgraph/)
- [Neo4j Vector Index](https://neo4j.com/docs/cypher-manual/current/indexes-for-vector-search/)
- [ECS Architecture](https://github.com/systemshift/ecs-nmmo)

---

**This document captures the architectural vision as of November 2024. It should be updated as decisions are made and the product evolves.**
