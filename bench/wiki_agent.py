#!/usr/bin/env python3
"""
Wikipedia AI Agent - Analyzes article history and writes intelligent annotations.

This agent:
1. Fetches the article + full revision history with diffs
2. Identifies contentious paragraphs by analyzing what text was changed/reverted
3. Uses an LLM to write insightful "scribble notes" about editorial context
4. Returns the article with AI annotations embedded

Usage:
    python wiki_agent.py
    # Open http://localhost:5002
"""

import json
import re
import difflib
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Optional
import requests
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS
from openai import OpenAI
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# Wikipedia API
WIKI_API = "https://en.wikipedia.org/w/api.php"
HEADERS = {"User-Agent": "MemexWikiAgent/1.0 (research project)"}

# OpenAI client
llm = OpenAI()


def get_article_content(title: str) -> dict:
    """Get article HTML and plain text."""
    params = {
        "action": "parse",
        "page": title,
        "format": "json",
        "prop": "text|wikitext|sections",
    }
    resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=30)
    data = resp.json()

    if "error" in data:
        return {"error": data["error"]["info"]}

    parse = data.get("parse", {})
    return {
        "title": parse.get("title"),
        "html": parse.get("text", {}).get("*", ""),
        "wikitext": parse.get("wikitext", {}).get("*", ""),
        "sections": parse.get("sections", []),
    }


def get_revision_diffs(title: str, limit: int = 100) -> list[dict]:
    """Get revision history with actual content diffs."""
    # First get revision IDs
    params = {
        "action": "query",
        "titles": title,
        "prop": "revisions",
        "rvprop": "ids|timestamp|user|comment|size",
        "rvlimit": str(limit),
        "format": "json",
    }
    resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=30)
    data = resp.json()

    pages = data.get("query", {}).get("pages", {})
    revisions = []
    for page_id, page_data in pages.items():
        if page_id != "-1":
            revisions = page_data.get("revisions", [])
            break

    # Now get diffs for interesting revisions (reverts, large changes)
    diffs = []
    for i, rev in enumerate(revisions[:50]):  # Limit to recent 50
        comment = rev.get("comment", "").lower()
        is_revert = any(w in comment for w in ["revert", "undo", "undid", "rv"])

        # Get diff for reverts and significant edits
        if is_revert or (i < 20):  # Always get recent 20
            diff_data = get_single_diff(rev.get("revid"), rev.get("parentid"))
            if diff_data:
                diffs.append({
                    "revid": rev.get("revid"),
                    "timestamp": rev.get("timestamp"),
                    "user": rev.get("user"),
                    "comment": rev.get("comment", ""),
                    "is_revert": is_revert,
                    "diff": diff_data,
                })

    return diffs


def get_single_diff(revid: int, parentid: int) -> Optional[str]:
    """Get the actual text diff between two revisions."""
    if not parentid:
        return None

    params = {
        "action": "compare",
        "fromrev": parentid,
        "torev": revid,
        "format": "json",
    }
    try:
        resp = requests.get(WIKI_API, params=params, headers=HEADERS, timeout=10)
        data = resp.json()
        return data.get("compare", {}).get("*", "")
    except:
        return None


def extract_changed_text(diff_html: str) -> dict:
    """Extract added and removed text from diff HTML."""
    added = []
    removed = []

    # Remove HTML tags but keep text content
    def strip_tags(html):
        return re.sub(r'<[^>]+>', ' ', html).strip()

    # Wikipedia diffs use tables with diff-deletedline and diff-addedline classes
    # Extract from deleted lines (left side of diff)
    for match in re.finditer(r'<td[^>]*class="[^"]*diff-deletedline[^"]*"[^>]*>(.*?)</td>', diff_html, re.DOTALL):
        text = strip_tags(match.group(1)).strip()
        text = re.sub(r'\s+', ' ', text)  # Normalize whitespace
        if len(text) > 5:
            removed.append(text)

    # Extract from added lines (right side of diff)
    for match in re.finditer(r'<td[^>]*class="[^"]*diff-addedline[^"]*"[^>]*>(.*?)</td>', diff_html, re.DOTALL):
        text = strip_tags(match.group(1)).strip()
        text = re.sub(r'\s+', ' ', text)
        if len(text) > 5:
            added.append(text)

    # Also check for ins/del tags (inline changes)
    for match in re.finditer(r'<del[^>]*>([^<]+)</del>', diff_html):
        text = match.group(1).strip()
        if len(text) > 5 and text not in removed:
            removed.append(text)

    for match in re.finditer(r'<ins[^>]*>([^<]+)</ins>', diff_html):
        text = match.group(1).strip()
        if len(text) > 5 and text not in added:
            added.append(text)

    return {"added": added, "removed": removed}


def analyze_paragraph_contention(wikitext: str, diffs: list[dict]) -> list[dict]:
    """Analyze which paragraphs have been most contested."""
    # Split into paragraphs
    paragraphs = [p.strip() for p in wikitext.split('\n\n') if p.strip() and len(p.strip()) > 50]

    # Debug: count total changes found
    total_changes = 0
    for diff in diffs:
        if diff.get("diff"):
            changes = extract_changed_text(diff["diff"])
            total_changes += len(changes["added"]) + len(changes["removed"])
    print(f"Total text changes found across all diffs: {total_changes}")

    paragraph_stats = []
    for i, para in enumerate(paragraphs[:30]):  # Limit to first 30 paragraphs
        # Count how many diffs touched this paragraph
        edit_count = 0
        revert_count = 0
        editors = set()
        sample_changes = []

        # Use more words from paragraph for better matching
        para_lower = para.lower()
        para_words = set(re.findall(r'\b\w+\b', para_lower)[:50])

        for diff in diffs:
            if not diff.get("diff"):
                continue

            changes = extract_changed_text(diff["diff"])

            # Check if this diff touched this paragraph
            for text in changes["added"] + changes["removed"]:
                text_words = set(re.findall(r'\b\w+\b', text.lower()))
                overlap = para_words & text_words
                # Lower threshold: at least 2 significant words overlap
                significant_overlap = [w for w in overlap if len(w) > 3]
                if len(significant_overlap) >= 2:  # At least 2 significant words
                    edit_count += 1
                    editors.add(diff["user"])
                    if diff["is_revert"]:
                        revert_count += 1
                    if len(sample_changes) < 3:
                        sample_changes.append({
                            "user": diff["user"],
                            "timestamp": diff["timestamp"],
                            "is_revert": diff["is_revert"],
                            "comment": diff["comment"][:100],
                        })
                    break

        if edit_count > 0:
            paragraph_stats.append({
                "index": i,
                "preview": para[:200] + "..." if len(para) > 200 else para,
                "edit_count": edit_count,
                "revert_count": revert_count,
                "unique_editors": len(editors),
                "editors": list(editors)[:5],
                "sample_changes": sample_changes,
                "contention_score": min(1.0, (edit_count * 0.1) + (revert_count * 0.3)),
            })

    # Sort by contention
    paragraph_stats.sort(key=lambda x: -x["contention_score"])
    return paragraph_stats


def generate_ai_annotations(article_title: str, paragraph_stats: list[dict], diffs: list[dict]) -> list[dict]:
    """Use LLM to generate insightful annotations for contentious paragraphs."""

    if not paragraph_stats:
        return []

    # Prepare context for LLM
    contentious = [p for p in paragraph_stats if p["contention_score"] > 0.2][:5]

    if not contentious:
        return []

    # Build prompt
    prompt = f"""You are analyzing the Wikipedia article "{article_title}" to identify editorial conflicts and write margin notes for readers.

Here are the most contentious paragraphs based on edit history:

"""
    for i, para in enumerate(contentious):
        prompt += f"""
PARAGRAPH {i+1}:
Text preview: "{para['preview'][:150]}..."
- Edited {para['edit_count']} times by {para['unique_editors']} different editors
- Reverted {para['revert_count']} times
- Recent editors: {', '.join(para['editors'][:3])}
- Sample edit comment: "{para['sample_changes'][0]['comment'] if para['sample_changes'] else 'N/A'}"
"""

    prompt += """

For each paragraph, write a SHORT (1-2 sentence) margin note that a reader would find helpful. The note should:
- Sound like a knowledgeable friend scribbling a warning/insight in the margin
- Be specific about WHY this text is contested (if you can infer it)
- Use casual but informative tone

Return as JSON array:
[
  {"paragraph_index": 0, "note": "your note here", "severity": "high/medium/low"},
  ...
]

Only include paragraphs worth annotating. Be concise and insightful."""

    try:
        response = llm.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            temperature=0.7,
            max_tokens=1000,
        )

        content = response.choices[0].message.content
        # Extract JSON from response
        json_match = re.search(r'\[[\s\S]*\]', content)
        if json_match:
            annotations = json.loads(json_match.group())
            # Merge with paragraph data
            for ann in annotations:
                idx = ann.get("paragraph_index", 0)
                if idx < len(contentious):
                    ann["preview"] = contentious[idx]["preview"]
                    ann["edit_count"] = contentious[idx]["edit_count"]
                    ann["revert_count"] = contentious[idx]["revert_count"]
            return annotations
    except Exception as e:
        print(f"LLM error: {e}")

    return []


def annotate_html_with_notes(html: str, wikitext: str, annotations: list[dict]) -> str:
    """Inject AI annotations into the HTML at the right positions."""

    for ann in annotations:
        preview = ann.get("preview", "")[:50]
        note = ann.get("note", "")
        severity = ann.get("severity", "medium")

        if not preview or not note:
            continue

        # Find this text in HTML and wrap it
        # Use first 30 chars as search key
        search_text = re.escape(preview[:30])

        severity_class = {
            "high": "annotation-danger",
            "medium": "annotation-warning",
            "low": "annotation-info",
        }.get(severity, "annotation-warning")

        icon = {"high": "⚠️", "medium": "📝", "low": "💡"}.get(severity, "📝")

        annotation_html = f'''<span class="wiki-annotation {severity_class}" data-note="{note}">
            <span class="annotation-icon">{icon}</span>
            <span class="annotation-bubble">{note}</span>
        </span>'''

        # Try to insert annotation near matching text
        pattern = f'(<p[^>]*>)([^<]*{search_text})'
        replacement = f'\\1{annotation_html}\\2'
        html = re.sub(pattern, replacement, html, count=1, flags=re.IGNORECASE)

    return html


# ============== Flask Routes ==============

HTML_TEMPLATE = '''
<!DOCTYPE html>
<html>
<head>
    <title>Wikipedia X-Ray</title>
    <link href="https://fonts.googleapis.com/css2?family=Caveat:wght@400;600&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: 'Inter', sans-serif;
            margin: 0;
            background: #f8f9fa;
            color: #333;
        }
        .header {
            background: linear-gradient(135deg, #2c3e50, #3498db);
            color: white;
            padding: 25px;
            text-align: center;
        }
        .header h1 { margin: 0; font-size: 28px; }
        .header p { margin: 8px 0 0; opacity: 0.8; }

        .search-box {
            max-width: 600px;
            margin: -25px auto 20px;
            background: white;
            border-radius: 50px;
            padding: 8px 8px 8px 25px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.15);
            display: flex;
            gap: 10px;
        }
        .search-box input {
            flex: 1;
            border: none;
            font-size: 16px;
            outline: none;
        }
        .search-box button {
            background: linear-gradient(135deg, #e74c3c, #c0392b);
            color: white;
            border: none;
            padding: 12px 25px;
            border-radius: 25px;
            font-weight: 600;
            cursor: pointer;
        }
        .search-box button:hover { opacity: 0.9; }

        .loading {
            text-align: center;
            padding: 50px;
            font-size: 18px;
            color: #666;
        }
        .loading .spinner {
            width: 40px;
            height: 40px;
            border: 4px solid #eee;
            border-top-color: #3498db;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 15px;
        }
        @keyframes spin { to { transform: rotate(360deg); } }

        .content {
            max-width: 900px;
            margin: 0 auto;
            padding: 20px;
        }

        .article {
            background: white;
            border-radius: 12px;
            padding: 40px 50px;
            box-shadow: 0 2px 15px rgba(0,0,0,0.08);
            line-height: 1.8;
            position: relative;
        }
        .article img { max-width: 100%; height: auto; }
        .article h1, .article h2, .article h3 { margin-top: 1.5em; }

        /* AI Annotation Styles */
        .wiki-annotation {
            position: relative;
            display: inline;
        }
        .annotation-icon {
            cursor: pointer;
            font-size: 16px;
            vertical-align: super;
            margin-right: 2px;
        }
        .annotation-bubble {
            display: none;
            position: absolute;
            left: 0;
            top: 100%;
            background: #2c3e50;
            color: white;
            padding: 12px 16px;
            border-radius: 8px;
            font-family: 'Caveat', cursive;
            font-size: 18px;
            line-height: 1.4;
            width: 280px;
            z-index: 100;
            box-shadow: 0 4px 15px rgba(0,0,0,0.2);
        }
        .wiki-annotation:hover .annotation-bubble {
            display: block;
        }
        .annotation-danger .annotation-bubble { background: #c0392b; }
        .annotation-warning .annotation-bubble { background: #d35400; }
        .annotation-info .annotation-bubble { background: #2980b9; }

        /* Highlighted paragraphs */
        .para-contentious {
            background: linear-gradient(90deg, rgba(231,76,60,0.1), transparent);
            border-left: 4px solid #e74c3c;
            padding-left: 15px;
            margin-left: -19px;
        }
        .para-warning {
            background: linear-gradient(90deg, rgba(243,156,18,0.1), transparent);
            border-left: 4px solid #f39c12;
            padding-left: 15px;
            margin-left: -19px;
        }

        /* Margin notes */
        .margin-note {
            position: absolute;
            right: -280px;
            width: 250px;
            background: #fffbeb;
            border-left: 3px solid #f39c12;
            padding: 15px;
            font-family: 'Caveat', cursive;
            font-size: 17px;
            line-height: 1.4;
            color: #92400e;
            transform: rotate(-1deg);
        }
        .margin-note.danger {
            background: #fef2f2;
            border-color: #ef4444;
            color: #991b1b;
        }
        .margin-note::before {
            content: '✎ ';
        }

        /* Summary cards */
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 25px;
        }
        .summary-card {
            background: white;
            border-radius: 10px;
            padding: 20px;
            text-align: center;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
        }
        .summary-card .number {
            font-size: 32px;
            font-weight: 700;
            color: #2c3e50;
        }
        .summary-card .label {
            color: #666;
            font-size: 13px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .summary-card.danger .number { color: #e74c3c; }
        .summary-card.warning .number { color: #f39c12; }

        /* Annotations list */
        .annotations-list {
            background: white;
            border-radius: 12px;
            padding: 25px;
            margin-bottom: 25px;
            box-shadow: 0 2px 15px rgba(0,0,0,0.08);
        }
        .annotations-list h2 {
            margin: 0 0 20px;
            font-size: 18px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .annotation-item {
            padding: 15px;
            margin: 10px 0;
            border-radius: 8px;
            border-left: 4px solid #f39c12;
            background: #fffbeb;
        }
        .annotation-item.danger {
            border-color: #e74c3c;
            background: #fef2f2;
        }
        .annotation-item .note {
            font-family: 'Caveat', cursive;
            font-size: 20px;
            color: #333;
            margin-bottom: 8px;
        }
        .annotation-item .meta {
            font-size: 12px;
            color: #666;
        }
        .annotation-item .preview {
            font-size: 13px;
            color: #888;
            margin-top: 8px;
            font-style: italic;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Wikipedia X-Ray</h1>
        <p>AI-powered editorial analysis - see what's really going on behind the text</p>
    </div>

    <div class="search-box">
        <input type="text" id="url-input" placeholder="Paste Wikipedia URL..." />
        <button onclick="analyze()">Analyze</button>
    </div>

    <div id="loading" class="loading" style="display:none;">
        <div class="spinner"></div>
        <div>AI is reading the article and analyzing edit history...</div>
        <div style="font-size:14px;color:#999;margin-top:10px;">This may take 10-20 seconds</div>
    </div>

    <div id="content" class="content" style="display:none;">
        <div id="summary" class="summary"></div>
        <div id="annotations-list" class="annotations-list"></div>
        <div id="article" class="article"></div>
    </div>

    <script>
    async function analyze() {
        const url = document.getElementById('url-input').value.trim();
        if (!url) return;

        document.getElementById('loading').style.display = 'block';
        document.getElementById('content').style.display = 'none';

        try {
            const resp = await fetch('/api/analyze', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({url})
            });
            const data = await resp.json();

            if (data.error) {
                alert('Error: ' + data.error);
                return;
            }

            render(data);
        } catch (err) {
            alert('Error: ' + err.message);
        } finally {
            document.getElementById('loading').style.display = 'none';
        }
    }

    function render(data) {
        // Summary cards
        const annotations = data.annotations || [];
        const highSeverity = annotations.filter(a => a.severity === 'high').length;

        document.getElementById('summary').innerHTML = `
            <div class="summary-card">
                <div class="number">${data.stats?.total_diffs || 0}</div>
                <div class="label">Recent Edits Analyzed</div>
            </div>
            <div class="summary-card ${highSeverity > 0 ? 'danger' : ''}">
                <div class="number">${highSeverity}</div>
                <div class="label">High Contention Areas</div>
            </div>
            <div class="summary-card warning">
                <div class="number">${annotations.length}</div>
                <div class="label">AI Annotations</div>
            </div>
        `;

        // Annotations list
        let listHtml = '<h2>📝 AI Notes</h2>';
        if (annotations.length === 0) {
            listHtml += '<p style="color:#666;">This article appears relatively stable. No major editorial conflicts detected.</p>';
        } else {
            annotations.forEach((ann, i) => {
                const sevClass = ann.severity === 'high' ? 'danger' : '';
                const icon = ann.severity === 'high' ? '⚠️' : '📝';
                listHtml += `
                    <div class="annotation-item ${sevClass}">
                        <div class="note">${icon} "${ann.note}"</div>
                        <div class="meta">${ann.edit_count || 0} edits, ${ann.revert_count || 0} reverts</div>
                        <div class="preview">"${(ann.preview || '').substring(0, 100)}..."</div>
                    </div>
                `;
            });
        }
        document.getElementById('annotations-list').innerHTML = listHtml;

        // Article with annotations
        document.getElementById('article').innerHTML = data.annotated_html || data.html || '';
        document.getElementById('content').style.display = 'block';
    }

    document.getElementById('url-input').addEventListener('keypress', e => {
        if (e.key === 'Enter') analyze();
    });
    </script>
</body>
</html>
'''


@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/analyze', methods=['POST'])
def analyze():
    data = request.json
    url = data.get('url', '')

    # Extract title from URL
    match = re.search(r'wikipedia\.org/wiki/([^#?]+)', url)
    if not match:
        return jsonify({"error": "Invalid Wikipedia URL"})

    title = match.group(1).replace('_', ' ')
    print(f"\n{'='*50}")
    print(f"Analyzing: {title}")
    print('='*50)

    # Get article content
    print("Fetching article content...")
    article = get_article_content(title)
    if "error" in article:
        return jsonify({"error": article["error"]})

    # Get revision diffs
    print("Fetching revision history with diffs...")
    diffs = get_revision_diffs(title, limit=100)
    print(f"Got {len(diffs)} diffs")

    # Analyze paragraph contention
    print("Analyzing paragraph contention...")
    paragraph_stats = analyze_paragraph_contention(article["wikitext"], diffs)
    print(f"Found {len(paragraph_stats)} paragraphs with edits")

    # Generate AI annotations
    print("Generating AI annotations...")
    annotations = generate_ai_annotations(title, paragraph_stats, diffs)
    print(f"Generated {len(annotations)} annotations")

    # Annotate HTML
    annotated_html = annotate_html_with_notes(article["html"], article["wikitext"], annotations)

    return jsonify({
        "title": title,
        "html": article["html"],
        "annotated_html": annotated_html,
        "annotations": annotations,
        "stats": {
            "total_diffs": len(diffs),
            "contentious_paragraphs": len([p for p in paragraph_stats if p["contention_score"] > 0.3]),
        }
    })


if __name__ == '__main__':
    print("Starting Wikipedia X-Ray Agent...")
    print("Open http://localhost:5002")
    app.run(host='0.0.0.0', port=5002, debug=True)
