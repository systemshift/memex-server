# Memex - Knowledge Graphs for Agentic AI

[![Go Report Card](https://goreportcard.com/badge/github.com/systemshift/memex)](https://goreportcard.com/report/github.com/systemshift/memex)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/systemshift/memex)](https://github.com/systemshift/memex/releases)

> **⚠️ Project Status:** Memex is undergoing a significant architectural pivot. See [ARCHITECTURE_PIVOT.md](ARCHITECTURE_PIVOT.md) for details.

Memex provides structured knowledge graphs with rich ontologies for AI agents. While RAG gives agents document chunks, Memex gives them **understanding of relationships, context, and structure**.

## The Problem

AI agents today suffer from context amnesia:
- They retrieve similar documents but don't understand relationships
- They can't reason about structure or causality
- They start every query from scratch
- They have no memory between sessions
- They can't learn from their own history

## The Solution

Memex is a **knowledge infrastructure layer** that sits between your data and your agents:

```
Your Data → Memex → Structured Graph → AI Agents
```

**What Memex provides:**
1. **Ontology-Driven Graphs**: Define entity types and relationships for your domain
2. **Hybrid Retrieval**: Vector search + graph traversal + LLM synthesis
3. **Lens System**: Different ontology views (code, research, legal, business)
4. **Contextual Memory**: Track query patterns and concept activations
5. **Agent Integration**: Native tools for LangChain, CrewAI, AutoGPT

## Use Cases

### Development Memory (Launch Use Case)
Give coding agents memory of your project history:

```bash
# Ingest your development history
memex ingest-repo ./myproject
memex ingest-terminal ~/.zsh_history
memex ingest-llm-traces ./cursor_logs

# Now agents can query:
# "What approaches failed when fixing auth bugs?"
# "Show me terminal commands that resolved database errors"
# "What did the LLM suggest for similar issues?"
```

**Entities:** Commit, Error, Edit, Terminal Session, LLM Prompt, Function, File
**Relationships:** fixes, caused_by, attempts_to_fix, suggested_by, resolves

**Value:** Agents don't repeat failed attempts. They learn from project history.

### Research Assistant
Build citation graphs and concept ontologies:

```python
from memex import MemexClient

memex = MemexClient()
memex.ingest_papers("./research_papers/", lens="academic")

# Query with structure
result = memex.query(
    "How does Einstein's work influence quantum computing?",
    lens="academic",
    depth=3
)
# Returns: Citation path + concept relationships + key papers
```

**Entities:** Paper, Author, Concept, Method, Dataset
**Relationships:** cites, introduces, applies_to, authored_by

### Legal/Compliance
Navigate legal precedents and statutes:

```python
memex.query(
    "Find precedents for GDPR violations in EU",
    lens="legal",
    jurisdiction="EU"
)
# Returns: Relevant cases + legal relationships + statute connections
```

## Architecture

### Client-Server Model

```
┌─────────────────────────────────┐
│  Memex CLI / Python SDK         │  ← Your interface
└────────────┬────────────────────┘
             │ HTTP/gRPC API
             ↓
┌─────────────────────────────────┐
│      Memex Server (Go)          │  ← Core engine
│  ├─ Graph DB (Neo4j)            │
│  ├─ Ontology Engine             │
│  ├─ Entity Extraction (LLM)     │
│  ├─ Query Engine                │
│  └─ Transaction Log             │
└─────────────────────────────────┘
```

**Why client-server?**
- Scale to large datasets (millions of nodes)
- Share knowledge across teams
- Batch expensive LLM operations
- Professional deployment options

## Quick Start

### Self-Hosted (Docker)

```bash
# Clone repo
git clone https://github.com/systemshift/memex
cd memex

# Start server + Neo4j
docker-compose up -d

# Install CLI
go install ./cmd/memex

# Connect
memex connect http://localhost:8080

# Create ontology
memex ontology create dev-history.yaml

# Ingest data
memex ingest-repo ./myproject

# Query
memex query "Show me auth-related errors from last month"
```

### Cloud Hosted (Coming Soon)

```bash
# Connect to managed instance
memex connect https://api.memex.cloud

# Same commands, zero ops
```

## Integration with AI Agents

### LangChain

```python
from memex import MemexClient
from langchain.tools import Tool
from langchain.agents import create_react_agent

memex = MemexClient("http://localhost:8080")

# Create tool
memex_tool = Tool(
    name="knowledge_graph",
    description="Query structured knowledge graph with relationships",
    func=lambda q: memex.query(q, lens="dev-history")
)

# Use in agent
agent = create_react_agent(
    llm=llm,
    tools=[memex_tool, web_search, calculator]
)

result = agent.run("How did we fix the auth timeout issue?")
```

### CrewAI

```python
from crewai import Agent, Task, Crew
from memex.crewai import MemexTool

researcher = Agent(
    role="Research Analyst",
    tools=[MemexTool(lens="academic")],
    goal="Analyze research papers"
)

task = Task(
    description="Find papers about quantum entanglement",
    agent=researcher
)

crew = Crew(agents=[researcher], tasks=[task])
result = crew.kickoff()
```

## The Lens System

**Lenses** define how to view your data:

```yaml
# dev-history-lens.yaml
name: Development History
description: Track code changes, errors, and solutions

entities:
  Commit:
    properties: [hash, message, author, timestamp]
  Error:
    properties: [type, message, location, severity]
  Fix:
    properties: [approach, success, reasoning]

relationships:
  - Commit fixes Error
  - Error caused_by Code
  - Fix attempts Error
  - LLM suggests Fix

queries:
  error_resolution: "Path from Error to successful Fix"
  similar_issues: "Errors with similar Fix patterns"
  effectiveness: "LLM suggestion success rate"
```

**Use different lenses for different domains:**
- `dev-history`: Code, errors, fixes
- `academic`: Papers, citations, concepts
- `legal`: Cases, statutes, precedents
- `business`: Companies, markets, relationships

## Roadmap

### Phase 1: Core Server (Current)
- [x] Transaction system (from v1)
- [x] Core graph types
- [ ] Neo4j integration
- [ ] HTTP API
- [ ] CLI as API client
- [ ] Basic ontology validation

### Phase 2: Entity Extraction
- [ ] LLM-powered entity extraction
- [ ] Ontology-driven prompts
- [ ] Batch processing
- [ ] Template learning (for large datasets)

### Phase 3: Python SDK
- [ ] HTTP client wrapper
- [ ] LangChain integration
- [ ] Pydantic models
- [ ] Async support
- [ ] Example notebooks

### Phase 4: Enhanced Features
- [ ] ECS representations (model-agnostic embeddings)
- [ ] Activation tracking (contextual memory)
- [ ] Multi-model ensemble queries
- [ ] Query-induced ontology evolution

### Phase 5: Production Ready
- [ ] Multi-tenancy
- [ ] Access control
- [ ] Observability
- [ ] Cloud deployment
- [ ] Enterprise features

## Documentation

- [Architecture Pivot](ARCHITECTURE_PIVOT.md) - Vision and design decisions
- [API Documentation](docs/API.md) - HTTP API reference
- [Ontology Guide](docs/ONTOLOGY.md) - Creating custom ontologies (coming soon)
- [Lens Development](docs/LENSES.md) - Building domain-specific lenses (coming soon)
- [Agent Integration](docs/AGENTS.md) - Using with LangChain/CrewAI (coming soon)
- [Development Guide](docs/DEVELOPMENT.md) - Contributing to Memex

## Why Memex?

### vs Vector Databases
| Vector DBs | Memex |
|-----------|-------|
| Semantic similarity | Semantic + structural relationships |
| Returns chunks | Returns subgraphs |
| Static retrieval | Dynamic context assembly |
| No reasoning | Graph traversal + LLM synthesis |
| No memory | Session and usage memory |

### vs Traditional Graph DBs
| Graph DBs | Memex |
|-----------|-------|
| Manual schema | LLM-extracted entities |
| Expert queries | Natural language + lenses |
| No vector search | Hybrid vector + graph |
| Static | Learns from usage |
| General purpose | Optimized for AI agents |

### vs LangChain/LangGraph
**Not competing, complementing:**
- LangChain/LangGraph: Agent orchestration ("how to think")
- Memex: Knowledge structure ("what to think about")
- Use together: Agents call Memex as a tool

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Priority areas:**
- Lens definitions for different domains
- Integration examples (LangChain, CrewAI, etc.)
- Entity extraction patterns
- Performance optimizations
- Documentation improvements

## Community

- [GitHub Discussions](https://github.com/systemshift/memex/discussions) - Ask questions, share ideas
- [Discord](https://discord.gg/memex) - Real-time chat (coming soon)
- [Twitter](https://twitter.com/memex_ai) - Updates and announcements (coming soon)

## License

Memex is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for details.

## Acknowledgments

Built on the shoulders of giants:
- Neo4j for graph database
- LangChain for agent framework inspiration
- The ECS architecture pattern from game engines
- Research on GraphRAG and knowledge graphs

---

**Status Note:** Memex v1.x was a local-first personal knowledge tool. We're pivoting to v2.x as knowledge infrastructure for AI agents. The current codebase is in transition. See [ARCHITECTURE_PIVOT.md](ARCHITECTURE_PIVOT.md) for the full story.

If you're interested in the v1.x branch, see the `v1-stable` tag.
