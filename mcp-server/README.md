# Memex MCP Server

Model Context Protocol (MCP) server for Memex knowledge graph. Allows AI agents to query and explore the Memex graph using standard MCP tooling.

## What is MCP?

[Model Context Protocol](https://modelcontextprotocol.io) is a standard protocol for connecting AI agents to external data sources and tools. Think of it like a USB port for AI memory.

With MCP, any compatible agent (Claude Desktop, custom agents, future tools) can connect to your Memex server and query your knowledge graph.

## Architecture

```
┌─────────────┐
│ AI Agent    │  (Claude Desktop, custom agent, etc.)
│ (MCP Client)│
└──────┬──────┘
       │ MCP Protocol (stdio)
       │
┌──────▼──────┐
│ MCP Server  │  (This Python server)
│ server.py   │
└──────┬──────┘
       │ HTTP API
       │
┌──────▼──────┐
│ Memex       │  (Go server)
│ HTTP Server │
└──────┬──────┘
       │
┌──────▼──────┐
│ Neo4j       │
│ Graph DB    │
└─────────────┘
```

## Setup

1. **Install Python dependencies**:
   ```bash
   cd mcp-server
   pip install -r requirements.txt
   ```

2. **Make sure Memex server is running**:
   ```bash
   # In main memex directory
   ./memex-server
   ```

3. **Configure Claude Desktop** (or other MCP client):

   Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or
   `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

   ```json
   {
     "mcpServers": {
       "memex": {
         "command": "python",
         "args": ["/absolute/path/to/memex/mcp-server/server.py"],
         "env": {
           "MEMEX_URL": "http://localhost:8080"
         }
       }
     }
   }
   ```

4. **Restart Claude Desktop**

## Available Tools

The MCP server exposes 6 tools that agents can use:

### 1. `search_nodes`
Full-text search across all nodes.
```
Query: "beacon"
→ Finds all nodes containing "beacon" in ID, type, properties, or content
```

### 2. `filter_nodes`
Structured filtering by type and properties.
```
Types: ["Person"]
→ Returns all Person nodes

Types: ["Concept"], Property: "status" = "active"
→ Returns active Concept nodes
```

### 3. `traverse_graph`
Graph traversal from a starting node.
```
Start: "systemshift", Depth: 2
→ Returns all nodes within 2 hops of systemshift

Start: "systemshift", Relationship Types: ["AUTHORED", "FIXED"]
→ Only follows AUTHORED and FIXED edges
```

### 4. `get_node`
Get full details of a specific node.
```
Node ID: "sha256:abc123..."
→ Returns complete node data (ID, type, content, metadata, timestamps)
```

### 5. `get_node_links`
Get all outgoing relationships from a node.
```
Node ID: "systemshift"
→ Returns all links: [{Source, Target, Type, Meta}, ...]
```

### 6. `list_all_nodes`
List all node IDs in the graph (use sparingly).
```
→ Returns: ["systemshift", "dag-time", "beacon-prototype", ...]
```

## Usage Example

Once connected to Claude Desktop:

```
User: "What has systemshift worked on?"

Claude: [Calls traverse_graph with start="systemshift", depth=1]
        [Gets back: dag-time, socketagentd, beacon-prototype, ...]

        "systemshift has worked on several projects:
         - dag-time: A time tracking tool with fuzzy anchoring
         - socketagentd: A Docker agent with socket API
         - beacon-prototype: An experimental feature
         ..."
```

The agent automatically decides which tools to use and how to combine results.

## Environment Variables

- `MEMEX_URL`: Base URL of Memex HTTP server (default: `http://localhost:8080`)

## Development

Test the MCP server directly:
```bash
python server.py
# Send MCP protocol messages via stdin
```

Or use the MCP Inspector:
```bash
npx @modelcontextprotocol/inspector python server.py
```

## Why MCP vs Direct API?

**Direct API** (curl, custom code):
- ❌ Every agent needs custom integration code
- ❌ No standard protocol
- ❌ Tight coupling to your API

**MCP**:
- ✅ Any MCP agent can connect instantly
- ✅ Standard protocol (like USB for AI)
- ✅ Growing ecosystem (Claude Desktop, others)
- ✅ Future-proof

## Next Steps

1. **Add more tools**: Ingest content, create nodes/links, delete nodes
2. **Add resources**: Expose popular nodes as MCP resources
3. **Add prompts**: Reusable query templates
4. **Authentication**: Add API key support for multi-user deployments
5. **Caching**: Cache frequent queries

## Learn More

- [MCP Specification](https://spec.modelcontextprotocol.io)
- [MCP Python SDK](https://github.com/modelcontextprotocol/python-sdk)
- [Claude Desktop MCP Guide](https://modelcontextprotocol.io/quickstart/user)
