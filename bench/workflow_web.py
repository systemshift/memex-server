#!/usr/bin/env python3
"""
Workflow Knowledge Web UI - Query tacit knowledge captured by screen_monitor.py

Usage:
    python workflow_web.py
    # Open http://localhost:5003
"""

import json
import os
import requests
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS

app = Flask(__name__)
CORS(app)

MEMEX_API = os.getenv("MEMEX_API", "http://localhost:8080")


def parse_properties(props):
    """Parse properties from string or dict."""
    if isinstance(props, str):
        try:
            return json.loads(props)
        except:
            return {}
    return props if isinstance(props, dict) else {}


def get_node(node_id: str) -> dict:
    """Get a single node by ID."""
    try:
        resp = requests.get(f"{MEMEX_API}/api/nodes/{node_id}", timeout=10)
        if resp.status_code == 200:
            return resp.json()
    except:
        pass
    return {}


def search_nodes(query: str, limit: int = 50) -> list:
    """Search nodes by text."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/query/search",
            params={"q": query, "limit": limit},
            timeout=10
        )
        if resp.status_code == 200:
            node_ids = resp.json().get("nodes", [])
            return [get_node(nid) for nid in node_ids if nid]
    except:
        pass
    return []


def filter_nodes(node_type: str, limit: int = 100) -> list:
    """Get nodes by type."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/query/filter",
            params={"type": node_type, "limit": limit},
            timeout=10
        )
        if resp.status_code == 200:
            node_ids = resp.json().get("nodes", [])
            return [get_node(nid) for nid in node_ids if nid]
    except:
        pass
    return []


def get_all_nodes(limit: int = 200) -> list:
    """Get all nodes."""
    try:
        resp = requests.get(
            f"{MEMEX_API}/api/nodes",
            params={"limit": limit},
            timeout=10
        )
        if resp.status_code == 200:
            node_ids = resp.json().get("nodes", [])
            return [get_node(nid) for nid in node_ids if nid]
    except:
        pass
    return []


HTML_TEMPLATE = '''
<!DOCTYPE html>
<html>
<head>
    <title>Workflow Knowledge</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Inter', sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            min-height: 100vh;
            color: #e4e4e7;
        }
        .header {
            background: rgba(255,255,255,0.05);
            backdrop-filter: blur(10px);
            border-bottom: 1px solid rgba(255,255,255,0.1);
            padding: 20px 40px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        .header h1 {
            font-size: 24px;
            font-weight: 600;
            background: linear-gradient(135deg, #667eea, #764ba2);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .header .subtitle {
            font-size: 13px;
            color: #a1a1aa;
            margin-top: 4px;
        }
        .stats {
            display: flex;
            gap: 25px;
        }
        .stat {
            text-align: center;
        }
        .stat .number {
            font-size: 28px;
            font-weight: 700;
            color: #667eea;
        }
        .stat .label {
            font-size: 11px;
            color: #71717a;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 30px;
        }

        .search-section {
            background: rgba(255,255,255,0.03);
            border: 1px solid rgba(255,255,255,0.1);
            border-radius: 16px;
            padding: 25px;
            margin-bottom: 30px;
        }
        .search-box {
            display: flex;
            gap: 12px;
        }
        .search-box input {
            flex: 1;
            padding: 14px 20px;
            font-size: 16px;
            border: 1px solid rgba(255,255,255,0.15);
            border-radius: 10px;
            background: rgba(0,0,0,0.3);
            color: white;
            outline: none;
        }
        .search-box input:focus {
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102,126,234,0.2);
        }
        .search-box input::placeholder { color: #71717a; }
        .search-box button {
            padding: 14px 28px;
            font-size: 15px;
            font-weight: 600;
            border: none;
            border-radius: 10px;
            background: linear-gradient(135deg, #667eea, #764ba2);
            color: white;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .search-box button:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 20px rgba(102,126,234,0.3);
        }

        .filters {
            display: flex;
            gap: 10px;
            margin-top: 15px;
            flex-wrap: wrap;
        }
        .filter-btn {
            padding: 8px 16px;
            font-size: 13px;
            border: 1px solid rgba(255,255,255,0.15);
            border-radius: 20px;
            background: transparent;
            color: #a1a1aa;
            cursor: pointer;
            transition: all 0.2s;
        }
        .filter-btn:hover, .filter-btn.active {
            background: rgba(102,126,234,0.2);
            border-color: #667eea;
            color: white;
        }

        .results {
            display: grid;
            gap: 16px;
        }

        .card {
            background: rgba(255,255,255,0.03);
            border: 1px solid rgba(255,255,255,0.08);
            border-radius: 12px;
            padding: 20px;
            transition: all 0.2s;
        }
        .card:hover {
            background: rgba(255,255,255,0.05);
            border-color: rgba(255,255,255,0.15);
            transform: translateY(-2px);
        }

        .card.knowledge {
            border-left: 4px solid #667eea;
        }
        .card.task {
            border-left: 4px solid #22c55e;
        }
        .card.app {
            border-left: 4px solid #f59e0b;
        }

        .card-header {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 12px;
        }
        .card-type {
            padding: 4px 10px;
            font-size: 11px;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            border-radius: 6px;
        }
        .card-type.knowledge { background: rgba(102,126,234,0.2); color: #667eea; }
        .card-type.task { background: rgba(34,197,94,0.2); color: #22c55e; }
        .card-type.app { background: rgba(245,158,11,0.2); color: #f59e0b; }
        .card-type.user { background: rgba(236,72,153,0.2); color: #ec4899; }

        .card-title {
            font-size: 16px;
            font-weight: 500;
            color: #fafafa;
            flex: 1;
        }

        .card-meta {
            display: flex;
            gap: 20px;
            font-size: 13px;
            color: #71717a;
            margin-top: 10px;
        }
        .card-meta span {
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .confidence {
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }
        .confidence-bar {
            width: 60px;
            height: 6px;
            background: rgba(255,255,255,0.1);
            border-radius: 3px;
            overflow: hidden;
        }
        .confidence-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 3px;
        }

        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #71717a;
        }
        .empty-state h3 {
            font-size: 18px;
            margin-bottom: 8px;
            color: #a1a1aa;
        }

        .examples {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid rgba(255,255,255,0.1);
        }
        .examples p {
            font-size: 13px;
            color: #71717a;
            margin-bottom: 8px;
        }
        .example-queries {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
        }
        .example-query {
            padding: 6px 12px;
            font-size: 12px;
            background: rgba(102,126,234,0.1);
            border: 1px solid rgba(102,126,234,0.3);
            border-radius: 6px;
            color: #667eea;
            cursor: pointer;
            transition: all 0.2s;
        }
        .example-query:hover {
            background: rgba(102,126,234,0.2);
        }
    </style>
</head>
<body>
    <div class="header">
        <div>
            <h1>Workflow Knowledge</h1>
            <div class="subtitle">Tacit knowledge captured from screen activity</div>
        </div>
        <div class="stats" id="stats">
            <div class="stat">
                <div class="number" id="stat-insights">-</div>
                <div class="label">Insights</div>
            </div>
            <div class="stat">
                <div class="number" id="stat-tasks">-</div>
                <div class="label">Tasks</div>
            </div>
            <div class="stat">
                <div class="number" id="stat-apps">-</div>
                <div class="label">Apps</div>
            </div>
        </div>
    </div>

    <div class="container">
        <div class="search-section">
            <div class="search-box">
                <input type="text" id="search-input" placeholder="Ask a question... (e.g., 'how to deploy', 'debugging tips')">
                <button onclick="search()">Search</button>
            </div>
            <div class="filters">
                <button class="filter-btn active" data-type="all" onclick="filterType('all', this)">All</button>
                <button class="filter-btn" data-type="Knowledge" onclick="filterType('Knowledge', this)">Knowledge</button>
                <button class="filter-btn" data-type="Task" onclick="filterType('Task', this)">Tasks</button>
                <button class="filter-btn" data-type="Application" onclick="filterType('Application', this)">Apps</button>
                <button class="filter-btn" data-type="User" onclick="filterType('User', this)">Users</button>
            </div>
        </div>

        <div class="results" id="results">
            <div class="empty-state">
                <h3>Search for workflow knowledge</h3>
                <p>Or browse by category using the filters above</p>
                <div class="examples">
                    <p>Try searching for:</p>
                    <div class="example-queries">
                        <span class="example-query" onclick="searchFor('debugging')">debugging</span>
                        <span class="example-query" onclick="searchFor('deploy')">deploy</span>
                        <span class="example-query" onclick="searchFor('config')">config</span>
                        <span class="example-query" onclick="searchFor('error')">error</span>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
    let currentFilter = 'all';
    let allResults = [];

    async function loadStats() {
        try {
            const resp = await fetch('/api/stats');
            const stats = await resp.json();
            document.getElementById('stat-insights').textContent = stats.knowledge || 0;
            document.getElementById('stat-tasks').textContent = stats.tasks || 0;
            document.getElementById('stat-apps').textContent = stats.apps || 0;
        } catch (e) {
            console.error('Failed to load stats:', e);
        }
    }

    async function search() {
        const query = document.getElementById('search-input').value.trim();
        if (!query) {
            filterType('all', document.querySelector('.filter-btn'));
            return;
        }

        try {
            const resp = await fetch('/api/search?q=' + encodeURIComponent(query));
            allResults = await resp.json();
            renderResults(allResults);
        } catch (e) {
            console.error('Search failed:', e);
        }
    }

    function searchFor(query) {
        document.getElementById('search-input').value = query;
        search();
    }

    async function filterType(type, btn) {
        // Update active button
        document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        currentFilter = type;

        // Clear search
        document.getElementById('search-input').value = '';

        try {
            const resp = await fetch('/api/filter?type=' + encodeURIComponent(type));
            allResults = await resp.json();
            renderResults(allResults);
        } catch (e) {
            console.error('Filter failed:', e);
        }
    }

    function renderResults(nodes) {
        const container = document.getElementById('results');

        if (!nodes || nodes.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No results found</h3>
                    <p>Try a different search or browse by category</p>
                </div>
            `;
            return;
        }

        container.innerHTML = nodes.map(node => {
            const type = node.Type || 'Unknown';
            const meta = node.Meta || {};
            const typeClass = type.toLowerCase();

            let title = '';
            let metaHtml = '';

            if (type === 'Knowledge') {
                title = meta.insight || node.ID;
                metaHtml = `
                    <span>App: ${meta.source_app || 'Unknown'}</span>
                    <span>By: ${meta.discovered_by || 'Unknown'}</span>
                    <span class="confidence">
                        Confidence:
                        <span class="confidence-bar">
                            <span class="confidence-fill" style="width: ${(meta.confidence || 0) * 100}%"></span>
                        </span>
                        ${Math.round((meta.confidence || 0) * 100)}%
                    </span>
                `;
            } else if (type === 'Task') {
                title = meta.description || node.ID;
                metaHtml = `
                    <span>Step: ${meta.workflow_step || 'Unknown'}</span>
                `;
            } else if (type === 'Application') {
                title = meta.name || node.ID;
            } else if (type === 'User') {
                title = meta.name || node.ID;
            } else {
                title = node.ID;
            }

            return `
                <div class="card ${typeClass}">
                    <div class="card-header">
                        <span class="card-type ${typeClass}">${type}</span>
                        <span class="card-title">${escapeHtml(title)}</span>
                    </div>
                    ${metaHtml ? `<div class="card-meta">${metaHtml}</div>` : ''}
                </div>
            `;
        }).join('');
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Event listeners
    document.getElementById('search-input').addEventListener('keypress', e => {
        if (e.key === 'Enter') search();
    });

    // Initial load
    loadStats();
    </script>
</body>
</html>
'''


@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/stats')
def stats():
    """Get counts by type."""
    knowledge = filter_nodes("Knowledge")
    tasks = filter_nodes("Task")
    apps = filter_nodes("Application")

    return jsonify({
        "knowledge": len(knowledge),
        "tasks": len(tasks),
        "apps": len(apps),
    })


@app.route('/api/search')
def search_api():
    """Search for nodes."""
    query = request.args.get('q', '')
    if not query:
        return jsonify([])

    nodes = search_nodes(query, limit=50)
    # Filter out empty nodes
    nodes = [n for n in nodes if n and n.get('ID')]
    return jsonify(nodes)


@app.route('/api/filter')
def filter_api():
    """Filter nodes by type."""
    node_type = request.args.get('type', 'all')

    if node_type == 'all':
        nodes = get_all_nodes(limit=100)
        # Only show meaningful types
        nodes = [n for n in nodes if n and n.get('Type') in ('Knowledge', 'Task', 'Application', 'User')]
    else:
        nodes = filter_nodes(node_type, limit=100)

    # Filter out empty nodes
    nodes = [n for n in nodes if n and n.get('ID')]
    return jsonify(nodes)


if __name__ == '__main__':
    print("Starting Workflow Knowledge Web UI...")
    print("Open http://localhost:5003")
    app.run(host='0.0.0.0', port=5003, debug=True)
