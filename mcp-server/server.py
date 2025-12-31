#!/usr/bin/env python3
"""
Memex MCP Server

Exposes Memex knowledge graph as an MCP server that agents can connect to.
"""

import os
import json
import asyncio
import httpx
from typing import Any, Optional
from mcp.server import Server
from mcp.types import (
    Tool,
    TextContent,
    ImageContent,
    EmbeddedResource,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)
import mcp.server.stdio


# Memex API configuration
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")


class MemexMCP:
    """MCP Server for Memex knowledge graph"""

    def __init__(self):
        self.app = Server("memex-mcp")
        self.client = httpx.AsyncClient(base_url=MEMEX_URL)

        # Register handlers
        self.app.list_tools()(self.list_tools)
        self.app.call_tool()(self.call_tool)

    async def list_tools(self) -> list[Tool]:
        """List available Memex query tools"""
        return [
            Tool(
                name="search_nodes",
                description="Full-text search across all nodes in the knowledge graph. Searches IDs, types, properties, and content.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Search term to find in nodes",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results (default: 100)",
                            "default": 100,
                        },
                        "offset": {
                            "type": "integer",
                            "description": "Pagination offset (default: 0)",
                            "default": 0,
                        },
                    },
                    "required": ["query"],
                },
            ),
            Tool(
                name="filter_nodes",
                description="Filter nodes by type and/or property values. Use for structured queries.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "types": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Node types to filter (e.g., ['Person', 'Concept'])",
                        },
                        "property_key": {
                            "type": "string",
                            "description": "Property key to match",
                        },
                        "property_value": {
                            "type": "string",
                            "description": "Property value to match",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results (default: 100)",
                            "default": 100,
                        },
                        "offset": {
                            "type": "integer",
                            "description": "Pagination offset (default: 0)",
                            "default": 0,
                        },
                    },
                },
            ),
            Tool(
                name="traverse_graph",
                description="Traverse the graph from a starting node, following relationships. Use to explore connections.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "start_node_id": {
                            "type": "string",
                            "description": "ID of the node to start traversal from",
                        },
                        "depth": {
                            "type": "integer",
                            "description": "How many hops to traverse (default: 2)",
                            "default": 2,
                        },
                        "relationship_types": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "Filter by relationship types (e.g., ['AUTHORED', 'FIXED'])",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results (default: 100)",
                            "default": 100,
                        },
                        "offset": {
                            "type": "integer",
                            "description": "Pagination offset (default: 0)",
                            "default": 0,
                        },
                    },
                    "required": ["start_node_id"],
                },
            ),
            Tool(
                name="get_node",
                description="Get full details of a specific node by ID.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "node_id": {
                            "type": "string",
                            "description": "ID of the node to retrieve",
                        },
                    },
                    "required": ["node_id"],
                },
            ),
            Tool(
                name="get_node_links",
                description="Get all outgoing links/relationships from a specific node.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "node_id": {
                            "type": "string",
                            "description": "ID of the node to get links from",
                        },
                    },
                    "required": ["node_id"],
                },
            ),
            Tool(
                name="list_all_nodes",
                description="List all node IDs in the graph. Use sparingly for overview.",
                inputSchema={
                    "type": "object",
                    "properties": {},
                },
            ),
            # Lens tools
            Tool(
                name="list_lenses",
                description="List all available lenses. Lenses define primitives and patterns for extracting entities from content.",
                inputSchema={
                    "type": "object",
                    "properties": {},
                },
            ),
            Tool(
                name="get_lens",
                description="Get a lens definition with its primitives and patterns. Use to understand the extraction schema.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "lens_id": {
                            "type": "string",
                            "description": "ID of the lens (with or without 'lens:' prefix)",
                        },
                    },
                    "required": ["lens_id"],
                },
            ),
            Tool(
                name="query_by_lens",
                description="Get entities that were extracted/interpreted through a specific lens. Optionally filter by matched pattern.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "lens_id": {
                            "type": "string",
                            "description": "ID of the lens (with or without 'lens:' prefix)",
                        },
                        "pattern": {
                            "type": "string",
                            "description": "Optional pattern name to filter entities (e.g., 'commitment', 'deadline')",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of results (default: 100)",
                            "default": 100,
                        },
                        "offset": {
                            "type": "integer",
                            "description": "Pagination offset (default: 0)",
                            "default": 0,
                        },
                    },
                    "required": ["lens_id"],
                },
            ),
            Tool(
                name="export_lens",
                description="Export a complete lens with all entities interpreted through it. Useful for exporting a coherent subgraph.",
                inputSchema={
                    "type": "object",
                    "properties": {
                        "lens_id": {
                            "type": "string",
                            "description": "ID of the lens (with or without 'lens:' prefix)",
                        },
                        "include_sources": {
                            "type": "boolean",
                            "description": "Include EXTRACTED_FROM links to source nodes (default: true)",
                            "default": True,
                        },
                    },
                    "required": ["lens_id"],
                },
            ),
        ]

    async def call_tool(self, name: str, arguments: dict) -> list[TextContent]:
        """Execute a Memex tool"""
        try:
            if name == "search_nodes":
                return await self._search_nodes(arguments)
            elif name == "filter_nodes":
                return await self._filter_nodes(arguments)
            elif name == "traverse_graph":
                return await self._traverse_graph(arguments)
            elif name == "get_node":
                return await self._get_node(arguments)
            elif name == "get_node_links":
                return await self._get_node_links(arguments)
            elif name == "list_all_nodes":
                return await self._list_all_nodes()
            elif name == "list_lenses":
                return await self._list_lenses()
            elif name == "get_lens":
                return await self._get_lens(arguments)
            elif name == "query_by_lens":
                return await self._query_by_lens(arguments)
            elif name == "export_lens":
                return await self._export_lens(arguments)
            else:
                return [TextContent(type="text", text=f"Unknown tool: {name}")]
        except Exception as e:
            return [TextContent(type="text", text=f"Error: {str(e)}")]

    async def _search_nodes(self, args: dict) -> list[TextContent]:
        """Search nodes by query term"""
        params = {
            "q": args["query"],
            "limit": args.get("limit", 100),
            "offset": args.get("offset", 0),
        }
        response = await self.client.get("/api/query/search", params=params)
        response.raise_for_status()
        data = response.json()

        return [TextContent(
            type="text",
            text=f"Found {data['count']} nodes:\n\n" + json.dumps(data["nodes"], indent=2)
        )]

    async def _filter_nodes(self, args: dict) -> list[TextContent]:
        """Filter nodes by type and properties"""
        params = {
            "limit": args.get("limit", 100),
            "offset": args.get("offset", 0),
        }

        if "types" in args:
            for t in args["types"]:
                params["type"] = t  # Multiple type params

        if "property_key" in args:
            params["key"] = args["property_key"]
        if "property_value" in args:
            params["value"] = args["property_value"]

        response = await self.client.get("/api/query/filter", params=params)
        response.raise_for_status()
        data = response.json()

        return [TextContent(
            type="text",
            text=f"Found {data['count']} nodes:\n\n" + json.dumps(data["nodes"], indent=2)
        )]

    async def _traverse_graph(self, args: dict) -> list[TextContent]:
        """Traverse graph from starting node"""
        params = {
            "start": args["start_node_id"],
            "depth": args.get("depth", 2),
            "limit": args.get("limit", 100),
            "offset": args.get("offset", 0),
        }

        if "relationship_types" in args:
            for rt in args["relationship_types"]:
                params["rel_type"] = rt

        response = await self.client.get("/api/query/traverse", params=params)
        response.raise_for_status()
        data = response.json()

        return [TextContent(
            type="text",
            text=f"Traversed from {args['start_node_id']} (depth={data['depth']}), found {data['count']} nodes:\n\n" + json.dumps(data["nodes"], indent=2)
        )]

    async def _get_node(self, args: dict) -> list[TextContent]:
        """Get specific node by ID"""
        response = await self.client.get(f"/api/nodes/{args['node_id']}")
        response.raise_for_status()
        node = response.json()

        return [TextContent(
            type="text",
            text=f"Node details:\n\n" + json.dumps(node, indent=2)
        )]

    async def _get_node_links(self, args: dict) -> list[TextContent]:
        """Get all links from a node"""
        response = await self.client.get(f"/api/nodes/{args['node_id']}/links")
        response.raise_for_status()
        links = response.json()

        return [TextContent(
            type="text",
            text=f"Links from {args['node_id']}:\n\n" + json.dumps(links, indent=2)
        )]

    async def _list_all_nodes(self) -> list[TextContent]:
        """List all node IDs"""
        response = await self.client.get("/api/nodes")
        response.raise_for_status()
        data = response.json()

        return [TextContent(
            type="text",
            text=f"Total nodes: {data['count']}\n\nNode IDs:\n" + "\n".join(data["nodes"])
        )]

    async def _list_lenses(self) -> list[TextContent]:
        """List all available lenses"""
        response = await self.client.get("/api/lenses")
        response.raise_for_status()
        data = response.json()

        if data["count"] == 0:
            return [TextContent(
                type="text",
                text="No lenses found. Create lenses to define extraction schemas."
            )]

        lens_summaries = []
        for lens in data["lenses"]:
            summary = f"- {lens['id']}: {lens.get('name', 'Unnamed')}"
            if lens.get("version"):
                summary += f" (v{lens['version']})"
            lens_summaries.append(summary)

        return [TextContent(
            type="text",
            text=f"Found {data['count']} lenses:\n\n" + "\n".join(lens_summaries)
        )]

    async def _get_lens(self, args: dict) -> list[TextContent]:
        """Get lens definition with primitives and patterns"""
        lens_id = args["lens_id"]
        # Handle lens: prefix
        if not lens_id.startswith("lens:"):
            lens_id = f"lens:{lens_id}"

        response = await self.client.get(f"/api/lenses/{lens_id.replace('lens:', '')}")
        response.raise_for_status()
        lens = response.json()

        # Format lens for readability
        meta = lens.get("Meta", {})
        output = f"""Lens: {lens['ID']}
Name: {meta.get('name', 'Unnamed')}
Version: {meta.get('version', '?')}
Author: {meta.get('author', 'Unknown')}

Primitives (extraction vocabulary):
{json.dumps(meta.get('primitives', {}), indent=2)}

Patterns (structural templates):
{json.dumps(meta.get('patterns', {}), indent=2)}

Extraction Hints:
{meta.get('extraction_hints', 'None')}
"""
        return [TextContent(type="text", text=output)]

    async def _query_by_lens(self, args: dict) -> list[TextContent]:
        """Get entities interpreted through a lens"""
        params = {
            "lens_id": args["lens_id"],
            "limit": args.get("limit", 100),
            "offset": args.get("offset", 0),
        }
        if "pattern" in args and args["pattern"]:
            params["pattern"] = args["pattern"]

        response = await self.client.get("/api/query/by_lens", params=params)
        response.raise_for_status()
        data = response.json()

        if data["count"] == 0:
            return [TextContent(
                type="text",
                text=f"No entities found for lens {data['lens_id']}" +
                     (f" with pattern '{data['pattern']}'" if data.get("pattern") else "")
            )]

        return [TextContent(
            type="text",
            text=f"Found {data['count']} entities interpreted through {data['lens_id']}:\n\n" +
                 json.dumps(data["entities"], indent=2)
        )]

    async def _export_lens(self, args: dict) -> list[TextContent]:
        """Export a complete lens with entities"""
        params = {"lens_id": args["lens_id"]}
        if "include_sources" in args:
            params["include_sources"] = str(args["include_sources"]).lower()

        response = await self.client.get("/api/graph/export", params=params)
        response.raise_for_status()
        data = response.json()

        output = f"""Lens Export: {data['lens']['ID']}

Lens Definition:
{json.dumps(data['lens'], indent=2)}

Entities ({data['stats']['entity_count']} total):
{json.dumps(data['entities'], indent=2) if data['entities'] else 'None'}

Links ({data['stats']['link_count']} total):
{json.dumps(data['links'], indent=2) if data['links'] else 'None'}
"""
        return [TextContent(type="text", text=output)]

    async def run(self):
        """Run the MCP server"""
        async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
            await self.app.run(
                read_stream,
                write_stream,
                self.app.create_initialization_options()
            )


async def main():
    """Main entry point"""
    server = MemexMCP()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main())
