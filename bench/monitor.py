#!/usr/bin/env python3
"""
Monitor ingestion progress and graph health.

Usage:
    python monitor.py          # One-time status
    python monitor.py --watch  # Continuous monitoring
"""

import argparse
import json
import time
from datetime import datetime
from pathlib import Path

import httpx

MEMEX_URL = "http://localhost:8080"
BASE_DIR = Path(__file__).parent


def load_progress() -> dict:
    """Load and aggregate ingestion progress from all workers."""
    aggregated = {
        "sources_created": 0,
        "entities_created": 0,
        "relationships_created": 0,
        "errors": 0,
        "completed_sources": [],
        "workers": {},
    }

    # Find all progress files (main + workers)
    progress_files = list(BASE_DIR.glob(".ingest*progress*.json"))

    for pfile in progress_files:
        try:
            with open(pfile) as f:
                data = json.load(f)

            worker_name = pfile.stem
            aggregated["workers"][worker_name] = {
                "sources": data.get("sources_created", 0),
                "entities": data.get("entities_created", 0),
                "relationships": data.get("relationships_created", 0),
                "errors": data.get("errors", 0),
                "last_update": data.get("last_update", "N/A"),
            }

            aggregated["sources_created"] += data.get("sources_created", 0)
            aggregated["entities_created"] += data.get("entities_created", 0)
            aggregated["relationships_created"] += data.get("relationships_created", 0)
            aggregated["errors"] += data.get("errors", 0)
            aggregated["completed_sources"].extend(data.get("completed_sources", []))
        except Exception as e:
            print(f"Error reading {pfile}: {e}")

    return aggregated


def get_graph_stats() -> dict:
    """Get current graph statistics."""
    try:
        resp = httpx.get(f"{MEMEX_URL}/api/graph/map", timeout=10)
        if resp.status_code == 200:
            return resp.json()
    except Exception as e:
        print(f"Error fetching graph stats: {e}")
    return {}


def check_server_health() -> bool:
    """Check if Memex server is healthy."""
    try:
        resp = httpx.get(f"{MEMEX_URL}/health", timeout=5)
        return resp.status_code == 200
    except:
        return False


def print_status():
    """Print current status."""
    print(f"\n{'='*60}")
    print(f"MEMEX INGESTION STATUS - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"{'='*60}")

    # Server health
    healthy = check_server_health()
    print(f"\nServer Status: {'✓ Healthy' if healthy else '✗ Not responding'}")

    if not healthy:
        print("⚠️  Memex server is not running!")
        return

    # Progress
    progress = load_progress()
    if progress and (progress.get('sources_created', 0) > 0 or progress.get('workers')):
        print(f"\nIngestion Progress (Aggregated):")
        print(f"  Sources processed: {progress.get('sources_created', 0)}")
        print(f"  Entities created:  {progress.get('entities_created', 0)}")
        print(f"  Relationships:     {progress.get('relationships_created', 0)}")
        print(f"  Errors:            {progress.get('errors', 0)}")

        # Calculate rate
        completed = len(progress.get('completed_sources', []))
        print(f"  Unique sources:    {completed}")

        # Show per-worker status
        workers = progress.get('workers', {})
        if workers:
            print(f"\n  Per-Worker Status:")
            for wname, wdata in sorted(workers.items()):
                print(f"    {wname}: {wdata['sources']} sources, "
                      f"{wdata['entities']} entities, "
                      f"{wdata['errors']} errors, "
                      f"updated {wdata['last_update']}")
    else:
        print("\nNo progress file found. Ingestion may not have started.")

    # Graph stats
    stats = get_graph_stats()
    if stats:
        print(f"\nGraph Statistics:")
        print(f"  Total nodes: {stats.get('stats', {}).get('total_nodes', 'N/A')}")
        print(f"  Total edges: {stats.get('stats', {}).get('total_edges', 'N/A')}")

        node_types = stats.get('node_types', {})
        if node_types:
            print(f"\n  Node Types:")
            for ntype, count in sorted(node_types.items(), key=lambda x: -x[1]):
                print(f"    {ntype}: {count}")

        edge_types = stats.get('edge_types', {})
        if edge_types:
            print(f"\n  Edge Types (top 10):")
            for etype, count in sorted(edge_types.items(), key=lambda x: -x[1])[:10]:
                print(f"    {etype}: {count}")


def main():
    parser = argparse.ArgumentParser(description="Monitor Memex ingestion")
    parser.add_argument("--watch", "-w", action="store_true", help="Watch mode (refresh every 30s)")
    parser.add_argument("--interval", "-i", type=int, default=30, help="Refresh interval in seconds")
    args = parser.parse_args()

    if args.watch:
        try:
            while True:
                print("\033[2J\033[H", end="")  # Clear screen
                print_status()
                print(f"\nRefreshing in {args.interval}s... (Ctrl+C to stop)")
                time.sleep(args.interval)
        except KeyboardInterrupt:
            print("\nStopped watching.")
    else:
        print_status()


if __name__ == "__main__":
    main()
