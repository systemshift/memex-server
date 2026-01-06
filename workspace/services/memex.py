"""
Memex client service.

Handles all interactions with the Memex graph database:
- Searching for context
- Auto-filling suggestions
- Saving workspace items
"""

import os
from typing import Dict, Any, List, Optional
import requests
from dataclasses import dataclass

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")


@dataclass
class MemexNode:
    """A node from Memex"""
    id: str
    type: str
    meta: Dict[str, Any]
    score: float = 0.0


class MemexClient:
    """Client for interacting with Memex API"""

    def __init__(self, base_url: str = MEMEX_URL):
        self.base_url = base_url

    def _get(self, path: str, params: Optional[Dict] = None) -> Optional[Dict]:
        """Make GET request to Memex"""
        try:
            resp = requests.get(f"{self.base_url}{path}", params=params, timeout=5)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            print(f"Memex GET error: {e}")
            return None

    def _post(self, path: str, data: Dict) -> Optional[Dict]:
        """Make POST request to Memex"""
        try:
            resp = requests.post(f"{self.base_url}{path}", json=data, timeout=5)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            print(f"Memex POST error: {e}")
            return None

    def search(self, query: str, limit: int = 10, types: Optional[List[str]] = None) -> List[MemexNode]:
        """Search Memex for relevant nodes"""
        params = {"q": query, "limit": limit}
        if types:
            params["types"] = ",".join(types)

        result = self._get("/api/query/search", params)
        if not result:
            return []

        nodes = result.get("nodes")
        if not nodes:
            return []

        return [
            MemexNode(
                id=node.get("ID", ""),
                type=node.get("Type", ""),
                meta=node.get("Meta", {}),
                score=node.get("Score", 0.0)
            )
            for node in nodes
        ]

    def get_node(self, node_id: str) -> Optional[MemexNode]:
        """Get a specific node by ID"""
        result = self._get(f"/api/nodes/{node_id}")
        if not result:
            return None
        return MemexNode(
            id=result.get("ID", node_id),
            type=result.get("Type", ""),
            meta=result.get("Meta", {})
        )

    def create_node(self, node_type: str, meta: Dict[str, Any], node_id: Optional[str] = None) -> Optional[str]:
        """Create a new node in Memex"""
        import uuid
        if not node_id:
            node_id = f"{node_type.lower()}:{uuid.uuid4().hex[:12]}"

        result = self._post("/api/nodes", {
            "id": node_id,
            "type": node_type,
            "meta": meta
        })

        return node_id if result else None

    def create_link(self, source: str, target: str, link_type: str, meta: Optional[Dict] = None) -> bool:
        """Create a link between two nodes"""
        result = self._post("/api/links", {
            "source": source,
            "target": target,
            "type": link_type,
            "meta": meta or {}
        })
        return result is not None

    def get_context_for_input(self, user_input: str, limit: int = 5) -> List[Dict[str, Any]]:
        """
        Get relevant context from Memex for a user input.
        Returns context cards with title, content, and source info.
        """
        context_cards = []

        # Search for similar workflows
        workflows = self.search(user_input, limit=limit, types=["Workflow"])
        for node in workflows:
            meta = node.meta
            context_cards.append({
                "title": f"Similar: {meta.get('title', 'Workflow')}",
                "content": meta.get('description', str(meta.get('final_state', {}).get('fields', {}))[:100]),
                "source_id": node.id,
                "source_type": "workflow",
                "relevance": node.score
            })

        # Search for related entities (companies, people, etc.)
        entities = self.search(user_input, limit=3, types=["Company", "Person", "Vendor"])
        for node in entities:
            meta = node.meta
            context_cards.append({
                "title": f"{node.type}: {meta.get('name', node.id)}",
                "content": meta.get('description', ''),
                "source_id": node.id,
                "source_type": "entity",
                "relevance": node.score
            })

        # Search for policies or rules
        policies = self.search(f"{user_input} policy rule", limit=2, types=["Policy", "Rule"])
        for node in policies:
            meta = node.meta
            context_cards.append({
                "title": meta.get('title', 'Policy'),
                "content": meta.get('content', meta.get('description', '')),
                "source_id": node.id,
                "source_type": "policy",
                "relevance": node.score
            })

        # Sort by relevance and return top items
        context_cards.sort(key=lambda x: x.get("relevance", 0), reverse=True)
        return context_cards[:limit]

    def get_suggestions_for_field(
        self,
        field_name: str,
        field_value: Any,
        context: str = ""
    ) -> List[Dict[str, Any]]:
        """
        Get auto-complete suggestions for a field based on Memex history.
        """
        suggestions = []

        # Search for similar values in past workflows
        search_query = f"{field_name}:{field_value}" if field_value else field_name
        if context:
            search_query = f"{context} {search_query}"

        results = self.search(search_query, limit=10)

        seen_values = set()
        for node in results:
            meta = node.meta

            # Look for the field in various places
            value = None
            if field_name in meta:
                value = meta[field_name]
            elif "fields" in meta and field_name in meta["fields"]:
                value = meta["fields"][field_name].get("value")
            elif "final_state" in meta:
                final_state = meta["final_state"]
                if "fields" in final_state and field_name in final_state["fields"]:
                    value = final_state["fields"][field_name].get("value")

            if value and value not in seen_values:
                seen_values.add(str(value))
                suggestions.append({
                    "value": value,
                    "source": node.id,
                    "confidence": min(node.score, 1.0),
                    "label": f"From: {meta.get('title', node.type)}"
                })

        return suggestions[:5]

    def save_workspace_item(self, view_spec: Dict[str, Any]) -> Optional[str]:
        """Save a completed workspace item to Memex"""
        node_id = self.create_node(
            node_type="WorkspaceItem",
            meta={
                "title": view_spec.get("title"),
                "view_type": view_spec.get("view_type"),
                "source_input": view_spec.get("source_input"),
                "components": view_spec.get("components"),
                "complete": view_spec.get("complete"),
                "created": view_spec.get("created")
            }
        )

        # Extract entities and create links
        if node_id:
            for component in view_spec.get("components", []):
                if component.get("suggestions"):
                    for suggestion in component["suggestions"]:
                        if suggestion.get("source"):
                            self.create_link(
                                source=node_id,
                                target=suggestion["source"],
                                link_type="DERIVED_FROM"
                            )

        return node_id

    # ============================================
    # Lens and Anchor Methods (Phase 2)
    # ============================================

    def _patch(self, path: str, data: Dict) -> Optional[Dict]:
        """Make PATCH request to Memex"""
        try:
            resp = requests.patch(f"{self.base_url}{path}", json=data, timeout=5)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            print(f"Memex PATCH error: {e}")
            return None

    def create_lens(self, lens_data: Dict[str, Any]) -> Optional[str]:
        """Create a lens in Memex"""
        result = self._post("/api/lenses", lens_data)
        return lens_data.get("id") if result else None

    def get_lens(self, lens_id: str) -> Optional[Dict[str, Any]]:
        """Get a lens definition by ID"""
        # Ensure proper ID format
        if not lens_id.startswith("lens:"):
            lens_id = f"lens:{lens_id}"
        return self._get(f"/api/lenses/{lens_id}")

    def list_lenses(self) -> List[Dict[str, Any]]:
        """List all available lenses"""
        result = self._get("/api/lenses")
        if result and "lenses" in result:
            return result["lenses"]
        return []

    def ingest_content(self, content: str) -> Optional[str]:
        """
        Ingest content into Memex and get content-addressed ID.
        Returns the SHA256-based source ID.
        """
        import hashlib
        # Calculate SHA256 for the content
        content_hash = hashlib.sha256(content.encode()).hexdigest()
        source_id = f"sha256:{content_hash}"

        # Try to create as a Source node
        result = self._post("/api/ingest", {
            "content": content,
            "format": "text"
        })

        if result:
            return result.get("source_id", source_id)

        # Fallback: create as regular node
        self.create_node(
            node_type="Source",
            meta={"content": content, "format": "text"},
            node_id=source_id
        )
        return source_id

    def query_by_lens(
        self,
        lens_id: str,
        pattern: Optional[str] = None,
        limit: int = 50
    ) -> List[MemexNode]:
        """
        Query entities that were interpreted through a specific lens.
        Optionally filter by pattern.
        """
        if not lens_id.startswith("lens:"):
            lens_id = f"lens:{lens_id}"

        params = {"lens_id": lens_id, "limit": limit}
        if pattern:
            params["pattern"] = pattern

        result = self._get("/api/query/by_lens", params)
        if not result:
            return []

        nodes = result.get("entities", result.get("nodes", []))
        return [
            MemexNode(
                id=node.get("ID", ""),
                type=node.get("Type", ""),
                meta=node.get("Meta", {}),
                score=node.get("Score", 0.0)
            )
            for node in nodes
        ]

    def get_node_links(self, node_id: str) -> List[Dict[str, Any]]:
        """Get all links for a node"""
        result = self._get(f"/api/nodes/{node_id}/links")
        if result:
            return result.get("links", [])
        return []

    def get_subgraph(
        self,
        start_id: str,
        depth: int = 2,
        rel_types: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Get a subgraph starting from a node.
        Used for dashboard visualization.
        """
        params = {"start": start_id, "depth": depth}
        if rel_types:
            params["rel_types"] = ",".join(rel_types)

        result = self._get("/api/query/subgraph", params)
        return result or {"nodes": [], "edges": []}

    def get_deal_flow_graph(self) -> Dict[str, Any]:
        """
        Get the deal flow subgraph for dashboard.
        Returns nodes and edges related to deals/work items.
        """
        # Search for work items
        work_items = self.search("WorkItem", limit=50, types=["WorkItem"])

        # Search for deals
        deals = self.search("Deal", limit=50, types=["Deal"])

        # Search for companies
        companies = self.search("Company", limit=20, types=["Company"])

        # Build graph structure
        nodes = []
        edges = []
        seen_ids = set()

        for item in work_items + deals:
            if item.id not in seen_ids:
                nodes.append({
                    "id": item.id,
                    "type": item.type,
                    "label": item.meta.get("title", item.meta.get("name", item.id)),
                    "status": item.meta.get("status", "unknown"),
                    "stage": item.meta.get("stage", ""),
                    "assigned_to": item.meta.get("assigned_to", "")
                })
                seen_ids.add(item.id)

                # Get links for this node
                links = self.get_node_links(item.id)
                for link in links:
                    edges.append({
                        "source": link.get("source", item.id),
                        "target": link.get("target", ""),
                        "type": link.get("type", "RELATES_TO")
                    })

        for company in companies:
            if company.id not in seen_ids:
                nodes.append({
                    "id": company.id,
                    "type": "Company",
                    "label": company.meta.get("name", company.id),
                    "industry": company.meta.get("industry", "")
                })
                seen_ids.add(company.id)

        return {"nodes": nodes, "edges": edges}

    def get_similar_work(
        self,
        query: str,
        limit: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Find similar past work for context.
        Returns context cards with relevant history.
        """
        context = []

        # Search implementations
        impls = self.search(query, limit=limit, types=["Implementation"])
        for impl in impls:
            meta = impl.meta
            context.append({
                "type": "implementation",
                "title": meta.get("title", "Past Implementation"),
                "content": meta.get("tips_for_next_time", meta.get("notes", "")),
                "duration": meta.get("duration_days"),
                "issues": meta.get("issues", []),
                "lessons": meta.get("lessons_learned", []),
                "source_id": impl.id,
                "relevance": impl.score
            })

        # Search patterns
        patterns = self.search(query, limit=3, types=["Pattern"])
        for pattern in patterns:
            meta = pattern.meta
            context.append({
                "type": "pattern",
                "title": meta.get("title", "Pattern"),
                "content": meta.get("content", ""),
                "confidence": meta.get("confidence", 0.5),
                "source_id": pattern.id,
                "relevance": pattern.score
            })

        # Sort by relevance
        context.sort(key=lambda x: x.get("relevance", 0), reverse=True)
        return context[:limit]


# Global client instance
memex = MemexClient()
