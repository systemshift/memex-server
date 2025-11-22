#!/usr/bin/env python3
"""
Scale test: Ingest 25 popular CS/tech Wikipedia pages
"""

from wikipedia_ingest import ingest_multiple_pages

# Top CS/tech pages for testing
pages = [
    # Programming languages
    "Python (programming language)",
    "JavaScript",
    "Java (programming language)",
    "C (programming language)",
    "C++",
    "Rust (programming language)",
    "Go (programming language)",
    "TypeScript",

    # AI/ML
    "Artificial intelligence",
    "Machine learning",
    "Deep learning",
    "Neural network (machine learning)",
    "Large language model",
    "GPT-4",

    # Databases & Infrastructure
    "Graph database",
    "Database",
    "PostgreSQL",
    "Neo4j",
    "Redis",

    # Companies & Projects
    "OpenAI",
    "Google",
    "Amazon (company)",
    "Meta Platforms",
    "Linux",
    "Git",
]

print(f"Ingesting {len(pages)} Wikipedia pages")
print(f"Expected: ~{len(pages) * 5} Source nodes, ~{len(pages) * 10} entities")
print(f"Estimated cost: ~${len(pages) * 0.05:.2f}")
print()

ingest_multiple_pages(pages, max_revisions=5, extract_concepts=True)
