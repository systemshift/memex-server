#!/usr/bin/env python3
"""
Reconcile progress files with actual Neo4j data.

After a server crash, progress files may contain sources that were "processed"
(LLM called) but never actually stored in Neo4j. This script:
1. Queries Neo4j for all Source node IDs that actually exist
2. Updates progress files to only keep sources that are in the database
3. This allows resuming ingestion without re-processing stored data,
   while re-processing any lost data.

Usage:
    python reconcile_progress.py [--dry-run]
"""

import argparse
import json
from pathlib import Path
from neo4j import GraphDatabase

NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "password"

BENCH_DIR = Path(__file__).parent


def get_existing_sources(driver) -> set:
    """Query Neo4j for all Source node IDs."""
    with driver.session() as session:
        # Schema uses Node label with type property
        result = session.run("MATCH (s:Node {type: 'Source'}) RETURN s.id AS id")
        return {record["id"] for record in result}


def get_complete_sources(driver) -> set:
    """
    Query Neo4j for Source nodes that have at least one EXTRACTED_FROM relationship.
    This ensures the source was fully processed (entities were linked to it).
    """
    with driver.session() as session:
        # Schema uses Node label and LINK relationship type
        result = session.run("""
            MATCH (s:Node {type: 'Source'})<-[:LINK]-()
            RETURN DISTINCT s.id AS id
        """)
        return {record["id"] for record in result}


def get_incomplete_sources(driver) -> set:
    """Find Source nodes with no incoming LINK relationships (potentially incomplete)."""
    with driver.session() as session:
        result = session.run("""
            MATCH (s:Node {type: 'Source'})
            WHERE NOT (s)<-[:LINK]-()
            RETURN s.id AS id
        """)
        return {record["id"] for record in result}


def reconcile_progress_file(filepath: Path, existing_sources: set, dry_run: bool) -> dict:
    """Reconcile a single progress file."""
    with open(filepath) as f:
        progress = json.load(f)

    original_count = len(progress.get("completed_sources", []))

    # Filter to only sources that exist in Neo4j
    valid_sources = [s for s in progress.get("completed_sources", []) if s in existing_sources]
    removed_count = original_count - len(valid_sources)

    stats = {
        "file": filepath.name,
        "original": original_count,
        "valid": len(valid_sources),
        "removed": removed_count,
    }

    if not dry_run and removed_count > 0:
        progress["completed_sources"] = valid_sources
        # Keep entity/relationship counts as-is (they reflect what was attempted)
        # The resume will update them correctly
        with open(filepath, "w") as f:
            json.dump(progress, f, indent=2)

    return stats


def main():
    parser = argparse.ArgumentParser(description="Reconcile progress files with Neo4j")
    parser.add_argument("--dry-run", action="store_true", help="Show what would be done without making changes")
    parser.add_argument("--strict", action="store_true",
                        help="Only keep sources with EXTRACTED_FROM relationships (guarantees full integrity)")
    args = parser.parse_args()

    print(f"Connecting to Neo4j at {NEO4J_URI}...")
    driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))

    try:
        # Test connection
        driver.verify_connectivity()
        print("Connected to Neo4j successfully")

        # Get existing sources from Neo4j
        print("Querying existing Source nodes...")
        all_sources = get_existing_sources(driver)
        print(f"Found {len(all_sources)} Source nodes in Neo4j")

        # Check for incomplete sources
        incomplete_sources = get_incomplete_sources(driver)
        if incomplete_sources:
            print(f"  ⚠️  {len(incomplete_sources)} sources have no EXTRACTED_FROM links (potentially incomplete)")

        if args.strict:
            print("\n--strict mode: Only keeping sources with complete entity extraction")
            existing_sources = get_complete_sources(driver)
            print(f"Found {len(existing_sources)} complete Source nodes")
        else:
            existing_sources = all_sources
            if incomplete_sources:
                print("  (use --strict to re-process incomplete sources)")

        # Find all progress files
        progress_files = list(BENCH_DIR.glob(".ingest_progress_worker_*.json"))
        progress_files.append(BENCH_DIR / ".ingest_ai_progress.json")
        progress_files = [f for f in progress_files if f.exists()]

        print(f"\nReconciling {len(progress_files)} progress files...")
        if args.dry_run:
            print("(DRY RUN - no changes will be made)\n")

        total_removed = 0
        for filepath in sorted(progress_files):
            stats = reconcile_progress_file(filepath, existing_sources, args.dry_run)
            total_removed += stats["removed"]
            status = "would remove" if args.dry_run else "removed"
            print(f"  {stats['file']}: {stats['original']} -> {stats['valid']} ({status} {stats['removed']})")

        print(f"\nTotal sources to re-process: {total_removed}")

        if args.dry_run and total_removed > 0:
            print("\nRun without --dry-run to apply changes.")
        elif total_removed > 0:
            print("\nProgress files updated. You can now resume ingestion.")
        else:
            print("\nAll progress files are in sync with Neo4j. No changes needed.")

    finally:
        driver.close()


if __name__ == "__main__":
    main()
