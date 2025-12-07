# Memex Architecture

**Date:** November 2025
**Status:** Active Development

## Vision

Memex provides **layered knowledge graphs** where raw data and ontological interpretation coexist. Like git for knowledge: content-addressed sources with interpretation layers on top.

## Core Concept

**Problem:** RAG returns similar text chunks. Agents need structured relationships and can't access raw sources.

**Solution:** Two-layer architecture:
- **Source Layer**: Raw data, content-addressed, immutable
- **Ontology Layer**: Interpreted entities and relationships, references sources

**Key insight:** One person's metadata is another's data. Don't hide sources behind interpretations.

## Layered Architecture

```
┌─────────────────────────────────────────┐
│ Transaction Log (Merkle DAG)            │  ← Git-like history
│  - Every operation is a commit          │
│  - Content-addressed                     │
│  - Verifiable, auditable                 │
└──────────────┬──────────────────────────┘
               │ records
               ↓
┌─────────────────────────────────────────┐
│ Graph Layer (Neo4j)                     │
│                                         │
│  ┌────────────────────────────────┐   │
│  │ Source Nodes (Layer 0)          │   │  ← Raw data
│  │  - Content-addressed (hash IDs) │   │
│  │  - Immutable                     │   │
│  │  - Format metadata               │   │
│  └────────────────────────────────┘   │
│               ↑                         │
│               │ extracted_from          │
│               │                         │
│  ┌────────────────────────────────┐   │
│  │ Ontology Nodes (Layer 1+)       │   │  ← Interpretations
│  │  - Domain entities              │   │
│  │  - Relationships                │   │
│  │  - Multiple interpretations OK  │   │
│  └────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### Source Layer
- Every raw input becomes a Source node
- ID = hash of content (content-addressed)
- Stored once, referenced many times
- Can be reinterpreted later without loss

**Example Source Node:**
```json
{
  "id": "sha256:abc123...",
  "type": "Source",
  "content": "<raw git log output>",
  "meta": {
    "format": "git-log",
    "ingested_at": "2025-11-17T10:00:00Z",
    "size_bytes": 15234
  }
}
```

### Ontology Layer
- LLM or parser extracts entities and relationships
- References source via `extracted_from` links
- Multiple interpretations can coexist
- Includes extraction metadata

**Example Ontology Nodes:**
```json
{
  "id": "commit-abc",
  "type": "Commit",
  "meta": {
    "hash": "abc123",
    "message": "fix: auth timeout",
    "author": "alice",
    "extracted_by": "gpt-4",
    "extraction_timestamp": "2025-11-17T10:01:00Z"
  }
}
```

**Extraction Link:**
```json
{
  "source": "commit-abc",
  "target": "sha256:abc123...",
  "type": "extracted_from",
  "meta": {
    "confidence": 0.95,
    "reasoning": "Extracted from git log line 42"
  }
}
```

### Transaction Log
- Every operation creates a transaction
- Transactions form Merkle DAG (like git)
- Content-addressed, verifiable
- Enables: audit trails, replication, time travel

**Example Transaction:**
```json
{
  "tx_hash": "sha256:def456...",
  "parent": "sha256:prev-tx...",
  "timestamp": "2025-11-17T10:00:00Z",
  "operations": [
    {
      "op": "create_source",
      "node_id": "sha256:abc123...",
      "format": "git-log"
    },
    {
      "op": "extract_ontology",
      "source_id": "sha256:abc123...",
      "created_nodes": ["commit-abc", "commit-def"],
      "extractor": "gpt-4",
      "model_version": "gpt-4-0613"
    }
  ]
}
```

## Current Implementation

**Status:** Basic graph storage working, building ingest layer next.

```
memex (CLI) → HTTP API → memex-server (Go) → Neo4j
```

**What works:**
- ✓ Neo4j connection and driver
- ✓ Basic node/link CRUD (create, get, list)
- ✓ HTTP API endpoints
- ✓ CLI as API client
- ✓ Docker deployment

**Building next:**
1. Content-addressed source storage
2. Transaction log for operations
3. Ingest endpoint with LLM extraction
4. Export/import for graph portability

**Future (not priority yet):**
- Lenses: Domain-specific ontology views
- ECS: Model-agnostic embeddings
- Activation tracking: Query-induced memory
- Vector search integration

## Design Principles

1. **Sources are first-class**: Never hide raw data behind interpretations
2. **Content-addressed**: Use hashes for immutable references
3. **Multiple interpretations**: Same source can have many ontology views
4. **Transaction log**: Every change is recorded and verifiable
5. **Export/import**: Graphs are portable via transactions + nodes

## Design Decisions

### Why Neo4j?
- Purpose-built for graph operations
- Native vector support (5.11+) for future use
- Mature, production-ready
- Can self-host or use cloud

### Why Transaction Log?
- Git-like history for knowledge graphs
- Enables: audit trails, replication, time travel, verification
- Already have transaction system in codebase

### Why Content-Addressed Sources?
- Immutable references (same content = same hash)
- Deduplication automatic
- Can reinterpret without re-ingesting
- Portable across servers

### Why LLM for Extraction?
- Universal parser (any data format)
- Emergent ontologies (no predefined schemas)
- Captures reasoning (stores why, not just what)
- Can improve over time (rerun with better models)

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

## Next Steps

1. **Content-addressed storage** - Implement SHA256 hashing for source nodes
2. **Transaction log** - Hook up existing transaction system to record operations
3. **Ingest endpoint** - `POST /api/ingest` with LLM extraction
4. **Export/import** - Serialize graph + transactions for portability

## Future Enhancements

**Lenses** - Domain-specific ontology views and extraction patterns
**ECS Representations** - Model-agnostic embeddings that can compile to any model
**Activation Tracking** - Learn from queries to improve retrieval
**Query Engine** - Natural language → graph traversal + LLM synthesis

---

**This architecture reflects our current understanding as of November 2024. It will evolve as we build and learn.**
