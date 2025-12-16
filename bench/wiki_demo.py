#!/usr/bin/env python3
"""
Wikipedia Memex Demo - Surface the hidden editorial context behind Wikipedia articles.

Features:
- Ingest Wikipedia article + full revision history + editor profiles
- Detect contentious paragraphs (edit wars, frequent reverts)
- Show who wrote what and editor reputation
- Annotate the page with "notes" about editorial context

Usage:
    python wiki_demo.py
    # Open http://localhost:5002
    # Paste a Wikipedia URL and see the hidden context
"""

import json
import re
import hashlib
from datetime import datetime, timedelta
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Optional
import requests
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS
from neo4j import GraphDatabase

app = Flask(__name__)
CORS(app)

# Neo4j connection (same as main memex)
NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "password"

driver = None

def get_driver():
    global driver
    if driver is None:
        driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
    return driver


# ============== Wikipedia API ==============

WIKI_API = "https://en.wikipedia.org/w/api.php"
HEADERS = {
    "User-Agent": "MemexWikiDemo/1.0 (https://memex.systems; contact@memex.systems)"
}

def extract_title_from_url(url: str) -> str:
    """Extract article title from Wikipedia URL."""
    # Handle various Wikipedia URL formats
    patterns = [
        r'wikipedia\.org/wiki/([^#?]+)',
        r'wikipedia\.org/w/index\.php\?title=([^&]+)',
    ]
    for pattern in patterns:
        match = re.search(pattern, url)
        if match:
            return match.group(1).replace('_', ' ')
    return url  # Assume it's already a title


def fetch_article_content(title: str) -> dict:
    """Fetch article HTML and basic info."""
    params = {
        "action": "parse",
        "page": title,
        "format": "json",
        "prop": "text|sections|categories",
        "disableeditsection": "true",
    }
    print(f"Fetching article: {title}")
    resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=30)

    if resp.status_code != 200:
        return {"error": f"HTTP {resp.status_code}"}

    if not resp.text:
        return {"error": "Empty response from Wikipedia"}

    try:
        data = resp.json()
    except Exception as e:
        return {"error": f"JSON parse error: {e}"}

    if "error" in data:
        return {"error": data["error"]["info"]}

    parse = data.get("parse", {})
    return {
        "title": parse.get("title", title),
        "pageid": parse.get("pageid"),
        "html": parse.get("text", {}).get("*", ""),
        "sections": parse.get("sections", []),
        "categories": [c["*"] for c in parse.get("categories", [])],
    }


def fetch_revision_history(title: str, limit: int = 500) -> list[dict]:
    """Fetch revision history with diffs."""
    params = {
        "action": "query",
        "titles": title,
        "prop": "revisions",
        "rvprop": "ids|timestamp|user|userid|comment|size|tags",
        "rvlimit": str(limit),
        "format": "json",
    }
    print(f"Fetching revisions for: {title}")
    try:
        resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=30)
        data = resp.json()
    except Exception as e:
        print(f"Error fetching revisions: {e}")
        return []

    pages = data.get("query", {}).get("pages", {})
    for page_id, page_data in pages.items():
        if page_id == "-1":
            return []
        return page_data.get("revisions", [])
    return []


def fetch_user_contributions(username: str, limit: int = 100) -> dict:
    """Fetch user's contribution summary."""
    params = {
        "action": "query",
        "list": "usercontribs",
        "ucuser": username,
        "uclimit": str(limit),
        "ucprop": "title|timestamp|comment|tags|sizediff",
        "format": "json",
    }
    resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=30)
    data = resp.json()

    contribs = data.get("query", {}).get("usercontribs", [])

    # Analyze contribution patterns
    topics = defaultdict(int)
    total_edits = len(contribs)
    reverts = 0

    for c in contribs:
        # Count topic areas
        title = c.get("title", "")
        if ":" in title:
            namespace = title.split(":")[0]
            topics[namespace] += 1
        else:
            topics["Article"] += 1

        # Count reverts
        comment = c.get("comment", "").lower()
        if any(word in comment for word in ["revert", "rv", "undid", "undo"]):
            reverts += 1

    return {
        "username": username,
        "total_edits": total_edits,
        "reverts": reverts,
        "revert_ratio": reverts / max(total_edits, 1),
        "top_topics": sorted(topics.items(), key=lambda x: -x[1])[:5],
    }


# ============== Analysis ==============

@dataclass
class EditEvent:
    """A single edit to the article."""
    revid: int
    timestamp: str
    user: str
    comment: str
    size_diff: int = 0
    is_revert: bool = False
    section: str = ""


@dataclass
class ContentionScore:
    """How contentious a section is."""
    section: str
    edit_count: int = 0
    unique_editors: int = 0
    revert_count: int = 0
    edit_war_detected: bool = False
    score: float = 0.0  # 0-1, higher = more contentious
    notable_editors: list = field(default_factory=list)
    recent_disputes: list = field(default_factory=list)


def analyze_revisions(revisions: list[dict]) -> dict:
    """Analyze revision history for patterns."""
    if not revisions:
        return {"total_edits": 0, "editors": {}, "contentious_sections": []}

    editors = defaultdict(lambda: {"edit_count": 0, "reverts_made": 0, "reverts_received": 0})
    section_edits = defaultdict(list)
    revert_chains = []

    prev_user = None
    prev_size = None

    for i, rev in enumerate(revisions):
        user = rev.get("user", "Anonymous")
        comment = rev.get("comment", "")
        size = rev.get("size", 0)
        timestamp = rev.get("timestamp", "")

        # Track editor stats
        editors[user]["edit_count"] += 1

        # Detect reverts
        is_revert = False
        comment_lower = comment.lower()
        if any(word in comment_lower for word in ["revert", "rv", "undid", "undo", "rollback"]):
            is_revert = True
            editors[user]["reverts_made"] += 1
            if prev_user:
                editors[prev_user]["reverts_received"] += 1

        # Try to identify section from comment
        section = "General"
        section_match = re.search(r'/\*\s*([^*]+)\s*\*/', comment)
        if section_match:
            section = section_match.group(1).strip()

        section_edits[section].append({
            "user": user,
            "timestamp": timestamp,
            "comment": comment,
            "is_revert": is_revert,
            "size_diff": size - prev_size if prev_size else 0,
        })

        prev_user = user
        prev_size = size

    # Calculate contention scores per section
    contentious_sections = []
    for section, edits in section_edits.items():
        unique_eds = len(set(e["user"] for e in edits))
        reverts = sum(1 for e in edits if e["is_revert"])

        # Detect edit wars (back-and-forth between users)
        edit_war = False
        if len(edits) >= 4:
            recent = edits[:10]
            user_sequence = [e["user"] for e in recent]
            # Check for A-B-A-B pattern
            for i in range(len(user_sequence) - 3):
                if (user_sequence[i] == user_sequence[i+2] and
                    user_sequence[i+1] == user_sequence[i+3] and
                    user_sequence[i] != user_sequence[i+1]):
                    edit_war = True
                    break

        # Calculate score (0-1)
        score = min(1.0, (
            (len(edits) / 50) * 0.3 +  # Edit frequency
            (reverts / max(len(edits), 1)) * 0.4 +  # Revert ratio
            (1.0 if edit_war else 0.0) * 0.3  # Edit war bonus
        ))

        if score > 0.2 or len(edits) > 10:  # Only track notable sections
            contentious_sections.append({
                "section": section,
                "edit_count": len(edits),
                "unique_editors": unique_eds,
                "revert_count": reverts,
                "edit_war": edit_war,
                "score": round(score, 2),
                "recent_edits": edits[:5],
            })

    # Sort by contention score
    contentious_sections.sort(key=lambda x: -x["score"])

    # Identify notable editors (high activity or high revert ratio)
    notable_editors = []
    for user, stats in editors.items():
        if stats["edit_count"] >= 3 or stats["reverts_made"] >= 2:
            revert_ratio = stats["reverts_made"] / max(stats["edit_count"], 1)
            notable_editors.append({
                "user": user,
                "edit_count": stats["edit_count"],
                "reverts_made": stats["reverts_made"],
                "reverts_received": stats["reverts_received"],
                "revert_ratio": round(revert_ratio, 2),
                "is_contentious": revert_ratio > 0.3 or stats["reverts_received"] > 2,
            })

    notable_editors.sort(key=lambda x: -x["edit_count"])

    return {
        "total_edits": len(revisions),
        "total_editors": len(editors),
        "editors": dict(editors),
        "notable_editors": notable_editors[:20],
        "contentious_sections": contentious_sections[:10],
        "time_span": {
            "first": revisions[-1].get("timestamp") if revisions else None,
            "last": revisions[0].get("timestamp") if revisions else None,
        }
    }


# ============== Neo4j Storage ==============

def store_article_in_memex(title: str, content: dict, analysis: dict):
    """Store article and its editorial context in memex."""
    with get_driver().session() as session:
        # Create article node
        article_id = f"wiki:{title.lower().replace(' ', '_')}"
        session.run("""
            MERGE (a:Node {id: $id})
            SET a.type = 'WikiArticle',
                a.properties = $props,
                a.updated = datetime()
        """, id=article_id, props=json.dumps({
            "title": title,
            "categories": content.get("categories", []),
            "total_edits": analysis.get("total_edits", 0),
        }))

        # Create editor nodes and relationships
        for editor in analysis.get("notable_editors", []):
            editor_id = f"wiki_editor:{editor['user'].lower().replace(' ', '_')}"
            session.run("""
                MERGE (e:Node {id: $id})
                SET e.type = 'WikiEditor',
                    e.properties = $props
            """, id=editor_id, props=json.dumps({
                "username": editor["user"],
                "edit_count": editor["edit_count"],
                "revert_ratio": editor["revert_ratio"],
                "is_contentious": editor["is_contentious"],
            }))

            # Link editor to article
            session.run("""
                MATCH (a:Node {id: $article_id}), (e:Node {id: $editor_id})
                MERGE (e)-[r:LINK {type: 'EDITED'}]->(a)
                SET r.properties = $props
            """, article_id=article_id, editor_id=editor_id, props=json.dumps({
                "edit_count": editor["edit_count"],
                "reverts_made": editor["reverts_made"],
            }))

        # Create contentious section nodes
        for section in analysis.get("contentious_sections", []):
            section_id = f"{article_id}:section:{section['section'].lower().replace(' ', '_')}"
            session.run("""
                MERGE (s:Node {id: $id})
                SET s.type = 'WikiSection',
                    s.properties = $props
            """, id=section_id, props=json.dumps({
                "name": section["section"],
                "contention_score": section["score"],
                "edit_war": section["edit_war"],
            }))

            # Link section to article
            session.run("""
                MATCH (a:Node {id: $article_id}), (s:Node {id: $section_id})
                MERGE (a)-[r:LINK {type: 'HAS_SECTION'}]->(s)
            """, article_id=article_id, section_id=section_id)


# ============== HTML Annotation ==============

def annotate_html(html: str, analysis: dict) -> str:
    """Add annotation markers to HTML content."""
    # Create section contention map
    section_scores = {s["section"].lower(): s for s in analysis.get("contentious_sections", [])}

    # Add data attributes and classes for contentious sections
    annotated = html

    # Inject CSS for annotations
    style = """
    <style>
    .wiki-contentious {
        background: linear-gradient(to right, rgba(255,200,0,0.1), transparent);
        border-left: 3px solid #f0ad4e;
        padding-left: 8px;
        margin-left: -11px;
        position: relative;
    }
    .wiki-edit-war {
        background: linear-gradient(to right, rgba(255,100,100,0.15), transparent);
        border-left: 3px solid #d9534f;
    }
    .wiki-note {
        position: absolute;
        right: -200px;
        width: 180px;
        background: #fffbea;
        border: 1px solid #f0ad4e;
        border-radius: 4px;
        padding: 8px;
        font-size: 11px;
        box-shadow: 2px 2px 4px rgba(0,0,0,0.1);
    }
    .wiki-note.war {
        background: #fff5f5;
        border-color: #d9534f;
    }
    .wiki-editor-tag {
        display: inline-block;
        background: #e8e8e8;
        padding: 2px 6px;
        border-radius: 3px;
        font-size: 10px;
        margin-left: 4px;
        cursor: help;
    }
    .wiki-editor-tag.contentious {
        background: #ffe0e0;
        color: #c00;
    }
    </style>
    """

    return style + annotated


# ============== Flask Routes ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html>
<head>
    <title>Wikipedia Memex - Editorial X-Ray</title>
    <link href="https://fonts.googleapis.com/css2?family=Caveat:wght@400;600&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            margin: 0;
            background: #fafafa;
        }
        .header {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            color: white;
            padding: 20px;
            text-align: center;
        }
        .header h1 { margin: 0 0 5px 0; font-size: 24px; }
        .header p { margin: 0; opacity: 0.7; font-size: 14px; }
        .input-section {
            max-width: 700px;
            margin: 20px auto;
            padding: 15px 20px;
            background: white;
            border-radius: 50px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
            display: flex;
            gap: 10px;
        }
        input[type="text"] {
            flex: 1;
            padding: 12px 20px;
            border: none;
            font-size: 14px;
            outline: none;
        }
        button {
            padding: 12px 30px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 25px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
        }
        button:hover { opacity: 0.9; transform: scale(1.02); }
        button:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
        .loading {
            text-align: center;
            padding: 60px;
            color: #666;
        }
        .loading::after {
            content: '';
            animation: dots 1.5s infinite;
        }
        @keyframes dots {
            0%, 20% { content: '.'; }
            40% { content: '..'; }
            60%, 100% { content: '...'; }
        }
        .article-container {
            max-width: 1300px;
            margin: 0 auto;
            padding: 20px;
        }
        .article-panel {
            background: white;
            border-radius: 12px;
            padding: 40px 60px;
            box-shadow: 0 2px 20px rgba(0,0,0,0.08);
            position: relative;
            line-height: 1.8;
        }
        .article-panel img { max-width: 100%; }

        /* Scribble note styles */
        .scribble-note {
            font-family: 'Caveat', cursive;
            position: absolute;
            right: -220px;
            width: 200px;
            padding: 12px 15px;
            font-size: 16px;
            line-height: 1.4;
            transform: rotate(-1deg);
            z-index: 10;
        }
        .scribble-note.warning {
            background: #fff3cd;
            border-left: 3px solid #ff9800;
            color: #856404;
        }
        .scribble-note.danger {
            background: #ffe0e0;
            border-left: 3px solid #e53935;
            color: #c62828;
        }
        .scribble-note.info {
            background: #e3f2fd;
            border-left: 3px solid #2196f3;
            color: #1565c0;
        }
        .scribble-note::before {
            content: '✎';
            position: absolute;
            left: -20px;
            top: 10px;
            font-size: 20px;
            opacity: 0.5;
        }
        .scribble-note .note-icon {
            font-size: 18px;
            margin-right: 5px;
        }

        /* Inline highlights */
        .highlight-contentious {
            background: linear-gradient(180deg, transparent 60%, rgba(255,200,0,0.3) 60%);
            cursor: help;
            position: relative;
        }
        .highlight-war {
            background: linear-gradient(180deg, transparent 60%, rgba(255,100,100,0.4) 60%);
            cursor: help;
        }
        .highlight-recent {
            background: linear-gradient(180deg, transparent 60%, rgba(100,200,255,0.3) 60%);
            cursor: help;
        }

        /* Margin annotations */
        .margin-note {
            position: absolute;
            left: -45px;
            width: 35px;
            height: 35px;
            background: #ff9800;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-size: 14px;
            cursor: pointer;
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
            transition: transform 0.2s;
        }
        .margin-note:hover { transform: scale(1.1); }
        .margin-note.war { background: #e53935; }
        .margin-note.info { background: #2196f3; }

        /* Section annotations */
        .annotated-section {
            position: relative;
            padding: 15px 20px;
            margin: 20px -20px;
            border-radius: 8px;
            transition: background 0.3s;
        }
        .annotated-section.hot {
            background: rgba(255,200,0,0.08);
            border-left: 4px solid #ff9800;
        }
        .annotated-section.war {
            background: rgba(255,100,100,0.08);
            border-left: 4px solid #e53935;
        }
        .annotated-section:hover {
            background: rgba(255,200,0,0.15);
        }

        /* Floating insight cards */
        .insight-card {
            width: 320px;
            flex-shrink: 0;
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 30px rgba(0,0,0,0.15);
            padding: 20px;
            z-index: 100;
            max-height: 80vh;
            overflow-y: auto;
            position: sticky;
            top: 20px;
        }
        .insight-card h3 {
            margin: 0 0 15px 0;
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
            color: #666;
        }
        .insight-item {
            padding: 12px;
            margin: 8px 0;
            border-radius: 8px;
            font-size: 13px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .insight-item:hover { transform: translateX(5px); }
        .insight-item.danger {
            background: linear-gradient(135deg, #ffe0e0, #fff);
            border-left: 3px solid #e53935;
        }
        .insight-item.warning {
            background: linear-gradient(135deg, #fff3cd, #fff);
            border-left: 3px solid #ff9800;
        }
        .insight-item .title {
            font-weight: 600;
            margin-bottom: 4px;
        }
        .insight-item .detail {
            color: #666;
            font-size: 11px;
        }
        .insight-item .scribble {
            font-family: 'Caveat', cursive;
            font-size: 15px;
            color: #c62828;
            margin-top: 6px;
        }

        /* Editor tags inline */
        .editor-tag {
            display: inline-block;
            font-family: 'Caveat', cursive;
            font-size: 14px;
            padding: 2px 8px;
            background: #f0f0f0;
            border-radius: 4px;
            margin-left: 5px;
            color: #666;
        }
        .editor-tag.contentious {
            background: #ffe0e0;
            color: #c62828;
        }

        /* Tooltip for highlights */
        .wiki-tooltip {
            position: absolute;
            background: #333;
            color: white;
            padding: 10px 14px;
            border-radius: 8px;
            font-size: 12px;
            z-index: 1000;
            max-width: 300px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.3);
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.2s;
        }
        .wiki-tooltip.visible { opacity: 1; }
        .wiki-tooltip .tip-title {
            font-weight: 600;
            margin-bottom: 5px;
            color: #ffd700;
        }
        .wiki-tooltip .tip-scribble {
            font-family: 'Caveat', cursive;
            font-size: 15px;
            color: #ffeb3b;
            margin-top: 8px;
            border-top: 1px solid #555;
            padding-top: 8px;
        }

        /* Summary banner */
        .summary-banner {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px 30px;
            border-radius: 12px;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-around;
            text-align: center;
        }
        .summary-stat {
            padding: 0 20px;
        }
        .summary-stat .number {
            font-size: 32px;
            font-weight: 600;
        }
        .summary-stat .label {
            font-size: 12px;
            opacity: 0.8;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        @media (max-width: 1000px) {
            .scribble-note { display: none; }
            .insight-card { position: static; width: 100%; margin-bottom: 20px; }
            .article-container { padding: 10px; }
            .article-container > div { flex-direction: column; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Wikipedia Memex</h1>
        <p>See the hidden editorial battles behind every article</p>
    </div>

    <div class="input-section">
        <input type="text" id="url-input" placeholder="Paste any Wikipedia URL..." />
        <button onclick="analyzeArticle()" id="analyze-btn">X-Ray</button>
    </div>

    <div id="loading" class="loading" style="display:none">
        Analyzing editorial history
    </div>

    <div id="content" style="display:none">
        <div class="article-container">
            <div id="summary-banner" class="summary-banner"></div>
            <div style="display:flex;gap:20px;">
                <div class="article-panel" id="article-html" style="flex:1;"></div>
                <div class="insight-card" id="insights"></div>
            </div>
        </div>
    </div>

    <div id="tooltip" class="wiki-tooltip"></div>

    <script>
    let analysisData = null;

    async function analyzeArticle() {
        const url = document.getElementById('url-input').value.trim();
        if (!url) return;

        document.getElementById('analyze-btn').disabled = true;
        document.getElementById('loading').style.display = 'block';
        document.getElementById('content').style.display = 'none';

        try {
            const resp = await fetch('/api/analyze', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({url})
            });
            analysisData = await resp.json();

            if (analysisData.error) {
                alert('Error: ' + analysisData.error);
                return;
            }

            renderResults(analysisData);
        } catch (err) {
            alert('Error: ' + err.message);
        } finally {
            document.getElementById('analyze-btn').disabled = false;
            document.getElementById('loading').style.display = 'none';
        }
    }

    function getScribbleComment(section, editors) {
        const comments = [];

        if (section.edit_war) {
            comments.push("⚠️ Active edit war! Multiple editors fighting over this.");
        }
        if (section.revert_count > 20) {
            comments.push(`🔄 Heavily contested - ${section.revert_count} reverts!`);
        }
        if (section.unique_editors > 50) {
            comments.push(`👥 ${section.unique_editors} different people edited this`);
        }

        // Find contentious editors for this section
        const contentious = editors.filter(e => e.is_contentious);
        if (contentious.length > 0) {
            const names = contentious.slice(0,2).map(e => e.user).join(', ');
            comments.push(`👀 Watch out: ${names} known for edit wars`);
        }

        if (section.recent_edits?.length > 0) {
            const lastEdit = section.recent_edits[0];
            const daysAgo = Math.floor((Date.now() - new Date(lastEdit.timestamp)) / 86400000);
            if (daysAgo < 7) {
                comments.push(`✏️ Recently edited ${daysAgo === 0 ? 'today' : daysAgo + ' days ago'} by ${lastEdit.user}`);
            }
        }

        return comments[Math.floor(Math.random() * comments.length)] || "📝 This section sees regular edits";
    }

    function renderResults(data) {
        // Summary banner
        const editWarCount = data.analysis.contentious_sections.filter(s => s.edit_war).length;
        const contentiousEditors = data.analysis.notable_editors.filter(e => e.is_contentious).length;

        document.getElementById('summary-banner').innerHTML = `
            <div class="summary-stat">
                <div class="number">${data.analysis.total_edits}</div>
                <div class="label">Total Edits</div>
            </div>
            <div class="summary-stat">
                <div class="number">${data.analysis.total_editors}</div>
                <div class="label">Editors</div>
            </div>
            <div class="summary-stat">
                <div class="number">${editWarCount}</div>
                <div class="label">Edit Wars</div>
            </div>
            <div class="summary-stat">
                <div class="number">${contentiousEditors}</div>
                <div class="label">Contentious Users</div>
            </div>
        `;

        // Insights panel
        let insightsHtml = '<h3>⚡ Key Insights</h3>';

        console.log('Contentious sections:', data.analysis.contentious_sections);
        console.log('Notable editors:', data.analysis.notable_editors);

        // Add warnings for edit wars and high-score sections
        data.analysis.contentious_sections.forEach(s => {
            console.log(`Section ${s.section}: score=${s.score}, edit_war=${s.edit_war}`);
            if (s.edit_war || s.score > 0.5) {
                const scribble = getScribbleComment(s, data.analysis.notable_editors);
                insightsHtml += `
                    <div class="insight-item ${s.edit_war ? 'danger' : 'warning'}">
                        <div class="title">${s.edit_war ? '⚔️' : '🔥'} ${s.section}</div>
                        <div class="detail">${s.edit_count} edits · ${s.revert_count} reverts · ${s.unique_editors} editors</div>
                        <div class="scribble">"${scribble}"</div>
                    </div>
                `;
            }
        });

        // If no high-score sections, show the top section anyway
        if (!data.analysis.contentious_sections.some(s => s.edit_war || s.score > 0.5)) {
            const top = data.analysis.contentious_sections[0];
            if (top) {
                insightsHtml += `
                    <div class="insight-item warning">
                        <div class="title">🔥 ${top.section}</div>
                        <div class="detail">${top.edit_count} edits · ${top.revert_count} reverts</div>
                        <div class="scribble">"Most edited section"</div>
                    </div>
                `;
            }
        }

        // Add contentious editors
        const contentious = data.analysis.notable_editors.filter(e => e.is_contentious);
        if (contentious.length > 0) {
            insightsHtml += '<h3 style="margin-top:20px;">👤 Watch These Editors</h3>';
            contentious.slice(0,5).forEach(e => {
                insightsHtml += `
                    <div class="insight-item danger">
                        <div class="title">⚠️ ${e.user}</div>
                        <div class="detail">${e.edit_count} edits · ${e.reverts_made} reverts · ${Math.round(e.revert_ratio*100)}% conflict rate</div>
                        <div class="scribble">"Known for argumentative editing"</div>
                    </div>
                `;
            });
        } else if (data.analysis.notable_editors.length > 0) {
            // Show top editors even if none are "contentious"
            insightsHtml += '<h3 style="margin-top:20px;">👥 Top Editors</h3>';
            data.analysis.notable_editors.slice(0,3).forEach(e => {
                insightsHtml += `
                    <div class="insight-item warning">
                        <div class="title">${e.user}</div>
                        <div class="detail">${e.edit_count} edits on this article</div>
                    </div>
                `;
            });
        }

        // Always show something
        if (insightsHtml === '<h3>⚡ Key Insights</h3>') {
            insightsHtml += '<div style="color:#666;font-size:13px;">This article appears relatively stable with no major editorial conflicts detected.</div>';
        }

        document.getElementById('insights').innerHTML = insightsHtml;
        console.log('Final insights HTML length:', insightsHtml.length);

        // Annotate HTML
        let html = data.content.html;

        // Add section-level annotations
        data.analysis.contentious_sections.forEach((section, idx) => {
            if (section.score > 0.3) {
                const scribble = getScribbleComment(section, data.analysis.notable_editors);
                const noteClass = section.edit_war ? 'danger' : 'warning';

                // Create margin scribble note
                const noteHtml = `
                    <div class="scribble-note ${noteClass}" style="top: ${100 + idx * 120}px;">
                        <span class="note-icon">${section.edit_war ? '⚔️' : '⚠️'}</span>
                        ${scribble}
                    </div>
                `;

                // Try to inject near section headers
                const sectionId = section.section.replace(/\\s+/g, '_');
                const patterns = [
                    new RegExp(`(<h[23][^>]*>\\s*<span[^>]*class="[^"]*mw-headline[^"]*"[^>]*id="${sectionId}"[^>]*>)`, 'i'),
                    new RegExp(`(<h[23][^>]*>\\s*<span[^>]*>${section.section}</span>)`, 'i'),
                ];

                for (const pattern of patterns) {
                    if (pattern.test(html)) {
                        html = html.replace(pattern, `${noteHtml}<div class="annotated-section ${section.edit_war ? 'war' : 'hot'}">$1`);
                        break;
                    }
                }
            }
        });

        // Add highlights to first paragraph with info about edits
        const firstParagraph = data.analysis.contentious_sections.find(s => s.section === 'General');
        if (firstParagraph && firstParagraph.edit_count > 100) {
            html = html.replace(
                /(<p[^>]*>)(.{100,300}?)(\\.<)/,
                `$1<span class="highlight-contentious" data-info="This intro has been edited ${firstParagraph.edit_count} times by ${firstParagraph.unique_editors} different people">$2</span>$3`
            );
        }

        document.getElementById('article-html').innerHTML = html;
        document.getElementById('content').style.display = 'block';

        // Setup tooltip handlers
        setupTooltips();
    }

    function setupTooltips() {
        const tooltip = document.getElementById('tooltip');

        document.querySelectorAll('.highlight-contentious, .highlight-war').forEach(el => {
            el.addEventListener('mouseenter', (e) => {
                const info = el.dataset.info || 'This text has been frequently edited';
                tooltip.innerHTML = `
                    <div class="tip-title">Editorial Activity</div>
                    ${info}
                    <div class="tip-scribble">"Be aware: this is contested content"</div>
                `;
                tooltip.style.left = e.pageX + 10 + 'px';
                tooltip.style.top = e.pageY + 10 + 'px';
                tooltip.classList.add('visible');
            });

            el.addEventListener('mouseleave', () => {
                tooltip.classList.remove('visible');
            });
        });
    }

    // Handle Enter key
    document.getElementById('url-input').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') analyzeArticle();
    });
    </script>
</body>
</html>
"""


@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/analyze', methods=['POST'])
def analyze():
    data = request.json
    url = data.get('url', '')

    if not url:
        return jsonify({"error": "URL required"})

    try:
        # Extract title
        title = extract_title_from_url(url)
        print(f"Analyzing: {title}")

        # Fetch content
        content = fetch_article_content(title)
        if "error" in content:
            return jsonify({"error": content["error"]})

        # Fetch revisions
        print("Fetching revision history...")
        revisions = fetch_revision_history(title, limit=500)

        # Analyze
        print(f"Analyzing {len(revisions)} revisions...")
        analysis = analyze_revisions(revisions)

        # Store in memex (optional, non-blocking)
        try:
            store_article_in_memex(title, content, analysis)
        except Exception as e:
            print(f"Warning: Could not store in memex: {e}")

        return jsonify({
            "title": content["title"],
            "content": {
                "html": content["html"],
                "categories": content["categories"],
            },
            "analysis": analysis,
        })

    except Exception as e:
        import traceback
        traceback.print_exc()
        return jsonify({"error": str(e)})


@app.route('/api/editor/<username>')
def get_editor(username):
    """Get detailed editor profile."""
    try:
        profile = fetch_user_contributions(username)
        return jsonify(profile)
    except Exception as e:
        return jsonify({"error": str(e)})


if __name__ == '__main__':
    print("Starting Wikipedia Memex Demo...")
    print("Open http://localhost:5002")
    app.run(host='0.0.0.0', port=5002, debug=True)
