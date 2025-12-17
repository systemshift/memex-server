#!/usr/bin/env python3
"""
Workflow Query CLI - Search tacit knowledge captured by screen_monitor.py

Usage:
    python workflow_query.py "how to debug"
    python workflow_query.py --user john "deployment workflow"
    python workflow_query.py --app "VS Code" --list-insights
"""

import argparse
import json
import os
import requests
from typing import Optional

MEMEX_API = os.getenv("MEMEX_API", "http://localhost:8080")


def search_knowledge(query: str, limit: int = 20) -> list:
    """Search for knowledge insights matching the query."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/query/search",
            params={"q": query, "limit": limit},
            timeout=10
        )
        if resp.status_code == 200:
            return resp.json().get("nodes", [])
        return []
    except Exception as e:
        print(f"Search error: {e}")
        return []


def filter_by_type(node_type: str, limit: int = 50) -> list:
    """Get all nodes of a specific type."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/query/filter",
            params={"type": node_type, "limit": limit},
            timeout=10
        )
        if resp.status_code == 200:
            return resp.json().get("nodes", [])
        return []
    except Exception as e:
        print(f"Filter error: {e}")
        return []


def get_node_links(node_id: str) -> list:
    """Get all links from a node."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/nodes/{node_id}/links",
            timeout=10
        )
        if resp.status_code == 200:
            return resp.json().get("links", [])
        return []
    except Exception as e:
        print(f"Links error: {e}")
        return []


def parse_properties(node: dict) -> dict:
    """Parse the properties JSON from a node."""
    props = node.get("properties", "{}")
    if isinstance(props, str):
        try:
            return json.loads(props)
        except:
            return {}
    return props if isinstance(props, dict) else {}


def print_knowledge(nodes: list):
    """Pretty print knowledge nodes."""
    knowledge_nodes = [n for n in nodes if n.get("type") == "Knowledge"]

    if not knowledge_nodes:
        print("No knowledge insights found.\n")
        return

    print(f"\nFound {len(knowledge_nodes)} knowledge insights:\n")
    print("-" * 70)

    for node in knowledge_nodes:
        props = parse_properties(node)
        insight = props.get("insight", node.get("id", "Unknown"))
        confidence = props.get("confidence", 0)
        source_app = props.get("source_app", "Unknown")
        discovered_by = props.get("discovered_by", "Unknown")

        print(f"  {insight}")
        print(f"    App: {source_app} | By: {discovered_by} | Confidence: {confidence:.0%}")
        print()


def print_tasks(nodes: list):
    """Pretty print task nodes."""
    task_nodes = [n for n in nodes if n.get("type") == "Task"]

    if not task_nodes:
        print("No tasks found.\n")
        return

    print(f"\nFound {len(task_nodes)} tasks:\n")
    print("-" * 70)

    for node in task_nodes:
        props = parse_properties(node)
        desc = props.get("description", node.get("id", "Unknown"))
        step = props.get("workflow_step", "")

        print(f"  [{step}] {desc}")
        print()


def print_workflows(nodes: list):
    """Pretty print workflow sessions."""
    workflow_nodes = [n for n in nodes if n.get("type") == "Workflow"]

    if not workflow_nodes:
        print("No workflow sessions found.\n")
        return

    print(f"\nFound {len(workflow_nodes)} workflow sessions:\n")
    print("-" * 70)

    for node in workflow_nodes:
        props = parse_properties(node)
        user = props.get("user", "Unknown")
        session = props.get("session_id", "Unknown")
        started = props.get("started_at", "Unknown")

        print(f"  Session: {session}")
        print(f"    User: {user} | Started: {started}")
        print()


def list_apps():
    """List all captured applications."""
    nodes = filter_by_type("Application", limit=100)

    if not nodes:
        print("No applications captured yet.\n")
        return

    print(f"\nCaptured applications ({len(nodes)}):\n")
    for node in nodes:
        props = parse_properties(node)
        name = props.get("name", node.get("id", "Unknown"))
        print(f"  - {name}")
    print()


def list_users():
    """List all users who have been monitored."""
    nodes = filter_by_type("User", limit=100)

    if not nodes:
        print("No users found.\n")
        return

    print(f"\nUsers ({len(nodes)}):\n")
    for node in nodes:
        props = parse_properties(node)
        name = props.get("name", node.get("id", "Unknown"))
        print(f"  - {name}")
    print()


def main():
    parser = argparse.ArgumentParser(
        description="Query tacit knowledge captured from screen workflows"
    )
    parser.add_argument(
        "query",
        nargs="?",
        help="Search query (e.g., 'how to debug', 'deployment')"
    )
    parser.add_argument(
        "--list-insights", "-l",
        action="store_true",
        help="List all captured knowledge insights"
    )
    parser.add_argument(
        "--list-tasks", "-t",
        action="store_true",
        help="List all captured tasks"
    )
    parser.add_argument(
        "--list-workflows", "-w",
        action="store_true",
        help="List all workflow sessions"
    )
    parser.add_argument(
        "--list-apps", "-a",
        action="store_true",
        help="List all captured applications"
    )
    parser.add_argument(
        "--list-users", "-u",
        action="store_true",
        help="List all monitored users"
    )
    parser.add_argument(
        "--limit", "-n",
        type=int,
        default=20,
        help="Maximum results to return (default: 20)"
    )

    args = parser.parse_args()

    print("=" * 70)
    print("Workflow Knowledge Query")
    print("=" * 70)

    if args.list_apps:
        list_apps()
    elif args.list_users:
        list_users()
    elif args.list_insights:
        nodes = filter_by_type("Knowledge", limit=args.limit)
        print_knowledge(nodes)
    elif args.list_tasks:
        nodes = filter_by_type("Task", limit=args.limit)
        print_tasks(nodes)
    elif args.list_workflows:
        nodes = filter_by_type("Workflow", limit=args.limit)
        print_workflows(nodes)
    elif args.query:
        print(f"Searching for: '{args.query}'")
        nodes = search_knowledge(args.query, limit=args.limit)

        if not nodes:
            print("\nNo results found. Try:")
            print("  - Different keywords")
            print("  - --list-insights to see all captured knowledge")
            print("  - --list-tasks to see all captured tasks")
        else:
            # Separate by type
            knowledge = [n for n in nodes if n.get("type") == "Knowledge"]
            tasks = [n for n in nodes if n.get("type") == "Task"]
            other = [n for n in nodes if n.get("type") not in ("Knowledge", "Task")]

            if knowledge:
                print_knowledge(knowledge)
            if tasks:
                print_tasks(tasks)
            if other:
                print(f"\nOther results: {len(other)} nodes")
    else:
        parser.print_help()
        print("\nExamples:")
        print("  python workflow_query.py 'debugging'")
        print("  python workflow_query.py --list-insights")
        print("  python workflow_query.py --list-tasks")


if __name__ == "__main__":
    main()
