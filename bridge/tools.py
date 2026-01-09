"""
Memex tools for LLM function calling.

These tools call the Go Memex server endpoints.
"""

import os
import httpx
from typing import Dict, Any, List
from providers import Tool

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")


# Tool definitions
TOOLS = [
    Tool(
        name="search",
        description="Full-text search across all nodes in the knowledge graph",
        parameters={
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Search terms"},
                "limit": {"type": "integer", "description": "Max results (default 10)"}
            },
            "required": ["query"]
        }
    ),
    Tool(
        name="get_node",
        description="Get full details of a specific node by ID",
        parameters={
            "type": "object",
            "properties": {
                "id": {"type": "string", "description": "Node ID (e.g. person:001)"}
            },
            "required": ["id"]
        }
    ),
    Tool(
        name="get_links",
        description="Get all relationships for a node",
        parameters={
            "type": "object",
            "properties": {
                "id": {"type": "string", "description": "Node ID"}
            },
            "required": ["id"]
        }
    ),
    Tool(
        name="traverse",
        description="Traverse graph from a starting node",
        parameters={
            "type": "object",
            "properties": {
                "start": {"type": "string", "description": "Starting node ID"},
                "depth": {"type": "integer", "description": "Hops to follow (default 2)"}
            },
            "required": ["start"]
        }
    ),
    Tool(
        name="filter",
        description="Filter nodes by type",
        parameters={
            "type": "object",
            "properties": {
                "type": {"type": "string", "description": "Node type (Person, Document, etc.)"},
                "limit": {"type": "integer", "description": "Max results (default 20)"}
            },
            "required": ["type"]
        }
    ),
    Tool(
        name="list_nodes",
        description="List all nodes of a specific type",
        parameters={
            "type": "object",
            "properties": {
                "type": {"type": "string", "description": "Node type to list"},
                "limit": {"type": "integer", "description": "Max results (default 20)"}
            },
            "required": ["type"]
        }
    )
]


def execute(name: str, args: Dict[str, Any]) -> str:
    """Execute a tool and return result string"""
    try:
        if name == "search":
            return _search(args)
        elif name == "get_node":
            return _get_node(args)
        elif name == "get_links":
            return _get_links(args)
        elif name == "traverse":
            return _traverse(args)
        elif name == "filter":
            return _filter(args)
        elif name == "list_nodes":
            return _list_nodes(args)
        else:
            return f"Unknown tool: {name}"
    except Exception as e:
        return f"Error: {e}"


def _search(args: Dict) -> str:
    query = args.get("query", "")
    limit = args.get("limit", 10)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/search",
        params={"q": query, "limit": limit},
        timeout=10
    )

    if resp.status_code != 200:
        return f"Search failed: {resp.status_code}"

    data = resp.json()
    nodes = data.get("nodes", [])

    if not nodes:
        return f"No results for '{query}'"

    lines = [f"Found {len(nodes)} results:"]
    for n in nodes:
        nid = n.get("ID", "")
        ntype = n.get("Type", "")
        meta = n.get("Meta", {})
        name = meta.get("name") or meta.get("title") or nid
        lines.append(f"  [{ntype}] {name} (id: {nid})")

    return "\n".join(lines)


def _get_node(args: Dict) -> str:
    node_id = args.get("id", "")

    resp = httpx.get(f"{MEMEX_URL}/api/nodes/{node_id}", timeout=10)

    if resp.status_code != 200:
        return f"Node not found: {node_id}"

    n = resp.json()
    lines = [f"Node: {node_id}", f"  Type: {n.get('Type', '')}"]

    meta = n.get("Meta", {})
    for k, v in meta.items():
        if isinstance(v, (str, int, float, bool)):
            lines.append(f"  {k}: {v}")

    return "\n".join(lines)


def _get_links(args: Dict) -> str:
    node_id = args.get("id", "")

    resp = httpx.get(f"{MEMEX_URL}/api/nodes/{node_id}/links", timeout=10)

    if resp.status_code != 200:
        return f"No links for: {node_id}"

    data = resp.json()
    # API returns array directly, not {"links": [...]}
    links = data if isinstance(data, list) else data.get("links", [])

    if not links:
        return f"No links for {node_id}"

    # Dedupe links (API sometimes returns duplicates)
    seen = set()
    unique_links = []
    for link in links:
        key = (link.get("Source"), link.get("Target"), link.get("Type"))
        if key not in seen:
            seen.add(key)
            unique_links.append(link)

    lines = [f"Links for {node_id} ({len(unique_links)}):"]
    for link in unique_links[:20]:  # Limit output
        src = link.get("Source", "")
        tgt = link.get("Target", "")
        ltype = link.get("Type", "")
        if src == node_id:
            lines.append(f"  --[{ltype}]--> {tgt}")
        else:
            lines.append(f"  <--[{ltype}]-- {src}")

    if len(unique_links) > 20:
        lines.append(f"  ... and {len(unique_links) - 20} more")

    return "\n".join(lines)


def _traverse(args: Dict) -> str:
    start = args.get("start", "")
    depth = args.get("depth", 2)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/traverse",
        params={"start": start, "depth": depth},
        timeout=10
    )

    if resp.status_code != 200:
        return f"Traverse failed from: {start}"

    data = resp.json()
    nodes = data.get("nodes", [])
    edges = data.get("edges", [])

    if not nodes:
        return f"No nodes from {start}"

    lines = [f"Traversal from {start}: {len(nodes)} nodes, {len(edges)} edges"]
    for n in nodes[:10]:
        nid = n.get("ID", "")
        ntype = n.get("Type", "")
        meta = n.get("Meta", {})
        name = meta.get("name") or meta.get("title") or nid
        lines.append(f"  [{ntype}] {name}")

    if len(nodes) > 10:
        lines.append(f"  ... and {len(nodes) - 10} more")

    return "\n".join(lines)


def _filter(args: Dict) -> str:
    ntype = args.get("type", "")
    limit = args.get("limit", 20)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/filter",
        params={"type": ntype, "limit": limit},
        timeout=10
    )

    if resp.status_code != 200:
        return f"Filter failed for type: {ntype}"

    data = resp.json()
    nodes = data.get("nodes", [])

    if not nodes:
        return f"No {ntype} nodes found"

    lines = [f"{ntype} nodes ({len(nodes)}):"]
    for n in nodes:
        nid = n.get("ID", "") if isinstance(n, dict) else n
        lines.append(f"  {nid}")

    return "\n".join(lines)


def _list_nodes(args: Dict) -> str:
    return _filter(args)  # Same implementation
