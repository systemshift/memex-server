#!/usr/bin/env python3
"""
Live Memex Demo - Working Web App

A real working demo that queries the knowledge graph and uses LLM to answer questions.
Styled to match memex.systems/demo.html

Usage:
    python live_demo.py
    # Open http://localhost:5001
"""

import json
import os
import re
import time
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS
from neo4j import GraphDatabase
from openai import OpenAI
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# Demo Neo4j connection
NEO4J_URI = "bolt://localhost:7688"
NEO4J_USER = "neo4j"
NEO4J_PASSWORD = "demopass"

driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
llm_client = OpenAI()

MODEL = "gpt-4o-mini"
MAX_ITERATIONS = 6


# ============== Graph Tools ==============

def search_entities(query: str, limit: int = 15) -> list[dict]:
    """Search for entities by name."""
    with driver.session() as s:
        result = s.run("""
            MATCH (n:Node)
            WHERE n.type <> 'Source'
            AND toLower(n.properties) CONTAINS toLower($q)
            RETURN n.id as id, n.type as type, n.properties as props
            LIMIT $lim
        """, q=query, lim=limit)
        entities = []
        for r in result:
            props = json.loads(r["props"]) if r["props"] else {}
            entities.append({
                "id": r["id"],
                "type": r["type"],
                "name": props.get("name", r["id"].split(":")[-1].replace("-", " ").title())
            })
        return entities


def get_relationships(entity_id: str) -> list[dict]:
    """Get relationships for an entity."""
    with driver.session() as s:
        result = s.run("""
            MATCH (e:Node {id: $id})-[r:LINK]-(other:Node)
            WHERE other.type <> 'Source'
            RETURN other.id as id, other.type as type, r.type as rel_type, other.properties as props
            LIMIT 20
        """, id=entity_id)
        rels = []
        for r in result:
            props = json.loads(r["props"]) if r["props"] else {}
            rels.append({
                "entity_id": r["id"],
                "entity_type": r["type"],
                "relationship": r["rel_type"],
                "name": props.get("name", r["id"].split(":")[-1].replace("-", " ").title())
            })
        return rels


def get_sources(entity_id: str) -> list[dict]:
    """Get source documents linked to an entity."""
    with driver.session() as s:
        result = s.run("""
            MATCH (e:Node {id: $id})-[:LINK {type: 'EXTRACTED_FROM'}]->(s:Node {type: 'Source'})
            RETURN s.id as id, s.properties as props
            LIMIT 10
        """, id=entity_id)
        sources = []
        for r in result:
            props = json.loads(r["props"]) if r["props"] else {}
            sources.append({
                "id": r["id"],
                "doc_type": props.get("doc_type", "Document"),
                "content": props.get("content", "")[:500],
                "meta": {k: v for k, v in props.items() if k not in ("content", "doc_type")}
            })
        return sources


def read_source(source_id: str) -> dict:
    """Read full source content."""
    with driver.session() as s:
        result = s.run("""
            MATCH (s:Node {id: $id, type: 'Source'})
            RETURN s.properties as props
        """, id=source_id)
        record = result.single()
        if record:
            props = json.loads(record["props"]) if record["props"] else {}
            return {
                "id": source_id,
                "doc_type": props.get("doc_type", "Document"),
                "content": props.get("content", ""),
                "meta": {k: v for k, v in props.items() if k not in ("content", "doc_type")}
            }
        return None


TOOLS_SCHEMA = [
    {
        "type": "function",
        "function": {
            "name": "search_entities",
            "description": "Search for entities (people, companies, projects, concepts) by name",
            "parameters": {
                "type": "object",
                "properties": {"query": {"type": "string", "description": "Search term"}},
                "required": ["query"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "get_relationships",
            "description": "Get entities related to a specific entity",
            "parameters": {
                "type": "object",
                "properties": {"entity_id": {"type": "string", "description": "Entity ID"}},
                "required": ["entity_id"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "get_sources",
            "description": "Get source documents for an entity",
            "parameters": {
                "type": "object",
                "properties": {"entity_id": {"type": "string", "description": "Entity ID"}},
                "required": ["entity_id"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "read_source",
            "description": "Read full content of a source document",
            "parameters": {
                "type": "object",
                "properties": {"source_id": {"type": "string", "description": "Source ID"}},
                "required": ["source_id"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "submit_answer",
            "description": "Submit final answer with supporting sources",
            "parameters": {
                "type": "object",
                "properties": {
                    "answer": {"type": "string", "description": "The answer to the question"},
                    "sources": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Source IDs used"
                    }
                },
                "required": ["answer", "sources"]
            }
        }
    }
]


def agent_query(question: str) -> dict:
    """Run agent to answer question from knowledge graph."""
    start_time = time.time()

    messages = [
        {
            "role": "system",
            "content": """You are a research agent exploring a business knowledge graph for Nexus Technologies.

The graph contains: emails, Slack messages, documents, calendar events, invoices, and purchase orders.
Entities include: people, companies, projects, concepts, technologies, amounts, dates.

Strategy:
1. Search for key entities mentioned in the question
2. Explore relationships to find connected information
3. Read relevant source documents for details
4. Submit a comprehensive answer with sources

Be thorough but efficient. Read sources before answering to get accurate details."""
        },
        {"role": "user", "content": question}
    ]

    collected_sources = []
    exploration_log = []

    for iteration in range(MAX_ITERATIONS):
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=messages,
            tools=TOOLS_SCHEMA,
            tool_choice="auto"
        )

        msg = response.choices[0].message

        if not msg.tool_calls:
            break

        messages.append(msg)

        for tool_call in msg.tool_calls:
            fn_name = tool_call.function.name
            fn_args = json.loads(tool_call.function.arguments)

            exploration_log.append({"tool": fn_name, "args": fn_args})

            if fn_name == "search_entities":
                result = search_entities(fn_args["query"])
            elif fn_name == "get_relationships":
                result = get_relationships(fn_args["entity_id"])
            elif fn_name == "get_sources":
                result = get_sources(fn_args["entity_id"])
                for src in result:
                    if src["id"] not in [s["id"] for s in collected_sources]:
                        collected_sources.append(src)
            elif fn_name == "read_source":
                result = read_source(fn_args["source_id"])
                if result and result["id"] not in [s["id"] for s in collected_sources]:
                    collected_sources.append(result)
            elif fn_name == "submit_answer":
                elapsed = time.time() - start_time
                return {
                    "answer": fn_args["answer"],
                    "sources": collected_sources,
                    "hops": len(exploration_log),
                    "time": round(elapsed, 2),
                    "exploration": exploration_log
                }
            else:
                result = {"error": f"Unknown function: {fn_name}"}

            messages.append({
                "role": "tool",
                "tool_call_id": tool_call.id,
                "content": json.dumps(result)
            })

    # If we didn't get a submit_answer, ask for final answer
    messages.append({
        "role": "user",
        "content": "Based on what you found, provide your final answer. What is the answer to the original question?"
    })

    response = llm_client.chat.completions.create(
        model=MODEL,
        messages=messages
    )

    elapsed = time.time() - start_time
    return {
        "answer": response.choices[0].message.content,
        "sources": collected_sources,
        "hops": len(exploration_log),
        "time": round(elapsed, 2),
        "exploration": exploration_log
    }


# ============== HTML Template (matching memex.systems style) ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Memex Live Demo - Nexus Technologies</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;600&family=Space+Grotesk:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #0a0a0a;
            --bg-secondary: #111111;
            --text: #e0e0e0;
            --text-dim: #707070;
            --accent: #00ff88;
            --accent-dim: #00aa5a;
            --border: #222;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Space Grotesk', sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            min-height: 100vh;
        }

        .mono { font-family: 'JetBrains Mono', monospace; }

        header {
            border-bottom: 1px solid var(--border);
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .logo {
            font-family: 'JetBrains Mono', monospace;
            font-weight: 600;
            font-size: 1.2rem;
            color: var(--accent);
            text-decoration: none;
        }
        .logo span { color: var(--text-dim); }
        .logo .live { color: #ff4444; }

        .stats-bar {
            display: flex;
            gap: 2rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
        }
        .stats-bar span { color: var(--accent); }

        .container {
            max-width: 1100px;
            margin: 0 auto;
            padding: 3rem 2rem;
        }

        h1 {
            font-size: 2rem;
            font-weight: 500;
            margin-bottom: 0.5rem;
        }
        h1 .highlight { color: var(--accent); }

        .subtitle {
            color: var(--text-dim);
            font-size: 1.1rem;
            margin-bottom: 2rem;
        }

        .query-section {
            margin-bottom: 2rem;
        }

        .section-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.1em;
            margin-bottom: 1rem;
        }

        .query-input-box {
            display: flex;
            gap: 1rem;
            margin-bottom: 1rem;
        }

        .query-input-box input {
            flex: 1;
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            padding: 0.75rem 1rem;
            color: var(--text);
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9rem;
            border-radius: 4px;
        }
        .query-input-box input:focus {
            outline: none;
            border-color: var(--accent);
        }

        .query-input-box button {
            background: var(--accent);
            color: #000;
            border: none;
            padding: 0.75rem 1.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9rem;
            font-weight: 500;
            border-radius: 4px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .query-input-box button:hover { background: var(--accent-dim); color: #fff; }
        .query-input-box button:disabled { background: var(--border); cursor: not-allowed; }

        .query-buttons {
            display: flex;
            flex-wrap: wrap;
            gap: 0.75rem;
        }

        .query-btn {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem 0.75rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            border-radius: 4px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .query-btn:hover {
            border-color: var(--accent);
            color: var(--accent);
        }

        .sources-showcase {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 1rem;
            margin-bottom: 2rem;
        }

        .source-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 1rem;
            transition: border-color 0.2s;
        }
        .source-card:hover { border-color: var(--accent-dim); }

        .source-card-header {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 0.75rem;
        }

        .source-icon { font-size: 1rem; }

        .source-type {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.1em;
        }

        .source-card-channel {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--accent);
            margin-bottom: 0.5rem;
        }

        .source-card-content p {
            font-size: 0.85rem;
            color: var(--text-dim);
            line-height: 1.5;
            margin: 0;
            max-height: 80px;
            overflow: hidden;
        }

        .source-card-meta {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--text-dim);
            opacity: 0.7;
            margin-top: 0.75rem;
        }

        .flow-arrow {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 0.25rem;
            margin: 1.5rem 0;
            color: var(--accent);
            font-size: 1.25rem;
        }

        .flow-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.1em;
        }

        .query-panel {
            background: var(--bg-secondary);
            border: 1px solid var(--accent-dim);
            border-radius: 6px;
            overflow: hidden;
        }

        .query-header {
            padding: 0.75rem 1rem;
            border-bottom: 1px solid var(--border);
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            color: var(--accent);
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .query-result { padding: 1rem; }

        .result-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            margin-bottom: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .compiled-answer {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            padding: 1.25rem;
        }

        .compiled-answer p {
            margin: 0;
            line-height: 1.8;
            color: var(--text);
            font-size: 0.95rem;
            white-space: pre-wrap;
        }

        .result-stats {
            margin-top: 1rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            display: flex;
            gap: 1.5rem;
        }
        .stat-item span { color: var(--accent); }

        .exploration-toggle {
            margin-top: 1.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            color: var(--text-dim);
            cursor: pointer;
        }
        .exploration-toggle:hover { color: var(--accent); }

        .exploration-log {
            display: none;
            margin-top: 1rem;
            padding: 1rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 4px;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
        }
        .exploration-log.show { display: block; }

        .exploration-step {
            padding: 0.25rem 0;
            color: var(--text-dim);
            border-left: 2px solid var(--border);
            padding-left: 1rem;
            margin-bottom: 0.5rem;
        }

        .loading {
            text-align: center;
            padding: 3rem;
            color: var(--text-dim);
            font-family: 'JetBrains Mono', monospace;
        }

        .empty-state {
            text-align: center;
            padding: 3rem;
            color: var(--text-dim);
        }

        @media (max-width: 768px) {
            .sources-showcase { grid-template-columns: 1fr; }
            .query-buttons { flex-direction: column; }
            .stats-bar { display: none; }
        }
    </style>
</head>
<body>
    <header>
        <a href="https://memex.systems" class="logo">memex<span>.live</span> <span class="live">&bull;</span></a>
        <div class="stats-bar">
            <div><span>640</span> entities</div>
            <div><span>1,491</span> relationships</div>
            <div><span>72</span> documents</div>
        </div>
    </header>

    <div class="container">
        <h1><span class="highlight">Nexus Technologies</span> Knowledge Graph</h1>
        <p class="subtitle">
            Live queries against the knowledge graph. Type any question or click a suggestion.
        </p>

        <div class="query-section">
            <div class="section-label">Ask anything</div>
            <div class="query-input-box">
                <input type="text" id="query" placeholder="What do you want to know about Nexus Technologies?" />
                <button id="submit" onclick="runQuery()">Query</button>
            </div>
            <div class="query-buttons">
                <button class="query-btn" onclick="setQuery('What is the Acme Corp deal about?')">Acme deal</button>
                <button class="query-btn" onclick="setQuery('Who is working on Project Phoenix?')">Project Phoenix</button>
                <button class="query-btn" onclick="setQuery('What is the Series A fundraising status?')">Series A</button>
                <button class="query-btn" onclick="setQuery('What is the SOC2 compliance timeline?')">SOC2 timeline</button>
                <button class="query-btn" onclick="setQuery('What are the Q4 budget concerns?')">Q4 budget</button>
            </div>
        </div>

        <div id="results-container">
            <div class="empty-state">
                <p>Enter a query above to see live results from the knowledge graph</p>
            </div>
        </div>
    </div>

    <script>
        function setQuery(q) {
            document.getElementById('query').value = q;
            runQuery();
        }

        function toggleExploration() {
            const log = document.getElementById('exploration-log');
            log.classList.toggle('show');
        }

        function getIcon(type) {
            const icons = {
                'email': '&#x2709;',
                'slack': '&#x0023;',
                'document': '&#x1F4C4;',
                'calendar': '&#x1F4C5;',
                'invoice': '&#x1F4B0;',
                'purchaseorder': '&#x1F4DD;'
            };
            return icons[type.toLowerCase()] || '&#x1F4C1;';
        }

        async function runQuery() {
            const query = document.getElementById('query').value;
            if (!query) return;

            const container = document.getElementById('results-container');
            container.innerHTML = '<div class="loading">Exploring knowledge graph...</div>';
            document.getElementById('submit').disabled = true;

            try {
                const resp = await fetch('/api/query', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({query})
                });
                const data = await resp.json();

                const sourcesHtml = data.sources.slice(0, 4).map(s => {
                    const title = s.meta?.title || s.meta?.subject || s.meta?.channel || s.doc_type;
                    const date = s.meta?.date || '';
                    return `
                        <div class="source-card">
                            <div class="source-card-header">
                                <span class="source-icon">${getIcon(s.doc_type)}</span>
                                <span class="source-type">${s.doc_type}</span>
                            </div>
                            <div class="source-card-content">
                                <div class="source-card-channel">${title}</div>
                                <p>${(s.content || '').substring(0, 200)}...</p>
                            </div>
                            <div class="source-card-meta">${date}</div>
                        </div>
                    `;
                }).join('');

                const explorationHtml = data.exploration.map((step, i) => `
                    <div class="exploration-step">${i+1}. ${step.tool}(${JSON.stringify(step.args)})</div>
                `).join('');

                container.innerHTML = `
                    <div class="query-panel">
                        <div class="query-header">
                            <span>memex</span> live query
                        </div>
                        <div class="query-result">
                            <div class="result-label">Compiled context from ${data.sources.length} sources</div>
                            <div class="compiled-answer">
                                <p>${data.answer}</p>
                            </div>
                            <div class="result-stats">
                                <div class="stat-item"><span>${data.sources.length}</span> sources</div>
                                <div class="stat-item"><span>${data.hops}</span> graph hops</div>
                                <div class="stat-item"><span>${data.time}s</span></div>
                            </div>
                        </div>
                    </div>

                    <div class="flow-arrow">
                        <span>&uarr;</span>
                        <span class="flow-label">compiled from sources</span>
                        <span>&uarr;</span>
                    </div>

                    <div class="sources-showcase">${sourcesHtml}</div>

                    <div class="exploration-toggle" onclick="toggleExploration()">+ Show exploration log</div>
                    <div id="exploration-log" class="exploration-log">${explorationHtml}</div>
                `;

            } catch (e) {
                container.innerHTML = '<div class="empty-state"><p>Error: ' + e.message + '</p></div>';
            }

            document.getElementById('submit').disabled = false;
        }

        document.getElementById('query').addEventListener('keypress', e => {
            if (e.key === 'Enter') runQuery();
        });
    </script>
</body>
</html>
"""


# ============== Routes ==============

@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/query', methods=['POST'])
def api_query():
    data = request.json
    query = data.get('query', '')

    if not query:
        return jsonify({"error": "No query provided"}), 400

    result = agent_query(query)
    return jsonify(result)


@app.route('/api/stats')
def api_stats():
    with driver.session() as s:
        nodes = s.run("MATCH (n) RETURN count(n) as c").single()["c"]
        edges = s.run("MATCH ()-[r]->() RETURN count(r) as c").single()["c"]
        sources = s.run("MATCH (n:Node {type: 'Source'}) RETURN count(n) as c").single()["c"]
    return jsonify({"nodes": nodes, "edges": edges, "sources": sources})


if __name__ == '__main__':
    print("Starting Memex Live Demo...")
    print("Open http://localhost:5001")
    app.run(host='0.0.0.0', port=5001, debug=True)
