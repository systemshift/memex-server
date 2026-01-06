#!/usr/bin/env python3
"""
Memex Workspace - Unified Interface

A single workspace that adapts to what you're doing. No more switching between
Docs, Sheets, Slack, Forms, Jira - just describe what you need.

This is the v1 product, not a demo. It's designed to be:
- Self-explanatory on first use
- Immediately useful
- Connected to organizational memory

Usage:
    python workspace.py
    # Open http://localhost:5001
"""

import json
import os
import uuid
import copy
from datetime import datetime
from typing import Dict, Any, List, Optional
from enum import Enum

import requests
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS
from openai import OpenAI
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

llm_client = OpenAI()
MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")


# ============== Intent Types ==============

class IntentType(str, Enum):
    WORKFLOW = "workflow"      # Multi-step process (expense, hire, approval)
    DOCUMENT = "document"      # Creating/editing text content
    QUERY = "query"            # Asking a question, searching
    TABLE = "table"            # Structured data view
    MESSAGE = "message"        # Communication to person/team
    UNKNOWN = "unknown"


# ============== State Management ==============

workspaces: Dict[str, Dict] = {}


def get_workspace(workspace_id: str) -> Dict:
    """Get or create a workspace"""
    if workspace_id not in workspaces:
        workspaces[workspace_id] = {
            "id": workspace_id,
            "created": datetime.now().isoformat(),
            "items": [],  # List of workspace items (workflows, docs, queries)
            "active_item": None,
            "history": []  # All inputs for context
        }
    return workspaces[workspace_id]


# ============== Memex Integration ==============

def memex_get(path: str) -> Optional[Dict]:
    try:
        resp = requests.get(f"{MEMEX_URL}{path}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except:
        return None


def memex_post(path: str, data: Dict) -> Optional[Dict]:
    try:
        resp = requests.post(f"{MEMEX_URL}{path}", json=data, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except:
        return None


def search_memex(query: str, limit: int = 10) -> List[Dict]:
    """Search memex for relevant context"""
    result = memex_get(f"/api/query/search?q={query}&limit={limit}")
    if result and result.get("nodes"):
        return result["nodes"]
    return []


def save_to_memex(item_type: str, content: Dict) -> Optional[str]:
    """Save an item to memex"""
    item_id = f"{item_type}:{uuid.uuid4().hex[:12]}"
    resp = memex_post("/api/nodes", {
        "id": item_id,
        "type": item_type.title(),
        "meta": {
            **content,
            "created": datetime.now().isoformat()
        }
    })
    return item_id if resp else None


# ============== Intent Classification ==============

def classify_intent(user_input: str, context: List[Dict]) -> Dict:
    """Classify what the user is trying to do"""

    context_str = ""
    if context:
        context_str = "\nRecent context:\n" + "\n".join([f"- {c.get('content', '')[:100]}" for c in context[:5]])

    prompt = f"""Classify this user input into one of these intent types:

1. WORKFLOW - Multi-step process like expense reports, hiring requests, approvals, onboarding
2. DOCUMENT - Creating or editing text content, drafting messages, writing proposals
3. QUERY - Asking questions, searching for information, "what is", "who is", "show me"
4. TABLE - Viewing structured data, lists, comparisons, "list all", "show table"
5. MESSAGE - Direct communication to a person or team, notifications

User input: "{user_input}"
{context_str}

Return JSON:
{{
    "intent": "workflow|document|query|table|message",
    "confidence": 0.0-1.0,
    "title": "short title for this item",
    "summary": "what the user wants to do",
    "entities": ["extracted", "entity", "names"],
    "suggested_action": "what should happen next"
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        return json.loads(response.choices[0].message.content)
    except:
        return {"intent": "unknown", "confidence": 0, "title": "Unknown", "summary": user_input}


# ============== Intent Handlers ==============

def handle_workflow(user_input: str, context: List[Dict]) -> Dict:
    """Handle workflow intent - returns form/process state"""

    context_str = ""
    if context:
        context_str = "Context from organization:\n" + "\n".join([
            f"- {c.get('Type', 'Item')}: {c.get('Meta', {}).get('title', c.get('ID', ''))}"
            for c in context[:5]
        ])

    prompt = f"""Create a workflow for this request.

User request: "{user_input}"

{context_str}

Return JSON with the workflow state:
{{
    "title": "Workflow title",
    "description": "What this workflow accomplishes",
    "fields": {{
        "field_name": {{
            "label": "Display Label",
            "type": "text|currency|date|select|textarea|email|file",
            "value": null,
            "done": false,
            "hint": "Help text",
            "required": true,
            "options": ["only", "for", "select"]
        }}
    }},
    "context_cards": [
        {{"title": "Relevant Policy", "content": "Helpful info from org memory"}}
    ],
    "next_steps": ["What happens after submission"],
    "complete": false
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        result["type"] = "workflow"
        return result
    except Exception as e:
        return {"type": "workflow", "title": "Workflow", "error": str(e)}


def handle_document(user_input: str, context: List[Dict]) -> Dict:
    """Handle document intent - returns editable content"""

    prompt = f"""Create a document based on this request.

User request: "{user_input}"

Return JSON:
{{
    "title": "Document title",
    "content": "The actual document content in markdown format",
    "suggestions": ["Suggested improvements or additions"],
    "related_topics": ["Topics to consider adding"]
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}]
        )
        # Parse or use as content
        content = response.choices[0].message.content
        try:
            result = json.loads(content)
        except:
            result = {"title": "Document", "content": content}
        result["type"] = "document"
        return result
    except Exception as e:
        return {"type": "document", "title": "Document", "error": str(e)}


def handle_query(user_input: str, context: List[Dict]) -> Dict:
    """Handle query intent - search and answer"""

    # Search memex for relevant info
    search_results = search_memex(user_input, limit=10)

    results_str = ""
    if search_results:
        results_str = "Information found in organizational memory:\n"
        for node in search_results[:5]:
            meta = node.get("Meta", {})
            results_str += f"- [{node.get('Type')}] {meta.get('title', node.get('ID', 'Item'))}\n"
            if meta.get('content'):
                results_str += f"  {meta.get('content', '')[:200]}\n"

    prompt = f"""Answer this question using the organizational memory.

Question: "{user_input}"

{results_str if results_str else "No specific information found in memory."}

Return JSON:
{{
    "answer": "Direct answer to the question",
    "sources": ["List of sources used"],
    "related_questions": ["Follow-up questions they might have"],
    "confidence": "high|medium|low"
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        result["type"] = "query"
        result["search_results"] = search_results[:5]
        return result
    except Exception as e:
        return {"type": "query", "answer": f"Error: {str(e)}", "search_results": search_results}


def handle_table(user_input: str, context: List[Dict]) -> Dict:
    """Handle table intent - structured data view"""

    # Search memex
    search_results = search_memex(user_input, limit=20)

    prompt = f"""Create a table view for this request.

Request: "{user_input}"

Available data from memory: {len(search_results)} items found

Return JSON:
{{
    "title": "Table title",
    "columns": ["Column1", "Column2", "Column3"],
    "rows": [
        ["data1", "data2", "data3"],
        ["data1", "data2", "data3"]
    ],
    "summary": "Summary of the data",
    "filters_available": ["Possible filters"]
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        result["type"] = "table"
        return result
    except Exception as e:
        return {"type": "table", "title": "Table", "error": str(e)}


def handle_message(user_input: str, context: List[Dict]) -> Dict:
    """Handle message intent - communication"""

    prompt = f"""Draft a message based on this request.

Request: "{user_input}"

Return JSON:
{{
    "to": "Recipient(s)",
    "subject": "Message subject",
    "body": "Message content",
    "tone": "professional|casual|urgent",
    "suggested_recipients": ["Other people who might need to know"]
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        result["type"] = "message"
        return result
    except Exception as e:
        return {"type": "message", "body": user_input, "error": str(e)}


INTENT_HANDLERS = {
    "workflow": handle_workflow,
    "document": handle_document,
    "query": handle_query,
    "table": handle_table,
    "message": handle_message,
}


# ============== Routes ==============

@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/workspace', methods=['POST'])
def create_workspace():
    """Create or get a workspace"""
    data = request.json or {}
    workspace_id = data.get("workspace_id", str(uuid.uuid4().hex[:12]))
    workspace = get_workspace(workspace_id)
    return jsonify({
        "workspace_id": workspace["id"],
        "items": workspace["items"],
        "active_item": workspace["active_item"]
    })


@app.route('/api/input', methods=['POST'])
def handle_input():
    """Main entry point - handle any user input"""
    data = request.json
    workspace_id = data.get("workspace_id", "default")
    user_input = data.get("input", "").strip()

    if not user_input:
        return jsonify({"error": "No input provided"}), 400

    workspace = get_workspace(workspace_id)

    # Add to history
    workspace["history"].append({
        "input": user_input,
        "timestamp": datetime.now().isoformat()
    })

    # Get context from memex
    context = search_memex(user_input, limit=5)

    # Classify intent
    classification = classify_intent(user_input, context)
    intent = classification.get("intent", "unknown")

    # Handle based on intent (normalize to lowercase)
    intent_lower = intent.lower()
    handler = INTENT_HANDLERS.get(intent_lower, handle_query)
    result = handler(user_input, context)

    # Create workspace item
    item_id = str(uuid.uuid4().hex[:8])
    item = {
        "id": item_id,
        "created": datetime.now().isoformat(),
        "input": user_input,
        "classification": classification,
        "result": result,
        "saved_to_memex": False
    }

    workspace["items"].append(item)
    workspace["active_item"] = item_id

    return jsonify({
        "item_id": item_id,
        "classification": classification,
        "result": result,
        "context_found": len(context)
    })


@app.route('/api/update', methods=['POST'])
def update_item():
    """Update a workspace item (e.g., fill form fields)"""
    data = request.json
    workspace_id = data.get("workspace_id", "default")
    item_id = data.get("item_id")
    updates = data.get("updates", {})

    workspace = get_workspace(workspace_id)

    for item in workspace["items"]:
        if item["id"] == item_id:
            # Merge updates into result
            if "fields" in item["result"] and "fields" in updates:
                for field_name, field_value in updates["fields"].items():
                    if field_name in item["result"]["fields"]:
                        item["result"]["fields"][field_name]["value"] = field_value
                        item["result"]["fields"][field_name]["done"] = True

            # Check if complete
            if item["result"].get("type") == "workflow":
                all_done = all(
                    f.get("done") or not f.get("required", True)
                    for f in item["result"].get("fields", {}).values()
                )
                item["result"]["complete"] = all_done

            return jsonify({"status": "updated", "item": item})

    return jsonify({"error": "Item not found"}), 404


@app.route('/api/save', methods=['POST'])
def save_item():
    """Save an item to memex"""
    data = request.json
    workspace_id = data.get("workspace_id", "default")
    item_id = data.get("item_id")

    workspace = get_workspace(workspace_id)

    for item in workspace["items"]:
        if item["id"] == item_id:
            # Save to memex
            item_type = item["result"].get("type", "item")
            memex_id = save_to_memex(item_type, {
                "title": item["result"].get("title", "Untitled"),
                "input": item["input"],
                "result": item["result"]
            })

            item["saved_to_memex"] = True
            item["memex_id"] = memex_id

            return jsonify({
                "status": "saved",
                "memex_id": memex_id
            })

    return jsonify({"error": "Item not found"}), 404


@app.route('/api/recent')
def get_recent():
    """Get recent items from memex"""
    results = search_memex("Workflow OR Document OR Query", limit=10)
    return jsonify({"items": results})


# ============== HTML Template ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html>
<head>
    <title>Workspace</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #fafafa;
            --bg-secondary: #ffffff;
            --text: #1a1a1a;
            --text-dim: #666;
            --border: #e0e0e0;
            --accent: #2563eb;
            --accent-light: #dbeafe;
            --success: #22c55e;
            --warning: #f59e0b;
        }

        @media (prefers-color-scheme: dark) {
            :root {
                --bg: #0a0a0a;
                --bg-secondary: #141414;
                --text: #e0e0e0;
                --text-dim: #888;
                --border: #2a2a2a;
                --accent: #3b82f6;
                --accent-light: #1e3a5f;
            }
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Inter', -apple-system, sans-serif;
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

        /* Header */
        header {
            text-align: center;
            margin-bottom: 2rem;
        }

        header h1 {
            font-size: 1.5rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
        }

        header p {
            color: var(--text-dim);
            font-size: 0.95rem;
        }

        /* Main Input */
        .main-input {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 1rem;
            margin-bottom: 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }

        .input-row {
            display: flex;
            gap: 0.75rem;
        }

        .main-input input {
            flex: 1;
            border: none;
            background: transparent;
            font-size: 1.1rem;
            color: var(--text);
            outline: none;
        }

        .main-input input::placeholder {
            color: var(--text-dim);
        }

        .main-input button {
            background: var(--accent);
            color: white;
            border: none;
            padding: 0.6rem 1.2rem;
            border-radius: 8px;
            font-weight: 500;
            cursor: pointer;
            font-size: 0.95rem;
        }

        .main-input button:hover {
            opacity: 0.9;
        }

        .main-input button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        /* Quick Actions */
        .quick-actions {
            display: flex;
            gap: 0.5rem;
            margin-top: 0.75rem;
            flex-wrap: wrap;
        }

        .quick-action {
            background: var(--bg);
            border: 1px solid var(--border);
            padding: 0.4rem 0.8rem;
            border-radius: 20px;
            font-size: 0.8rem;
            color: var(--text-dim);
            cursor: pointer;
        }

        .quick-action:hover {
            border-color: var(--accent);
            color: var(--accent);
        }

        /* Content Area */
        .content-area {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            min-height: 400px;
            overflow: hidden;
        }

        .content-header {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .content-type {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            background: var(--accent-light);
            color: var(--accent);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
        }

        .content-title {
            font-weight: 500;
            flex: 1;
        }

        .content-body {
            padding: 1.5rem;
        }

        /* Welcome State */
        .welcome {
            text-align: center;
            padding: 3rem;
        }

        .welcome h2 {
            font-size: 1.3rem;
            margin-bottom: 0.5rem;
        }

        .welcome p {
            color: var(--text-dim);
            margin-bottom: 1.5rem;
        }

        .welcome-examples {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 1rem;
            text-align: left;
        }

        .welcome-example {
            background: var(--bg);
            border: 1px solid var(--border);
            padding: 1rem;
            border-radius: 8px;
            cursor: pointer;
        }

        .welcome-example:hover {
            border-color: var(--accent);
        }

        .welcome-example .type {
            font-size: 0.75rem;
            color: var(--accent);
            font-weight: 500;
            margin-bottom: 0.25rem;
        }

        .welcome-example .text {
            font-size: 0.9rem;
        }

        /* Workflow View */
        .workflow-fields {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }

        .field-group {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
        }

        .field-label {
            font-size: 0.85rem;
            font-weight: 500;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .field-label .status {
            width: 16px;
            height: 16px;
            border-radius: 50%;
            border: 2px solid var(--border);
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 0.7rem;
        }

        .field-label .status.done {
            background: var(--success);
            border-color: var(--success);
            color: white;
        }

        .field-input {
            padding: 0.6rem 0.8rem;
            border: 1px solid var(--border);
            border-radius: 6px;
            font-size: 0.95rem;
            background: var(--bg);
            color: var(--text);
        }

        .field-input:focus {
            outline: none;
            border-color: var(--accent);
        }

        .field-hint {
            font-size: 0.8rem;
            color: var(--text-dim);
        }

        /* Context Cards */
        .context-cards {
            margin-top: 1.5rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--border);
        }

        .context-card {
            background: var(--accent-light);
            border-left: 3px solid var(--accent);
            padding: 0.75rem;
            border-radius: 0 6px 6px 0;
            margin-bottom: 0.5rem;
        }

        .context-card-title {
            font-size: 0.8rem;
            font-weight: 500;
            color: var(--accent);
        }

        .context-card-content {
            font-size: 0.85rem;
            margin-top: 0.25rem;
        }

        /* Query View */
        .query-answer {
            font-size: 1.05rem;
            line-height: 1.7;
        }

        .query-sources {
            margin-top: 1rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }

        .query-sources h4 {
            font-size: 0.85rem;
            color: var(--text-dim);
            margin-bottom: 0.5rem;
        }

        /* Document View */
        .document-content {
            font-size: 1rem;
            line-height: 1.8;
            white-space: pre-wrap;
        }

        .document-content textarea {
            width: 100%;
            min-height: 300px;
            border: none;
            background: transparent;
            font-family: inherit;
            font-size: inherit;
            line-height: inherit;
            color: var(--text);
            resize: vertical;
        }

        .document-content textarea:focus {
            outline: none;
        }

        /* Table View */
        .table-view {
            overflow-x: auto;
        }

        .table-view table {
            width: 100%;
            border-collapse: collapse;
        }

        .table-view th, .table-view td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }

        .table-view th {
            font-weight: 500;
            font-size: 0.85rem;
            color: var(--text-dim);
        }

        /* Message View */
        .message-preview {
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
        }

        .message-header {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
            margin-bottom: 1rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--border);
        }

        .message-field {
            display: flex;
            gap: 0.5rem;
            font-size: 0.9rem;
        }

        .message-field-label {
            color: var(--text-dim);
            min-width: 60px;
        }

        .message-body {
            font-size: 0.95rem;
            line-height: 1.7;
            white-space: pre-wrap;
        }

        /* Actions Bar */
        .actions-bar {
            display: flex;
            gap: 0.5rem;
            margin-top: 1.5rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--border);
        }

        .action-btn {
            padding: 0.6rem 1rem;
            border-radius: 6px;
            font-size: 0.9rem;
            font-weight: 500;
            cursor: pointer;
            border: 1px solid var(--border);
            background: var(--bg);
            color: var(--text);
        }

        .action-btn.primary {
            background: var(--accent);
            border-color: var(--accent);
            color: white;
        }

        .action-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        /* Loading */
        .loading {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 3rem;
            color: var(--text-dim);
        }

        /* Sidebar - Recent Activity */
        .sidebar {
            margin-top: 1.5rem;
        }

        .sidebar h3 {
            font-size: 0.9rem;
            color: var(--text-dim);
            margin-bottom: 0.75rem;
        }

        .recent-item {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.85rem;
        }

        .recent-item:hover {
            background: var(--bg-secondary);
        }

        .recent-item .type-badge {
            font-size: 0.7rem;
            padding: 0.15rem 0.4rem;
            border-radius: 4px;
            background: var(--accent-light);
            color: var(--accent);
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Workspace</h1>
            <p>One place for everything. Just describe what you need.</p>
        </header>

        <div class="main-input">
            <div class="input-row">
                <input type="text" id="main-input" placeholder="What do you need?" autofocus
                       onkeypress="if(event.key==='Enter')handleInput()">
                <button onclick="handleInput()" id="go-btn">Go</button>
            </div>
            <div class="quick-actions">
                <span class="quick-action" onclick="setInput('Submit an expense report')">Expense</span>
                <span class="quick-action" onclick="setInput('Draft a project update email')">Document</span>
                <span class="quick-action" onclick="setInput('What projects are we working on?')">Query</span>
                <span class="quick-action" onclick="setInput('List all pending approvals')">Table</span>
                <span class="quick-action" onclick="setInput('Message the team about the deadline')">Message</span>
            </div>
        </div>

        <div class="content-area" id="content-area">
            <div class="welcome">
                <h2>Welcome to your workspace</h2>
                <p>Everything you do here becomes part of your organization's memory.</p>

                <div class="welcome-examples">
                    <div class="welcome-example" onclick="setInput('I need to submit an expense for a client dinner, $247 at Marea')">
                        <div class="type">Workflow</div>
                        <div class="text">Submit an expense for client dinner</div>
                    </div>
                    <div class="welcome-example" onclick="setInput('What expenses did we have last month?')">
                        <div class="type">Query</div>
                        <div class="text">Search organizational memory</div>
                    </div>
                    <div class="welcome-example" onclick="setInput('Draft a proposal for the Q2 marketing campaign')">
                        <div class="type">Document</div>
                        <div class="text">Create a new document</div>
                    </div>
                    <div class="welcome-example" onclick="setInput('Send a message to the engineering team about the launch')">
                        <div class="type">Message</div>
                        <div class="text">Communicate with your team</div>
                    </div>
                </div>
            </div>
        </div>

        <div class="sidebar" id="sidebar"></div>
    </div>

    <script>
        let workspaceId = null;
        let currentItem = null;

        // Initialize
        document.addEventListener('DOMContentLoaded', async () => {
            const resp = await fetch('/api/workspace', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({})
            });
            const data = await resp.json();
            workspaceId = data.workspace_id;
            loadRecent();
        });

        function setInput(text) {
            document.getElementById('main-input').value = text;
            document.getElementById('main-input').focus();
        }

        async function handleInput() {
            const input = document.getElementById('main-input').value.trim();
            if (!input) return;

            const btn = document.getElementById('go-btn');
            const contentArea = document.getElementById('content-area');

            btn.disabled = true;
            btn.textContent = '...';
            contentArea.innerHTML = '<div class="loading">Processing...</div>';

            try {
                const resp = await fetch('/api/input', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ workspace_id: workspaceId, input })
                });
                const data = await resp.json();

                currentItem = data;
                renderResult(data);
                document.getElementById('main-input').value = '';
            } catch (e) {
                contentArea.innerHTML = `<div class="content-body">Error: ${e.message}</div>`;
            } finally {
                btn.disabled = false;
                btn.textContent = 'Go';
            }
        }

        function renderResult(data) {
            const contentArea = document.getElementById('content-area');
            const result = data.result;
            const classification = data.classification;
            const type = result.type || 'unknown';

            let html = `
                <div class="content-header">
                    <span class="content-type">${type.toUpperCase()}</span>
                    <span class="content-title">${result.title || classification.title || 'Result'}</span>
                </div>
                <div class="content-body">
            `;

            switch (type) {
                case 'workflow':
                    html += renderWorkflow(result);
                    break;
                case 'document':
                    html += renderDocument(result);
                    break;
                case 'query':
                    html += renderQuery(result);
                    break;
                case 'table':
                    html += renderTable(result);
                    break;
                case 'message':
                    html += renderMessage(result);
                    break;
                default:
                    html += `<pre>${JSON.stringify(result, null, 2)}</pre>`;
            }

            html += '</div>';
            contentArea.innerHTML = html;
        }

        function renderWorkflow(result) {
            let html = '<div class="workflow-fields">';

            for (const [name, field] of Object.entries(result.fields || {})) {
                const done = field.done;
                html += `
                    <div class="field-group">
                        <label class="field-label">
                            <span class="status ${done ? 'done' : ''}">${done ? '✓' : ''}</span>
                            ${field.label || name}
                            ${field.required ? '*' : ''}
                        </label>
                        ${field.type === 'textarea'
                            ? `<textarea class="field-input" data-field="${name}" placeholder="${field.hint || ''}">${field.value || ''}</textarea>`
                            : field.type === 'select'
                                ? `<select class="field-input" data-field="${name}">
                                    <option value="">Select...</option>
                                    ${(field.options || []).map(o => `<option ${field.value === o ? 'selected' : ''}>${o}</option>`).join('')}
                                   </select>`
                                : `<input class="field-input" type="${field.type === 'currency' ? 'text' : field.type}"
                                    data-field="${name}" value="${field.value || ''}" placeholder="${field.hint || ''}">`
                        }
                        ${field.hint && !done ? `<span class="field-hint">${field.hint}</span>` : ''}
                    </div>
                `;
            }

            html += '</div>';

            // Context cards
            if (result.context_cards && result.context_cards.length > 0) {
                html += '<div class="context-cards">';
                for (const card of result.context_cards) {
                    html += `
                        <div class="context-card">
                            <div class="context-card-title">${card.title}</div>
                            <div class="context-card-content">${card.content}</div>
                        </div>
                    `;
                }
                html += '</div>';
            }

            // Actions
            html += `
                <div class="actions-bar">
                    <button class="action-btn primary" onclick="saveItem()" ${result.complete ? '' : 'disabled'}>
                        Submit
                    </button>
                    <button class="action-btn" onclick="updateFields()">Update</button>
                    <button class="action-btn" onclick="saveItem()">Save Draft</button>
                </div>
            `;

            return html;
        }

        function renderDocument(result) {
            return `
                <div class="document-content">
                    <textarea id="doc-content">${result.content || ''}</textarea>
                </div>
                <div class="actions-bar">
                    <button class="action-btn primary" onclick="saveItem()">Save</button>
                    <button class="action-btn">Share</button>
                </div>
            `;
        }

        function renderQuery(result) {
            let html = `<div class="query-answer">${result.answer || 'No answer found.'}</div>`;

            if (result.sources && result.sources.length > 0) {
                html += '<div class="query-sources"><h4>Sources</h4><ul>';
                for (const source of result.sources) {
                    html += `<li>${source}</li>`;
                }
                html += '</ul></div>';
            }

            if (result.related_questions && result.related_questions.length > 0) {
                html += '<div class="quick-actions" style="margin-top: 1rem;">';
                for (const q of result.related_questions) {
                    html += `<span class="quick-action" onclick="setInput('${q}')">${q}</span>`;
                }
                html += '</div>';
            }

            return html;
        }

        function renderTable(result) {
            if (!result.columns || !result.rows) {
                return `<p>${result.summary || 'No data to display.'}</p>`;
            }

            let html = '<div class="table-view"><table><thead><tr>';
            for (const col of result.columns) {
                html += `<th>${col}</th>`;
            }
            html += '</tr></thead><tbody>';

            for (const row of result.rows) {
                html += '<tr>';
                for (const cell of row) {
                    html += `<td>${cell}</td>`;
                }
                html += '</tr>';
            }

            html += '</tbody></table></div>';

            if (result.summary) {
                html += `<p style="margin-top: 1rem; color: var(--text-dim);">${result.summary}</p>`;
            }

            return html;
        }

        function renderMessage(result) {
            return `
                <div class="message-preview">
                    <div class="message-header">
                        <div class="message-field">
                            <span class="message-field-label">To:</span>
                            <span>${result.to || 'Select recipient...'}</span>
                        </div>
                        <div class="message-field">
                            <span class="message-field-label">Subject:</span>
                            <span>${result.subject || ''}</span>
                        </div>
                    </div>
                    <div class="message-body">${result.body || ''}</div>
                </div>
                <div class="actions-bar">
                    <button class="action-btn primary" onclick="saveItem()">Send</button>
                    <button class="action-btn">Edit</button>
                    <button class="action-btn">Save Draft</button>
                </div>
            `;
        }

        async function updateFields() {
            if (!currentItem) return;

            const fields = {};
            document.querySelectorAll('.field-input').forEach(input => {
                const fieldName = input.dataset.field;
                if (fieldName && input.value) {
                    fields[fieldName] = input.value;
                }
            });

            const resp = await fetch('/api/update', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    workspace_id: workspaceId,
                    item_id: currentItem.item_id,
                    updates: { fields }
                })
            });

            const data = await resp.json();
            if (data.item) {
                currentItem.result = data.item.result;
                renderResult(currentItem);
            }
        }

        async function saveItem() {
            if (!currentItem) return;

            // Update fields first
            await updateFields();

            const resp = await fetch('/api/save', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    workspace_id: workspaceId,
                    item_id: currentItem.item_id
                })
            });

            const data = await resp.json();
            alert(data.memex_id ? `Saved to memory: ${data.memex_id}` : 'Saved locally');
            loadRecent();
        }

        async function loadRecent() {
            try {
                const resp = await fetch('/api/recent');
                const data = await resp.json();

                const sidebar = document.getElementById('sidebar');
                if (data.items && data.items.length > 0) {
                    sidebar.innerHTML = '<h3>Recent from Memory</h3>' +
                        data.items.slice(0, 5).map(item => `
                            <div class="recent-item">
                                <span class="type-badge">${item.Type || 'Item'}</span>
                                <span>${item.Meta?.title || item.ID}</span>
                            </div>
                        `).join('');
                }
            } catch (e) {
                // Memex might not be running
            }
        }
    </script>
</body>
</html>
"""


if __name__ == '__main__':
    print("=" * 50)
    print("WORKSPACE")
    print("=" * 50)
    print("")
    print("One place for everything. Just describe what you need.")
    print("")
    print("Open http://localhost:5001")
    print("")
    print("Try:")
    print("  - 'Submit an expense for client dinner'")
    print("  - 'What projects are we working on?'")
    print("  - 'Draft a proposal for Q2 marketing'")
    print("  - 'Message the team about the deadline'")
    print("")
    app.run(host='0.0.0.0', port=5001, debug=True)
