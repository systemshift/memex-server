#!/usr/bin/env python3
"""
Load synthetic organization data into Memex.
"""

import json
import requests
from typing import Dict, Any

MEMEX_API = "http://localhost:8080"


def load_data(path: str = "bench/synthetic_org_data.json") -> Dict[str, Any]:
    """Load generated data."""
    with open(path) as f:
        return json.load(f)


def create_node(node_id: str, node_type: str, meta: Dict[str, Any]) -> bool:
    """Create a node in memex."""
    try:
        resp = requests.post(
            f"{MEMEX_API}/api/nodes",
            json={"id": node_id, "type": node_type, "meta": meta},
            timeout=10
        )
        return resp.status_code in (200, 201)
    except Exception as e:
        print(f"Error creating node {node_id}: {e}")
        return False


def create_link(source: str, target: str, link_type: str, meta: Dict[str, Any] = None) -> bool:
    """Create a link in memex."""
    try:
        resp = requests.post(
            f"{MEMEX_API}/api/links",
            json={
                "source": source,
                "target": target,
                "type": link_type,
                "meta": meta or {}
            },
            timeout=10
        )
        return resp.status_code in (200, 201)
    except Exception as e:
        print(f"Error creating link {source} -> {target}: {e}")
        return False


def create_attention_edge(source: str, target: str, weight: float, query_id: str) -> bool:
    """Create an attention edge."""
    try:
        resp = requests.post(
            f"{MEMEX_API}/api/edges/attention",
            json={
                "source": source,
                "target": target,
                "weight": weight,
                "query_id": query_id
            },
            timeout=10
        )
        return resp.status_code in (200, 201)
    except Exception as e:
        print(f"Error creating attention edge {source} -> {target}: {e}")
        return False


def main():
    print("Loading synthetic data into Memex...")

    data = load_data()
    meta = data["metadata"]
    print(f"  Data: {meta['num_people']} people, {meta['num_projects']} projects, "
          f"{meta['num_documents']} documents")

    # Create nodes
    print("\nCreating nodes...")

    # Teams
    for team in data["entities"]["teams"]:
        create_node(team["id"], "Team", {"name": team["name"]})
    print(f"  Created {len(data['entities']['teams'])} teams")

    # People
    for person in data["entities"]["people"]:
        create_node(person["id"], "Person", {
            "name": person["name"],
            "team": person["team"],
            "seniority": person["seniority"],
            "skills": person["skills"],
            "joined": person["joined"]
        })
    print(f"  Created {len(data['entities']['people'])} people")

    # Projects
    for project in data["entities"]["projects"]:
        create_node(project["id"], "Project", {
            "name": project["name"],
            "team": project["team"],
            "status": project["status"],
            "started": project["started"]
        })
    print(f"  Created {len(data['entities']['projects'])} projects")

    # Documents
    for doc in data["entities"]["documents"]:
        create_node(doc["id"], "Document", {
            "title": doc["title"],
            "doc_type": doc["doc_type"],
            "topics": doc["topics"],
            "created": doc["created"]
        })
    print(f"  Created {len(data['entities']['documents'])} documents")

    # Create explicit links
    print("\nCreating links...")
    link_count = 0
    for link in data["links"]:
        if create_link(link["source"], link["target"], link["type"], link.get("meta")):
            link_count += 1
    print(f"  Created {link_count} links")

    # Create attention edges
    print("\nCreating attention edges...")
    edge_count = 0
    for edge in data["attention_edges"]:
        if create_attention_edge(edge["source"], edge["target"], edge["weight"], edge["query_id"]):
            edge_count += 1
    print(f"  Created {edge_count} attention edges")

    print("\nDone! Data loaded into Memex.")
    print(f"\nHidden patterns to discover: {meta['num_hidden_patterns']}")
    print("Run the world model training next.")


if __name__ == "__main__":
    main()
