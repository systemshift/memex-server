#!/usr/bin/env python3
"""
Reset the Memex Workspace demo to a clean state.

This script:
1. Clears all in-memory work items, handoffs, and notifications
2. Optionally deletes WorkItem nodes from Memex
3. Keeps seed data (Companies, Deals, Implementations, Patterns, Users, Lenses)

Usage:
    python demo/reset_demo.py           # Reset in-memory state only
    python demo/reset_demo.py --full    # Also delete WorkItem nodes from Memex
    python demo/reset_demo.py --reseed  # Reset and re-run seed scripts
"""

import argparse
import requests
import sys

WORKSPACE_URL = "http://localhost:5002"
MEMEX_URL = "http://localhost:8080"


def reset_workspace():
    """Reset the workspace in-memory state"""
    print("Resetting workspace state...")

    try:
        response = requests.post(f"{WORKSPACE_URL}/api/reset", timeout=10)
        if response.status_code == 200:
            result = response.json()
            print(f"  {result.get('message')}")
            return True
        else:
            print(f"  Failed: {response.status_code}")
            return False
    except requests.exceptions.ConnectionError:
        print(f"  Could not connect to workspace at {WORKSPACE_URL}")
        print("  Make sure the workspace is running: python run.py")
        return False
    except Exception as e:
        print(f"  Error: {e}")
        return False


def delete_work_items_from_memex():
    """Delete WorkItem nodes from Memex"""
    print("\nDeleting WorkItem nodes from Memex...")

    try:
        # Search for WorkItem nodes
        response = requests.get(
            f"{MEMEX_URL}/api/query/search",
            params={"q": "WorkItem", "types": "WorkItem", "limit": 100},
            timeout=10
        )

        if response.status_code != 200:
            print(f"  Failed to search: {response.status_code}")
            return False

        data = response.json()
        nodes = data.get("nodes", [])

        if not nodes:
            print("  No WorkItem nodes found")
            return True

        deleted = 0
        for node in nodes:
            node_id = node.get("ID")
            if node_id:
                del_response = requests.delete(
                    f"{MEMEX_URL}/api/nodes/{node_id}",
                    timeout=10
                )
                if del_response.status_code == 200:
                    deleted += 1

        print(f"  Deleted {deleted} WorkItem nodes")
        return True

    except requests.exceptions.ConnectionError:
        print(f"  Could not connect to Memex at {MEMEX_URL}")
        return False
    except Exception as e:
        print(f"  Error: {e}")
        return False


def run_seed_scripts():
    """Re-run the seed scripts"""
    print("\nRe-running seed scripts...")

    import subprocess
    import os

    demo_dir = os.path.dirname(os.path.abspath(__file__))

    # Run create_lens.py
    print("\n--- Running create_lens.py ---")
    result = subprocess.run(
        [sys.executable, os.path.join(demo_dir, "create_lens.py")],
        capture_output=False
    )

    # Run seed_data.py
    print("\n--- Running seed_data.py ---")
    result = subprocess.run(
        [sys.executable, os.path.join(demo_dir, "seed_data.py")],
        capture_output=False
    )

    return True


def main():
    parser = argparse.ArgumentParser(description="Reset Memex Workspace demo")
    parser.add_argument(
        "--full",
        action="store_true",
        help="Also delete WorkItem nodes from Memex"
    )
    parser.add_argument(
        "--reseed",
        action="store_true",
        help="Re-run seed scripts after reset"
    )

    args = parser.parse_args()

    print("=" * 50)
    print("Memex Workspace - Demo Reset")
    print("=" * 50)

    # Reset workspace
    reset_workspace()

    # Delete from Memex if --full
    if args.full:
        delete_work_items_from_memex()

    # Reseed if --reseed
    if args.reseed:
        run_seed_scripts()

    print("\n" + "=" * 50)
    print("Reset complete!")
    print("")
    print("To test the demo:")
    print("  1. Open http://localhost:5002")
    print("  2. Select 'Alex (Sales)' from the user dropdown")
    print("  3. Type: 'Closed deal with Acme Corp, $50k ARR, need SSO. Forward to Jordan.'")
    print("  4. Click 'Send Handoff' to Jordan")
    print("  5. Switch to Jordan and see the notification")
    print("=" * 50)


if __name__ == "__main__":
    main()
