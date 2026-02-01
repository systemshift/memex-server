"""Combined memex and dagit tools for function calling."""

import os
import json
import httpx

from dotenv import load_dotenv

load_dotenv()

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")


# --- Memex Tool Definitions ---

MEMEX_TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "memex_search",
            "description": "Full-text search across all nodes in the knowledge graph",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search terms"},
                    "limit": {
                        "type": "integer",
                        "description": "Max results (default 10)",
                    },
                },
                "required": ["query"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memex_get_node",
            "description": "Get full details of a specific node by ID",
            "parameters": {
                "type": "object",
                "properties": {
                    "id": {
                        "type": "string",
                        "description": "Node ID (e.g. person:001)",
                    },
                },
                "required": ["id"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memex_get_links",
            "description": "Get all relationships for a node",
            "parameters": {
                "type": "object",
                "properties": {
                    "id": {"type": "string", "description": "Node ID"},
                },
                "required": ["id"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memex_traverse",
            "description": "Traverse graph from a starting node",
            "parameters": {
                "type": "object",
                "properties": {
                    "start": {"type": "string", "description": "Starting node ID"},
                    "depth": {
                        "type": "integer",
                        "description": "Hops to follow (default 2)",
                    },
                },
                "required": ["start"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memex_filter",
            "description": "Filter nodes by type",
            "parameters": {
                "type": "object",
                "properties": {
                    "type": {
                        "type": "string",
                        "description": "Node type (Person, Document, etc.)",
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Max results (default 20)",
                    },
                },
                "required": ["type"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memex_create_node",
            "description": "Create a new node in the knowledge graph",
            "parameters": {
                "type": "object",
                "properties": {
                    "type": {
                        "type": "string",
                        "description": "Node type (Note, Document, Person, etc.)",
                    },
                    "content": {
                        "type": "string",
                        "description": "Main content or description",
                    },
                    "title": {
                        "type": "string",
                        "description": "Title or name for the node",
                    },
                },
                "required": ["type", "content"],
            },
        },
    },
]


def get_memex_tools() -> list[dict]:
    """Return memex tool definitions."""
    return MEMEX_TOOLS


def get_dagit_tools() -> list[dict]:
    """Return dagit tool definitions, or empty if not available."""
    try:
        from dagit.agent_tools import tools

        return tools()
    except ImportError:
        return []


def get_all_tools() -> list[dict]:
    """Return combined memex + dagit tools."""
    return get_memex_tools() + get_dagit_tools()


def execute_tool(name: str, args: dict) -> str:
    """Execute a tool and return result as string.

    Args:
        name: Tool name (memex_* or dagit_*)
        args: Tool arguments

    Returns:
        Result string for model consumption
    """
    try:
        if name.startswith("dagit_"):
            return _execute_dagit(name, args)
        elif name.startswith("memex_"):
            return _execute_memex(name, args)
        else:
            return f"Unknown tool: {name}"
    except Exception as e:
        return f"Error: {e}"


def _execute_dagit(name: str, args: dict) -> str:
    """Execute dagit tool and return string result."""
    try:
        from dagit.agent_tools import execute

        result = execute(name, args)
        if result.get("success"):
            return json.dumps(result.get("result", {}), indent=2)
        else:
            return f"Error: {result.get('error', 'Unknown error')}"
    except ImportError:
        return "Error: dagit not installed. Install with: pip install dagit"


def _execute_memex(name: str, args: dict) -> str:
    """Execute memex tool and return string result."""
    if name == "memex_search":
        return _memex_search(args)
    elif name == "memex_get_node":
        return _memex_get_node(args)
    elif name == "memex_get_links":
        return _memex_get_links(args)
    elif name == "memex_traverse":
        return _memex_traverse(args)
    elif name == "memex_filter":
        return _memex_filter(args)
    elif name == "memex_create_node":
        return _memex_create_node(args)
    else:
        return f"Unknown memex tool: {name}"


def _memex_search(args: dict) -> str:
    query = args.get("query", "")
    limit = args.get("limit", 10)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/search",
        params={"q": query, "limit": limit},
        timeout=10,
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


def _memex_get_node(args: dict) -> str:
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


def _memex_get_links(args: dict) -> str:
    node_id = args.get("id", "")

    resp = httpx.get(f"{MEMEX_URL}/api/nodes/{node_id}/links", timeout=10)

    if resp.status_code != 200:
        return f"No links for: {node_id}"

    data = resp.json()
    links = data if isinstance(data, list) else data.get("links", [])

    if not links:
        return f"No links for {node_id}"

    # Dedupe links
    seen = set()
    unique_links = []
    for link in links:
        key = (link.get("Source"), link.get("Target"), link.get("Type"))
        if key not in seen:
            seen.add(key)
            unique_links.append(link)

    lines = [f"Links for {node_id} ({len(unique_links)}):"]
    for link in unique_links[:20]:
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


def _memex_traverse(args: dict) -> str:
    start = args.get("start", "")
    depth = args.get("depth", 2)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/traverse",
        params={"start": start, "depth": depth},
        timeout=10,
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


def _memex_filter(args: dict) -> str:
    ntype = args.get("type", "")
    limit = args.get("limit", 20)

    resp = httpx.get(
        f"{MEMEX_URL}/api/query/filter",
        params={"type": ntype, "limit": limit},
        timeout=10,
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


def _memex_create_node(args: dict) -> str:
    ntype = args.get("type", "Note")
    content = args.get("content", "")
    title = args.get("title", "")

    payload = {
        "type": ntype,
        "meta": {
            "content": content,
        },
    }
    if title:
        payload["meta"]["title"] = title

    resp = httpx.post(
        f"{MEMEX_URL}/api/nodes",
        json=payload,
        timeout=10,
    )

    if resp.status_code not in (200, 201):
        return f"Create failed: {resp.status_code} - {resp.text}"

    data = resp.json()
    node_id = data.get("id") or data.get("ID", "unknown")
    return f"Created {ntype} node: {node_id}"
