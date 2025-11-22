#!/usr/bin/env python3
"""
Wikipedia Ingestion for Memex

Fetches Wikipedia pages with revision history and ingests into Memex.
Preserves version control, hyperlinks, and temporal data.
"""

import os
import re
import json
import time
import hashlib
from typing import List, Dict, Any, Optional
from datetime import datetime
import requests
from openai import OpenAI

# Configuration
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")
WIKIPEDIA_API = "https://en.wikipedia.org/w/api.php"
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")

client = OpenAI(api_key=OPENAI_API_KEY)


def fetch_page_revisions(page_title: str, limit: int = 10) -> List[Dict[str, Any]]:
    """
    Fetch revision history for a Wikipedia page.

    Args:
        page_title: Title of the Wikipedia page
        limit: Number of recent revisions to fetch

    Returns:
        List of revision dicts with content, timestamp, editor, etc.
    """
    params = {
        "action": "query",
        "format": "json",
        "titles": page_title,
        "prop": "revisions",
        "rvprop": "ids|timestamp|user|comment|content",
        "rvlimit": limit,
        "rvslots": "main",
    }

    headers = {
        "User-Agent": "Memex/1.0 (https://github.com/systemshift/memex; educational/research project)"
    }

    response = requests.get(WIKIPEDIA_API, params=params, headers=headers)

    # Debug: Print response
    if response.status_code != 200:
        print(f"Wikipedia API error: {response.status_code}")
        print(f"Response: {response.text[:500]}")
        raise ValueError(f"Wikipedia API returned {response.status_code}")

    try:
        data = response.json()
    except json.JSONDecodeError as e:
        print(f"Failed to parse Wikipedia response as JSON")
        print(f"Response text: {response.text[:500]}")
        raise

    # Extract page data
    pages = data.get("query", {}).get("pages", {})
    page = list(pages.values())[0]

    if "missing" in page:
        raise ValueError(f"Page not found: {page_title}")

    page_id = page["pageid"]
    revisions = page.get("revisions", [])

    # Parse revisions
    results = []
    for rev in revisions:
        content = rev.get("slots", {}).get("main", {}).get("*", "")
        results.append({
            "page_id": page_id,
            "page_title": page_title,
            "revision_id": rev["revid"],
            "timestamp": rev["timestamp"],
            "editor": rev.get("user", "anonymous"),
            "comment": rev.get("comment", ""),
            "content": content,
        })

    return results


def extract_wikilinks(wikitext: str) -> List[str]:
    """
    Extract internal Wikipedia links from wikitext.

    Matches: [[Page Name]], [[Page Name|Display Text]]

    Returns:
        List of linked page titles
    """
    # Match [[...]] but not [[File:...]] or [[Category:...]]
    pattern = r'\[\[(?!File:|Image:|Category:)([^|\]]+)(?:\|[^\]]+)?\]\]'
    matches = re.findall(pattern, wikitext)

    # Clean up titles (remove fragments, normalize)
    links = []
    for match in matches:
        title = match.split("#")[0].strip()  # Remove fragments
        if title:
            links.append(title)

    return list(set(links))  # Deduplicate


def extract_categories(wikitext: str) -> List[str]:
    """Extract categories from wikitext."""
    pattern = r'\[\[Category:([^\]]+)\]\]'
    return re.findall(pattern, wikitext)


def ingest_source(content: str, metadata: Dict[str, Any]) -> str:
    """
    Ingest content into Memex as Source node.

    Returns:
        Source ID (sha256:...)
    """
    payload = {
        "content": content,
        "format": "wikipedia",
    }

    response = requests.post(f"{MEMEX_URL}/api/ingest", json=payload)
    response.raise_for_status()
    result = response.json()

    source_id = result["source_id"]

    # Update Source node with Wikipedia metadata
    # Note: We should add PATCH /api/nodes/{id} endpoint to update Meta
    # For now, metadata is stored in ingest format field

    return source_id


def create_wikipage_node(page_title: str, page_id: int, latest_revision: int,
                         categories: List[str], wikilinks: List[str]) -> str:
    """Create WikiPage node in ontology layer."""
    node_id = f"wiki:{page_title.replace(' ', '_')}"

    payload = {
        "id": node_id,
        "type": "WikiPage",
        "meta": {
            "title": page_title,
            "page_id": page_id,
            "latest_revision": latest_revision,
            "categories": categories,
            "wikilinks": wikilinks,
            "ingested_at": datetime.now().isoformat(),
        }
    }

    response = requests.post(f"{MEMEX_URL}/api/nodes", json=payload)
    response.raise_for_status()
    return node_id


def create_link(source: str, target: str, link_type: str, meta: Optional[Dict] = None):
    """Create a link between two nodes."""
    payload = {
        "source": source,
        "target": target,
        "type": link_type,
        "meta": meta or {}
    }

    response = requests.post(f"{MEMEX_URL}/api/links", json=payload)
    response.raise_for_status()


def extract_entities_from_page(page_title: str, content: str) -> Dict[str, Any]:
    """
    Use LLM to extract entities and relationships from Wikipedia page.

    Returns:
        Dict with entities and relationships
    """
    # Truncate very long content
    max_chars = 10000
    if len(content) > max_chars:
        content = content[:max_chars] + "\n... (truncated)"

    prompt = f"""Extract structured knowledge from this Wikipedia article.

Article: {page_title}

Content:
{content}

Extract:
1. Key entities (people, places, concepts, technologies)
2. Relationships between entities
3. Main topics and themes

Return JSON:
{{
  "entities": [
    {{"id": "entity-slug", "type": "Person|Concept|Place|Technology", "label": "Display Name"}},
    ...
  ],
  "relationships": [
    {{"source": "entity-slug", "target": "entity-slug", "type": "RELATIONSHIP_TYPE"}},
    ...
  ]
}}

Focus on factual, important information. Limit to top 10 entities and relationships.
"""

    response = client.chat.completions.create(
        model="gpt-4o-mini",
        messages=[
            {"role": "system", "content": "You are a knowledge extraction system. Extract structured data from text and return valid JSON."},
            {"role": "user", "content": prompt}
        ],
        temperature=0,
    )

    result = response.choices[0].message.content

    # Parse JSON
    try:
        # Extract JSON from markdown code block if present
        if "```json" in result:
            result = result.split("```json")[1].split("```")[0]
        elif "```" in result:
            result = result.split("```")[1].split("```")[0]

        return json.loads(result.strip())
    except json.JSONDecodeError as e:
        print(f"Failed to parse LLM response: {e}")
        print(f"Response: {result}")
        return {"entities": [], "relationships": []}


def ingest_wikipedia_page(page_title: str, max_revisions: int = 10, extract_concepts: bool = True):
    """
    Ingest a Wikipedia page with revision history into Memex.

    Args:
        page_title: Title of Wikipedia page
        max_revisions: How many recent revisions to ingest
        extract_concepts: Whether to run LLM extraction (costs tokens)
    """
    print(f"\n{'='*60}")
    print(f"Ingesting: {page_title}")
    print(f"{'='*60}")

    # Fetch revisions
    print(f"Fetching {max_revisions} revisions...")
    try:
        revisions = fetch_page_revisions(page_title, limit=max_revisions)
        print(f"Found {len(revisions)} revisions")
    except Exception as e:
        print(f"Failed to fetch revisions: {e}")
        raise

    if not revisions:
        print("No revisions found!")
        return

    latest = revisions[0]

    # Extract wikilinks and categories from latest revision
    wikilinks = extract_wikilinks(latest["content"])
    categories = extract_categories(latest["content"])

    print(f"Extracted {len(wikilinks)} wikilinks, {len(categories)} categories")

    # Create WikiPage node
    wikipage_id = create_wikipage_node(
        page_title=page_title,
        page_id=latest["page_id"],
        latest_revision=latest["revision_id"],
        categories=categories,
        wikilinks=wikilinks
    )
    print(f"Created WikiPage node: {wikipage_id}")

    # Ingest each revision as Source node
    source_ids = []
    for i, rev in enumerate(revisions):
        print(f"Ingesting revision {i+1}/{len(revisions)}: {rev['revision_id']}")

        # Ingest content
        source_id = ingest_source(rev["content"], {
            "page_title": rev["page_title"],
            "revision_id": rev["revision_id"],
            "timestamp": rev["timestamp"],
            "editor": rev["editor"],
        })
        source_ids.append(source_id)

        # Link Source to WikiPage
        create_link(source_id, wikipage_id, "version_of", {
            "revision_id": rev["revision_id"],
            "timestamp": rev["timestamp"],
        })

        time.sleep(0.1)  # Rate limit

    # Link revisions to each other (temporal chain)
    for i in range(len(source_ids) - 1):
        create_link(source_ids[i], source_ids[i+1], "previous_version")
        create_link(source_ids[i+1], source_ids[i], "next_version")

    # Create links to other Wikipedia pages
    print(f"Creating {len(wikilinks)} cross-page links...")
    for link_title in wikilinks[:20]:  # Limit to first 20 to avoid spam
        target_id = f"wiki:{link_title.replace(' ', '_')}"
        create_link(wikipage_id, target_id, "links_to", {
            "link_text": link_title
        })

    # Extract concepts using LLM
    if extract_concepts:
        print("Extracting entities with LLM...")
        extracted = extract_entities_from_page(page_title, latest["content"])

        # Create entity nodes
        for entity in extracted.get("entities", []):
            node_id = entity["id"]
            node_type = entity["type"]
            label = entity["label"]

            try:
                payload = {
                    "id": node_id,
                    "type": node_type,
                    "meta": {
                        "label": label,
                        "extracted_from": wikipage_id,
                    }
                }
                requests.post(f"{MEMEX_URL}/api/nodes", json=payload)

                # Link to WikiPage
                create_link(wikipage_id, node_id, "mentions")
            except Exception as e:
                print(f"Failed to create entity {node_id}: {e}")

        # Create relationships
        for rel in extracted.get("relationships", []):
            try:
                create_link(rel["source"], rel["target"], rel["type"])
            except Exception as e:
                print(f"Failed to create relationship: {e}")

        print(f"Created {len(extracted.get('entities', []))} entities, {len(extracted.get('relationships', []))} relationships")

    print(f"\n✅ Successfully ingested {page_title}")
    print(f"   - {len(revisions)} revisions")
    print(f"   - {len(wikilinks)} wikilinks")
    print(f"   - {len(categories)} categories")


def ingest_multiple_pages(page_titles: List[str], max_revisions: int = 5, extract_concepts: bool = True):
    """Ingest multiple Wikipedia pages."""
    for i, title in enumerate(page_titles):
        print(f"\n[{i+1}/{len(page_titles)}] Processing: {title}")
        try:
            ingest_wikipedia_page(title, max_revisions, extract_concepts)
        except Exception as e:
            print(f"❌ Failed to ingest {title}: {e}")

        # Rate limit between pages
        time.sleep(1)


if __name__ == "__main__":
    # Test with a few interesting pages
    test_pages = [
        "Python (programming language)",
        "Artificial intelligence",
        "Graph database",
    ]

    print("Wikipedia → Memex Ingestion")
    print("=" * 60)
    print(f"Target: {MEMEX_URL}")
    print(f"Pages to ingest: {len(test_pages)}")
    print("=" * 60)

    ingest_multiple_pages(test_pages, max_revisions=5, extract_concepts=True)

    print("\n" + "=" * 60)
    print("✅ Ingestion complete!")
    print("=" * 60)
    print("\nTry these queries:")
    print("  1. Search: curl 'http://localhost:8080/api/query/search?q=Python'")
    print("  2. Filter WikiPages: curl 'http://localhost:8080/api/query/filter?type=WikiPage'")
    print("  3. Traverse from Python: curl 'http://localhost:8080/api/query/traverse?start=wiki:Python_(programming_language)&depth=2'")
