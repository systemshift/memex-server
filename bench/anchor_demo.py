#!/usr/bin/env python3
"""
Anchor Demo - Neo Language Skills

Demonstrates "computable vernacular" - natural language with structured anchors
that machines can process. Write naturally, get structure for free.

Usage:
    python anchor_demo.py
    # Open http://localhost:5002
"""

import json
import os
import time
import hashlib
import requests
from flask import Flask, request, jsonify, render_template
from flask_cors import CORS
from openai import OpenAI
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# Configuration
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")
MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

llm_client = OpenAI()

# Default lenses to create on startup
DEFAULT_LENSES = {
    "org-planning": {
        "name": "Org Planning",
        "description": "Extract commitments, deadlines, blockers from planning docs",
        "primitives": {
            "deadline": "temporal constraint with due date",
            "owner": "entity responsible for completion",
            "gate": "conditional blocker that must be resolved",
            "risk": "uncertain negative outcome",
            "dependency": "prerequisite relationship"
        },
        "patterns": {
            "commitment": {"requires": ["deadline", "owner"], "description": "time-bound responsibility"},
            "blocked": {"requires": ["gate"], "description": "work stopped by blocker"},
            "at-risk": {"requires": ["risk"], "description": "potential failure point"}
        },
        "extraction_hints": "Look for dates, names with ownership language, blocking conditions, risk indicators"
    },
    "research": {
        "name": "Research Notes",
        "description": "Extract claims, evidence, questions from research",
        "primitives": {
            "claim": "assertion that could be true or false",
            "evidence": "data or observation supporting a claim",
            "question": "open inquiry needing investigation",
            "assumption": "unverified belief taken as given",
            "finding": "verified result from investigation"
        },
        "patterns": {
            "supported-claim": {"requires": ["claim", "evidence"], "description": "claim with backing"},
            "open-question": {"requires": ["question"], "description": "unresolved inquiry"}
        },
        "extraction_hints": "Look for statements of fact, citations, question marks, hedging language"
    },
    "conversation": {
        "name": "Conversation",
        "description": "Extract agreements, action items, decisions from discussions",
        "primitives": {
            "agreement": "mutual commitment between parties",
            "action-item": "task assigned to someone",
            "decision": "choice made among alternatives",
            "question": "information request",
            "concern": "raised issue needing attention"
        },
        "patterns": {
            "actionable": {"requires": ["action-item"], "description": "work to be done"},
            "resolved": {"requires": ["question", "decision"], "description": "answered question"}
        },
        "extraction_hints": "Look for will/going to language, question-answer pairs, agreement markers"
    }
}


# ============== Memex API Helpers ==============

def memex_get(path):
    """GET request to memex API"""
    resp = requests.get(f"{MEMEX_URL}{path}")
    resp.raise_for_status()
    return resp.json()


def memex_post(path, data):
    """POST request to memex API"""
    resp = requests.post(f"{MEMEX_URL}{path}", json=data)
    resp.raise_for_status()
    return resp.json()


def ensure_default_lenses():
    """Create default lenses if they don't exist"""
    try:
        existing = memex_get("/api/lenses")
        existing_ids = {l["id"] for l in existing.get("lenses", [])}

        for lens_id, lens_def in DEFAULT_LENSES.items():
            full_id = f"lens:{lens_id}"
            if full_id not in existing_ids:
                memex_post("/api/lenses", {
                    "id": lens_id,
                    "name": lens_def["name"],
                    "description": lens_def["description"],
                    "primitives": lens_def["primitives"],
                    "patterns": lens_def["patterns"],
                    "extraction_hints": lens_def["extraction_hints"]
                })
                print(f"Created default lens: {full_id}")
    except Exception as e:
        print(f"Warning: Could not ensure default lenses: {e}")


# ============== LLM Extraction ==============

def extract_with_lens(content: str, lens_id: str) -> dict:
    """Extract anchors from content using specified lens"""

    # Fetch lens definition
    if not lens_id.startswith("lens:"):
        lens_id = f"lens:{lens_id}"

    lens = memex_get(f"/api/lenses/{lens_id.replace('lens:', '')}")
    meta = lens.get("Meta", {})

    primitives = meta.get("primitives", {})
    patterns = meta.get("patterns", {})
    hints = meta.get("extraction_hints", "")

    # Build extraction prompt
    prompt = f"""You are extracting structured anchors from text using a specific lens.

LENS: {meta.get('name', lens_id)}
DESCRIPTION: {meta.get('description', '')}

PRIMITIVES (types of things to find):
{json.dumps(primitives, indent=2)}

PATTERNS (meaningful combinations):
{json.dumps(patterns, indent=2)}

EXTRACTION HINTS: {hints}

TEXT TO ANALYZE:
{content}

Extract anchors as JSON. For each anchor found:
- id: unique slug (lowercase, hyphens)
- type: one of the primitive types
- text: exact text span from input
- start: character offset where span starts
- end: character offset where span ends
- properties: relevant extracted properties (e.g., date, person name)
- matched_patterns: list of pattern names this anchor participates in

Return format:
{{
  "anchors": [
    {{
      "id": "example-anchor",
      "type": "deadline",
      "text": "by Jan 14",
      "start": 10,
      "end": 19,
      "properties": {{"date": "Jan 14"}},
      "matched_patterns": ["commitment"]
    }}
  ]
}}

Rules:
- Only extract what's explicitly stated in the text
- Use exact text spans from input
- Character offsets must be accurate for highlighting
- Match patterns when multiple related primitives co-occur
- If nothing matches, return empty anchors array"""

    response = llm_client.chat.completions.create(
        model=MODEL,
        messages=[{"role": "user", "content": prompt}],
        response_format={"type": "json_object"}
    )

    result = json.loads(response.choices[0].message.content)
    return result.get("anchors", [])


def store_extraction(content: str, lens_id: str, anchors: list) -> str:
    """Store content and extracted anchors in memex"""

    # Create source node via ingest
    source_resp = memex_post("/api/ingest", {
        "content": content,
        "format": "text"
    })
    source_id = source_resp["source_id"]

    if not lens_id.startswith("lens:"):
        lens_id = f"lens:{lens_id}"

    # Create entity nodes and links for each anchor
    for anchor in anchors:
        entity_id = f"anchor:{anchor['id']}"

        # Create entity node
        try:
            memex_post("/api/nodes", {
                "id": entity_id,
                "type": anchor["type"].title(),
                "meta": {
                    "name": anchor["id"].replace("-", " ").title(),
                    "text": anchor["text"],
                    "start": anchor.get("start"),
                    "end": anchor.get("end"),
                    "properties": anchor.get("properties", {}),
                    "matched_patterns": anchor.get("matched_patterns", [])
                }
            })
        except:
            pass  # Entity might already exist

        # Link entity to source (EXTRACTED_FROM)
        try:
            memex_post("/api/links", {
                "source": entity_id,
                "target": source_id,
                "type": "EXTRACTED_FROM",
                "meta": {"extraction_model": MODEL}
            })
        except:
            pass

        # Link entity to lens (INTERPRETED_THROUGH)
        try:
            memex_post("/api/links", {
                "source": entity_id,
                "target": lens_id,
                "type": "INTERPRETED_THROUGH",
                "meta": {
                    "extraction_model": MODEL,
                    "matched_patterns": anchor.get("matched_patterns", [])
                }
            })
        except:
            pass

    return source_id


def build_annotated_html(content: str, anchors: list) -> str:
    """Build HTML with highlighted anchor spans"""
    if not anchors:
        return f"<p>{content}</p>"

    # Sort anchors by start position (reverse for easier insertion)
    sorted_anchors = sorted(anchors, key=lambda a: a.get("start", 0), reverse=True)

    result = content
    for anchor in sorted_anchors:
        start = anchor.get("start", 0)
        end = anchor.get("end", start + len(anchor.get("text", "")))

        if start >= 0 and end <= len(content):
            span_class = f"anchor anchor-{anchor['type'].lower()}"
            patterns = ",".join(anchor.get("matched_patterns", []))

            span = f'<span class="{span_class}" data-id="{anchor["id"]}" data-type="{anchor["type"]}" data-patterns="{patterns}">'
            result = result[:start] + span + result[start:end] + "</span>" + result[end:]

    return f"<p>{result}</p>"


# ============== Routes ==============

@app.route('/')
def index():
    return render_template('anchor.html')


@app.route('/api/lenses')
def list_lenses():
    """List available lenses"""
    try:
        data = memex_get("/api/lenses")
        return jsonify(data)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/lens/<lens_id>')
def get_lens(lens_id):
    """Get lens details"""
    try:
        data = memex_get(f"/api/lenses/{lens_id}")
        return jsonify(data)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/extract', methods=['POST'])
def extract():
    """Extract anchors from text using lens"""
    data = request.json
    content = data.get("content", "")
    lens_id = data.get("lens_id", "org-planning")
    store = data.get("store", True)

    if not content:
        return jsonify({"error": "No content provided"}), 400

    try:
        start_time = time.time()

        # Extract anchors
        anchors = extract_with_lens(content, lens_id)

        # Build annotated HTML
        annotated_html = build_annotated_html(content, anchors)

        # Store in memex if requested
        source_id = None
        if store and anchors:
            source_id = store_extraction(content, lens_id, anchors)

        elapsed = time.time() - start_time

        return jsonify({
            "anchors": anchors,
            "annotated_html": annotated_html,
            "source_id": source_id,
            "lens_id": lens_id if lens_id.startswith("lens:") else f"lens:{lens_id}",
            "time": round(elapsed, 2)
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/documents')
def list_documents():
    """List processed documents (sources with anchors)"""
    try:
        # Get all source nodes
        sources = memex_get("/api/query/filter?type=Source&limit=50")
        return jsonify(sources)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/document/<path:doc_id>')
def get_document(doc_id):
    """Get document with its anchors"""
    try:
        # Get the source node
        source = memex_get(f"/api/nodes/{doc_id}")

        # Get entities linked to this source
        links = memex_get(f"/api/nodes/{doc_id}/links")

        # Get the entities that were EXTRACTED_FROM this source
        # (links are outgoing, so we need to look at incoming)
        # For now, search for entities mentioning this source

        return jsonify({
            "source": source,
            "links": links
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/graph/<lens_id>')
def get_graph(lens_id):
    """Get graph data for visualization"""
    try:
        # Export lens subgraph
        data = memex_get(f"/api/graph/export?lens_id={lens_id}")

        # Format for D3.js
        nodes = []
        edges = []

        # Add lens node
        if data.get("lens"):
            lens = data["lens"]
            nodes.append({
                "id": lens["ID"],
                "type": "Lens",
                "label": lens.get("Meta", {}).get("name", lens["ID"]),
                "group": "lens"
            })

        # Add entity nodes
        for entity in (data.get("entities") or []):
            meta = entity.get("Meta", {})
            nodes.append({
                "id": entity["ID"],
                "type": entity["Type"],
                "label": meta.get("name", entity["ID"]),
                "group": "entity",
                "patterns": meta.get("matched_patterns", [])
            })

        # Add edges
        for link in (data.get("links") or []):
            edges.append({
                "source": link["source"],
                "target": link["target"],
                "type": link["type"]
            })

        return jsonify({
            "nodes": nodes,
            "edges": edges,
            "stats": data.get("stats", {})
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/stats')
def get_stats():
    """Get demo statistics"""
    try:
        lenses = memex_get("/api/lenses")
        lens_count = lenses.get("count", 0)

        # Count anchors (nodes that are anchor:*)
        # This is approximate - searches for "anchor:" in IDs
        anchors = memex_get("/api/query/search?q=anchor:&limit=1000")
        anchor_count = anchors.get("count", 0)

        return jsonify({
            "lenses": lens_count,
            "anchors": anchor_count
        })
    except Exception as e:
        return jsonify({"lenses": 0, "anchors": 0})


if __name__ == '__main__':
    print("Starting Anchor Demo...")
    print("Ensuring default lenses...")
    ensure_default_lenses()
    print(f"Open http://localhost:5002")
    app.run(host='0.0.0.0', port=5002, debug=True)
