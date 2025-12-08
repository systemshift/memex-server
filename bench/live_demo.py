#!/usr/bin/env python3
"""
Live Memex Demo - Working Web App

A real working demo that queries the knowledge graph and uses LLM to answer questions.

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
            # No more tool calls - generate answer from context
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


# ============== HTML Template ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Memex Live Demo</title>
    <style>
        :root {
            --bg: #0a0a0a;
            --surface: #141414;
            --border: #2a2a2a;
            --text: #e0e0e0;
            --text-dim: #888;
            --accent: #4a9eff;
            --accent-dim: #2a5a8a;
        }

        * { box-sizing: border-box; margin: 0; padding: 0; }

        body {
            font-family: 'SF Mono', 'Consolas', monospace;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            min-height: 100vh;
        }

        .container {
            max-width: 900px;
            margin: 0 auto;
            padding: 2rem;
        }

        header {
            text-align: center;
            margin-bottom: 3rem;
            padding-bottom: 2rem;
            border-bottom: 1px solid var(--border);
        }

        h1 {
            font-size: 2rem;
            font-weight: 400;
            margin-bottom: 0.5rem;
        }

        .subtitle {
            color: var(--text-dim);
            font-size: 0.9rem;
        }

        .stats {
            display: flex;
            justify-content: center;
            gap: 2rem;
            margin-top: 1rem;
            font-size: 0.8rem;
            color: var(--text-dim);
        }

        .query-section {
            margin-bottom: 2rem;
        }

        .query-box {
            display: flex;
            gap: 1rem;
        }

        input[type="text"] {
            flex: 1;
            padding: 1rem;
            background: var(--surface);
            border: 1px solid var(--border);
            color: var(--text);
            font-family: inherit;
            font-size: 1rem;
            border-radius: 4px;
        }

        input[type="text"]:focus {
            outline: none;
            border-color: var(--accent);
        }

        button {
            padding: 1rem 2rem;
            background: var(--accent);
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-family: inherit;
            font-size: 1rem;
        }

        button:hover {
            background: var(--accent-dim);
        }

        button:disabled {
            background: var(--border);
            cursor: not-allowed;
        }

        .examples {
            margin-top: 1rem;
            display: flex;
            flex-wrap: wrap;
            gap: 0.5rem;
        }

        .example-btn {
            padding: 0.5rem 1rem;
            background: var(--surface);
            border: 1px solid var(--border);
            color: var(--text-dim);
            font-size: 0.8rem;
            border-radius: 4px;
            cursor: pointer;
        }

        .example-btn:hover {
            border-color: var(--accent);
            color: var(--text);
        }

        .results {
            display: none;
        }

        .results.show {
            display: block;
        }

        .loading {
            text-align: center;
            padding: 3rem;
            color: var(--text-dim);
        }

        .answer-section {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 4px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }

        .answer-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid var(--border);
        }

        .answer-meta {
            font-size: 0.8rem;
            color: var(--text-dim);
        }

        .answer-content {
            white-space: pre-wrap;
            line-height: 1.8;
        }

        .sources-section h3 {
            margin-bottom: 1rem;
            font-weight: 400;
            color: var(--text-dim);
        }

        .source-card {
            background: var(--surface);
            border: 1px solid var(--border);
            border-radius: 4px;
            padding: 1rem;
            margin-bottom: 1rem;
        }

        .source-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 0.5rem;
        }

        .source-type {
            color: var(--accent);
            font-size: 0.8rem;
            text-transform: uppercase;
        }

        .source-meta {
            font-size: 0.75rem;
            color: var(--text-dim);
        }

        .source-content {
            font-size: 0.9rem;
            color: var(--text-dim);
            max-height: 150px;
            overflow: hidden;
            position: relative;
        }

        .source-content::after {
            content: '';
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            height: 50px;
            background: linear-gradient(transparent, var(--surface));
        }

        .exploration {
            margin-top: 1.5rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }

        .exploration h4 {
            font-weight: 400;
            color: var(--text-dim);
            margin-bottom: 0.5rem;
            cursor: pointer;
        }

        .exploration-log {
            display: none;
            font-size: 0.8rem;
            color: var(--text-dim);
            font-family: monospace;
        }

        .exploration-log.show {
            display: block;
        }

        .exploration-step {
            padding: 0.25rem 0;
            border-left: 2px solid var(--border);
            padding-left: 1rem;
            margin-left: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>memex <span style="color: var(--text-dim)">// live demo</span></h1>
            <p class="subtitle">Query the knowledge graph of Nexus Technologies</p>
            <div class="stats">
                <span>640 nodes</span>
                <span>1,491 edges</span>
                <span>72 documents</span>
            </div>
        </header>

        <div class="query-section">
            <div class="query-box">
                <input type="text" id="query" placeholder="Ask anything about Nexus Technologies..." />
                <button id="submit" onclick="runQuery()">Query</button>
            </div>
            <div class="examples">
                <button class="example-btn" onclick="setQuery('What is the Acme Corp deal about?')">Acme deal</button>
                <button class="example-btn" onclick="setQuery('Who is working on Project Phoenix?')">Project Phoenix</button>
                <button class="example-btn" onclick="setQuery('What are the Q4 budget priorities?')">Q4 budget</button>
                <button class="example-btn" onclick="setQuery('Tell me about the Series A fundraising')">Series A</button>
                <button class="example-btn" onclick="setQuery('What security compliance work is happening?')">Security compliance</button>
            </div>
        </div>

        <div id="loading" class="loading" style="display: none;">
            <p>Exploring knowledge graph...</p>
        </div>

        <div id="results" class="results">
            <div class="answer-section">
                <div class="answer-header">
                    <strong>Answer</strong>
                    <span class="answer-meta" id="meta"></span>
                </div>
                <div class="answer-content" id="answer"></div>
            </div>

            <div class="sources-section">
                <h3>Sources</h3>
                <div id="sources"></div>
            </div>

            <div class="exploration">
                <h4 onclick="toggleExploration()">+ Exploration log</h4>
                <div id="exploration-log" class="exploration-log"></div>
            </div>
        </div>
    </div>

    <script>
        function setQuery(q) {
            document.getElementById('query').value = q;
        }

        function toggleExploration() {
            const log = document.getElementById('exploration-log');
            log.classList.toggle('show');
        }

        async function runQuery() {
            const query = document.getElementById('query').value;
            if (!query) return;

            document.getElementById('loading').style.display = 'block';
            document.getElementById('results').classList.remove('show');
            document.getElementById('submit').disabled = true;

            try {
                const resp = await fetch('/api/query', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({query})
                });
                const data = await resp.json();

                document.getElementById('answer').textContent = data.answer;
                document.getElementById('meta').textContent = `${data.hops} hops | ${data.time}s | ${data.sources.length} sources`;

                const sourcesHtml = data.sources.map(s => `
                    <div class="source-card">
                        <div class="source-header">
                            <span class="source-type">${s.doc_type}</span>
                            <span class="source-meta">${Object.entries(s.meta || {}).slice(0, 3).map(([k,v]) => v).join(' | ')}</span>
                        </div>
                        <div class="source-content">${s.content || 'No content'}</div>
                    </div>
                `).join('');
                document.getElementById('sources').innerHTML = sourcesHtml;

                const explorationHtml = data.exploration.map((step, i) => `
                    <div class="exploration-step">${i+1}. ${step.tool}(${JSON.stringify(step.args)})</div>
                `).join('');
                document.getElementById('exploration-log').innerHTML = explorationHtml;

                document.getElementById('results').classList.add('show');
            } catch (e) {
                alert('Error: ' + e.message);
            }

            document.getElementById('loading').style.display = 'none';
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
