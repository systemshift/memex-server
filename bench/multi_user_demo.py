#!/usr/bin/env python3
"""
Multi-User Workflow Demo - Visual Web Interface

Shows multiple users working on workflows simultaneously,
demonstrating how context spills over between users.

Run this instead of workflow_demo_progressive.py for demos.

Usage:
    python multi_user_demo.py
    # Open http://localhost:5003
"""

import json
import os
import uuid
import copy
from datetime import datetime
from typing import Dict, Any, List, Optional

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

# ============== Session Management ==============

sessions: Dict[str, Dict] = {}

# Predefined users for demo
DEMO_USERS = [
    {"id": "alice", "name": "Alice", "dept": "Engineering", "color": "#4ecdc4"},
    {"id": "bob", "name": "Bob", "dept": "Engineering", "color": "#ff6b6b"},
    {"id": "carol", "name": "Carol", "dept": "Sales", "color": "#a29bfe"},
    {"id": "dave", "name": "Dave", "dept": "Finance", "color": "#ffeaa7"},
]

# Example prompts for demo
EXAMPLE_PROMPTS = [
    {"user": "alice", "text": "Need to expense team lunch at Chipotle, $89"},
    {"user": "bob", "text": "Expense for team dinner at Chipotle, $156"},
    {"user": "carol", "text": "Client dinner at Nobu with Acme Corp, $847"},
    {"user": "alice", "text": "Need to hire a senior backend engineer for payments team"},
    {"user": "bob", "text": "Looking to hire a frontend developer, React experience needed"},
    {"user": "carol", "text": "Need an NDA with TechStart Inc before our demo"},
]


def get_or_create_session(user_id: str) -> Dict:
    """Get existing session or create a new one for a user"""
    if user_id not in sessions:
        user = next((u for u in DEMO_USERS if u["id"] == user_id), {"id": user_id, "name": user_id, "dept": "Unknown", "color": "#888"})
        sessions[user_id] = {
            "id": user_id,
            "user": user,
            "created": datetime.now().isoformat(),
            "messages": [],
            "state": {
                "title": None,
                "fields": {},
                "pending": [],
                "context": [],
                "actions": [],
                "complete": False
            },
            "state_history": [],
            "workflow_id": None
        }
    return sessions[user_id]


def reset_all_sessions():
    """Clear all sessions for fresh demo"""
    global sessions
    sessions = {}


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


def fetch_context_from_memex(user_message: str, current_state: Dict, user_id: str) -> List[Dict]:
    """Fetch context from memex - this enables cross-user learning"""
    context_cards = []
    seen_ids = set()

    # Search for similar workflows
    workflow_title = current_state.get("title", "")
    if workflow_title:
        result = memex_get(f"/api/query/search?q={workflow_title}&limit=5")
        if result and result.get("nodes"):
            for node in result["nodes"]:
                if node.get("Type") == "Workflow" and node.get("ID") not in seen_ids:
                    seen_ids.add(node["ID"])
                    meta = node.get("Meta", {})
                    submitter = meta.get("user_name", "Someone")
                    context_cards.append({
                        "id": node["ID"],
                        "title": f"{submitter}'s similar workflow",
                        "content": meta.get("title", "Related request"),
                        "type": "Workflow",
                        "from_user": submitter
                    })

    # Search for keywords in message
    message_lower = user_message.lower()
    search_terms = []

    for word in user_message.split():
        if len(word) > 3 and word[0].isupper():
            search_terms.append(word)

    for term in search_terms[:3]:
        result = memex_get(f"/api/query/search?q={term}&limit=3")
        if result and result.get("nodes"):
            for node in result["nodes"]:
                node_id = node.get("ID")
                if node_id and node_id not in seen_ids:
                    seen_ids.add(node_id)
                    meta = node.get("Meta", {})
                    node_type = node.get("Type", "")

                    if node_type == "Workflow":
                        submitter = meta.get("user_name", "Someone")
                        context_cards.append({
                            "id": node_id,
                            "title": f"Related: {meta.get('title', term)}",
                            "content": f"From {submitter}",
                            "type": node_type,
                            "from_user": submitter
                        })
                    elif node_type == "WorkflowTurn":
                        context_cards.append({
                            "id": node_id,
                            "title": "Previous conversation",
                            "content": meta.get("content", "")[:60] + "...",
                            "type": node_type
                        })

    # Search for expense-related patterns
    if any(w in message_lower for w in ["expense", "reimburse", "dinner", "lunch"]):
        result = memex_get("/api/query/search?q=Expense&limit=3")
        if result and result.get("nodes"):
            for node in result["nodes"]:
                if node.get("Type") == "Workflow" and node.get("ID") not in seen_ids:
                    seen_ids.add(node["ID"])
                    meta = node.get("Meta", {})
                    submitter = meta.get("user_name", "Someone")
                    context_cards.append({
                        "id": node["ID"],
                        "title": f"Recent expense by {submitter}",
                        "content": meta.get("title", "Expense"),
                        "type": "Workflow",
                        "from_user": submitter
                    })

    return context_cards[:5]


def save_workflow_to_memex(session: Dict) -> Optional[str]:
    """Save workflow with user info for cross-user context"""
    workflow_id = f"workflow:{session['user']['id']}:{uuid.uuid4().hex[:8]}"

    workflow_resp = memex_post("/api/nodes", {
        "id": workflow_id,
        "type": "Workflow",
        "meta": {
            "title": session["state"].get("title", "Workflow"),
            "status": "complete" if session["state"].get("complete") else "in_progress",
            "user_id": session["user"]["id"],
            "user_name": session["user"]["name"],
            "user_dept": session["user"]["dept"],
            "created": session["created"],
            "final_state": session["state"],
            "turn_count": len(session["messages"])
        }
    })

    if not workflow_resp:
        return None

    # Create turn nodes
    prev_turn_id = None
    for i, msg in enumerate(session["messages"]):
        turn_id = f"turn:{workflow_id.split(':')[1]}:{workflow_id.split(':')[2]}:{i}"

        state_snapshot = session["state_history"][i] if i < len(session["state_history"]) else None

        memex_post("/api/nodes", {
            "id": turn_id,
            "type": "WorkflowTurn",
            "meta": {
                "turn_number": i + 1,
                "user_id": session["user"]["id"],
                "user_name": session["user"]["name"],
                "content": msg.get("content", ""),
                "timestamp": msg.get("timestamp"),
                "state_snapshot": state_snapshot
            }
        })

        memex_post("/api/links", {"source": workflow_id, "target": turn_id, "type": "HAS_TURN"})

        if prev_turn_id:
            memex_post("/api/links", {"source": prev_turn_id, "target": turn_id, "type": "NEXT"})
        prev_turn_id = turn_id

    return workflow_id


# ============== LLM Processing ==============

def update_workflow_state(user_message: str, current_state: Dict, context: List[Dict], user: Dict) -> Dict:
    """LLM updates the workflow state"""

    context_str = ""
    if context:
        context_str = "Context from other users' workflows:\n"
        for card in context:
            from_user = card.get("from_user", "")
            context_str += f"- {card['title']}: {card['content']}" + (f" (from {from_user})" if from_user else "") + "\n"

    prompt = f"""You are a workflow assistant for {user['name']} in the {user['dept']} department.
The user is building a request through conversation.

Current state:
{json.dumps(current_state, indent=2)}

{context_str}

User says: "{user_message}"

Update the workflow state. Rules:
1. Set a clear title for what they're doing
2. Extract information into fields (done:true if known, done:false if needed)
3. Add pending questions for missing info
4. Include helpful context cards (policies, tips)
5. Set complete:true when all essential fields are filled

Return JSON:
{{
  "title": "string",
  "fields": {{"name": {{"label": "Label", "type": "text|currency|date|select", "value": "or null", "done": bool, "hint": "if not done"}}}},
  "pending": ["Question?"],
  "context": [{{"title": "Title", "content": "Info"}}],
  "actions": ["Submit", "Save Draft"],
  "complete": bool
}}"""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[
                {"role": "system", "content": "Return only valid JSON."},
                {"role": "user", "content": prompt}
            ],
            response_format={"type": "json_object"}
        )
        return json.loads(response.choices[0].message.content)
    except Exception as e:
        return {**current_state, "context": current_state.get("context", []) + [{"title": "Error", "content": str(e)}]}


# ============== Routes ==============

@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/users')
def get_users():
    """Get all demo users with their current state"""
    users_state = []
    for user in DEMO_USERS:
        session = sessions.get(user["id"])
        users_state.append({
            **user,
            "has_session": session is not None,
            "state": session["state"] if session else None,
            "message_count": len(session["messages"]) if session else 0,
            "workflow_id": session.get("workflow_id") if session else None
        })
    return jsonify(users_state)


@app.route('/api/examples')
def get_examples():
    return jsonify(EXAMPLE_PROMPTS)


@app.route('/api/reset', methods=['POST'])
def reset():
    """Reset all sessions for fresh demo"""
    reset_all_sessions()
    return jsonify({"status": "reset"})


@app.route('/api/message', methods=['POST'])
def handle_message():
    """Handle a message from a specific user"""
    data = request.json
    user_id = data.get("user_id", "alice")
    user_message = data.get("message", "").strip()

    if not user_message:
        return jsonify({"error": "No message"}), 400

    session = get_or_create_session(user_id)
    session["state_history"].append(copy.deepcopy(session["state"]))
    session["messages"].append({
        "role": "user",
        "content": user_message,
        "timestamp": datetime.now().isoformat()
    })

    # Fetch context from memex (cross-user learning!)
    memex_context = fetch_context_from_memex(user_message, session["state"], user_id)

    # Update state via LLM
    new_state = update_workflow_state(user_message, session["state"], memex_context, session["user"])

    # Merge memex context
    if memex_context:
        existing = new_state.get("context", [])
        for mc in memex_context:
            if mc not in existing:
                existing.append(mc)
        new_state["context"] = existing[:6]

    session["state"] = new_state

    return jsonify({
        "user_id": user_id,
        "state": new_state,
        "message_count": len(session["messages"]),
        "memex_context_count": len(memex_context)
    })


@app.route('/api/submit', methods=['POST'])
def submit_workflow():
    """Submit a user's workflow to memex"""
    data = request.json
    user_id = data.get("user_id", "alice")

    session = sessions.get(user_id)
    if not session:
        return jsonify({"error": "No session"}), 404

    workflow_id = save_workflow_to_memex(session)
    session["workflow_id"] = workflow_id

    return jsonify({
        "status": "submitted",
        "workflow_id": workflow_id,
        "user": session["user"]["name"]
    })


# ============== HTML Template ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html>
<head>
    <title>Multi-User Workflow Demo</title>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Space+Grotesk:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #0a0a0a;
            --bg-secondary: #111;
            --bg-tertiary: #1a1a1a;
            --text: #e0e0e0;
            --text-dim: #666;
            --border: #222;
            --accent: #00ff88;
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Space Grotesk', sans-serif;
            background: var(--bg);
            color: var(--text);
            min-height: 100vh;
            padding: 1rem;
        }
        .mono { font-family: 'JetBrains Mono', monospace; }

        header {
            text-align: center;
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            margin-bottom: 1rem;
        }
        header h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
        header p { color: var(--text-dim); font-size: 0.9rem; }

        .controls {
            display: flex;
            justify-content: center;
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .controls button {
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem 1rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            cursor: pointer;
            border-radius: 4px;
        }
        .controls button:hover { border-color: var(--accent); color: var(--accent); }

        .users-grid {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 1rem;
            max-width: 1600px;
            margin: 0 auto;
        }

        .user-panel {
            background: var(--bg-secondary);
            border: 2px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
        }

        .user-header {
            padding: 0.75rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
        .user-avatar {
            width: 32px;
            height: 32px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: bold;
            font-size: 0.9rem;
        }
        .user-info { flex: 1; }
        .user-name { font-weight: 500; }
        .user-dept { font-size: 0.75rem; color: var(--text-dim); }
        .user-status {
            font-size: 0.7rem;
            padding: 0.2rem 0.5rem;
            border-radius: 10px;
            background: var(--bg-tertiary);
        }
        .user-status.active { background: var(--accent); color: #000; }

        .workflow-state {
            padding: 0.75rem;
            min-height: 200px;
            font-size: 0.85rem;
        }
        .workflow-title {
            font-weight: 500;
            margin-bottom: 0.5rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
        .field-row {
            display: flex;
            padding: 0.25rem 0;
            font-size: 0.8rem;
        }
        .field-status { width: 20px; }
        .field-label { color: var(--text-dim); width: 80px; }
        .field-value { flex: 1; }

        .context-card {
            background: var(--bg-tertiary);
            padding: 0.5rem;
            margin-top: 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            border-left: 2px solid var(--accent);
        }
        .context-title { color: var(--accent); font-weight: 500; }

        .user-input {
            padding: 0.75rem;
            border-top: 1px solid var(--border);
        }
        .user-input input {
            width: 100%;
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem;
            font-size: 0.85rem;
            border-radius: 4px;
        }
        .user-input input:focus { outline: none; border-color: var(--accent); }

        .examples-bar {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
            max-width: 1600px;
            margin-left: auto;
            margin-right: auto;
        }
        .examples-bar h3 { font-size: 0.85rem; margin-bottom: 0.5rem; color: var(--text-dim); }
        .examples-list {
            display: flex;
            flex-wrap: wrap;
            gap: 0.5rem;
        }
        .example-chip {
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            padding: 0.4rem 0.8rem;
            font-size: 0.75rem;
            border-radius: 20px;
            cursor: pointer;
        }
        .example-chip:hover { border-color: var(--accent); }
        .example-chip .user-tag {
            font-weight: bold;
            margin-right: 0.25rem;
        }

        .activity-log {
            max-width: 1600px;
            margin: 1rem auto;
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
        }
        .activity-log h3 { font-size: 0.85rem; margin-bottom: 0.5rem; color: var(--text-dim); }
        .log-entry {
            font-size: 0.8rem;
            padding: 0.25rem 0;
            border-bottom: 1px solid var(--border);
        }
        .log-entry:last-child { border: none; }
        .log-user { font-weight: bold; }
        .log-context { color: var(--accent); }

        @media (max-width: 1200px) {
            .users-grid { grid-template-columns: repeat(2, 1fr); }
        }
    </style>
</head>
<body>
    <header>
        <h1>Multi-User Workflow Demo</h1>
        <p>Watch how context spills over between users. Each workflow becomes knowledge for the organization.</p>
    </header>

    <div class="controls">
        <button onclick="resetDemo()">Reset All</button>
        <button onclick="loadUsers()">Refresh</button>
    </div>

    <div class="examples-bar">
        <h3>Quick Examples (click to send as that user)</h3>
        <div class="examples-list" id="examples"></div>
    </div>

    <div class="users-grid" id="users-grid"></div>

    <div class="activity-log">
        <h3>Activity Log</h3>
        <div id="activity-log"></div>
    </div>

    <script>
        let users = [];
        let activityLog = [];

        async function loadUsers() {
            const resp = await fetch('/api/users');
            users = await resp.json();
            renderUsers();
        }

        async function loadExamples() {
            const resp = await fetch('/api/examples');
            const examples = await resp.json();
            document.getElementById('examples').innerHTML = examples.map((ex, i) => {
                const user = users.find(u => u.id === ex.user) || {name: ex.user, color: '#888'};
                return `<span class="example-chip" onclick="sendExample(${i})" style="border-left: 3px solid ${user.color}">
                    <span class="user-tag">${user.name}:</span> ${ex.text.substring(0, 40)}...
                </span>`;
            }).join('');
            window.exampleData = examples;
        }

        function renderUsers() {
            document.getElementById('users-grid').innerHTML = users.map(user => `
                <div class="user-panel" style="border-color: ${user.color}40">
                    <div class="user-header">
                        <div class="user-avatar" style="background: ${user.color}40; color: ${user.color}">${user.name[0]}</div>
                        <div class="user-info">
                            <div class="user-name">${user.name}</div>
                            <div class="user-dept">${user.dept}</div>
                        </div>
                        <span class="user-status ${user.has_session ? 'active' : ''}">${user.message_count || 0} msgs</span>
                    </div>
                    <div class="workflow-state" id="state-${user.id}">
                        ${user.state ? renderState(user.state) : '<div style="color: var(--text-dim); text-align: center; padding: 2rem;">No active workflow</div>'}
                    </div>
                    <div class="user-input">
                        <input type="text" id="input-${user.id}" placeholder="Type as ${user.name}..."
                               onkeypress="if(event.key==='Enter')sendMessage('${user.id}')">
                    </div>
                </div>
            `).join('');
        }

        function renderState(state) {
            if (!state || !state.title) return '<div style="color: var(--text-dim);">Waiting...</div>';

            let html = `<div class="workflow-title">${state.title} ${state.complete ? '✓' : ''}</div>`;

            for (const [name, field] of Object.entries(state.fields || {})) {
                html += `<div class="field-row">
                    <span class="field-status">${field.done ? '✓' : '?'}</span>
                    <span class="field-label">${field.label || name}</span>
                    <span class="field-value">${field.value || field.hint || '-'}</span>
                </div>`;
            }

            for (const ctx of (state.context || []).slice(0, 2)) {
                html += `<div class="context-card">
                    <div class="context-title">${ctx.title}</div>
                    <div>${ctx.content}</div>
                </div>`;
            }

            return html;
        }

        async function sendMessage(userId) {
            const input = document.getElementById(`input-${userId}`);
            const message = input.value.trim();
            if (!message) return;

            const user = users.find(u => u.id === userId);
            addLog(user.name, user.color, `"${message}"`);

            input.value = '';
            input.disabled = true;

            try {
                const resp = await fetch('/api/message', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ user_id: userId, message })
                });
                const data = await resp.json();

                if (data.memex_context_count > 0) {
                    addLog(user.name, user.color, `Found ${data.memex_context_count} context items from other users`, true);
                }

                // Auto-submit if complete
                if (data.state.complete) {
                    const submitResp = await fetch('/api/submit', {
                        method: 'POST',
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({ user_id: userId })
                    });
                    const submitData = await submitResp.json();
                    addLog(user.name, user.color, `Submitted: ${submitData.workflow_id}`, true);
                }

                await loadUsers();
            } finally {
                input.disabled = false;
            }
        }

        async function sendExample(index) {
            const ex = window.exampleData[index];
            document.getElementById(`input-${ex.user}`).value = ex.text;
            await sendMessage(ex.user);
        }

        async function resetDemo() {
            await fetch('/api/reset', { method: 'POST' });
            activityLog = [];
            document.getElementById('activity-log').innerHTML = '';
            await loadUsers();
        }

        function addLog(userName, color, message, isContext = false) {
            const entry = `<div class="log-entry">
                <span class="log-user" style="color: ${color}">${userName}</span>:
                ${isContext ? '<span class="log-context">' + message + '</span>' : message}
            </div>`;
            document.getElementById('activity-log').innerHTML = entry + document.getElementById('activity-log').innerHTML;
        }

        // Init
        loadUsers().then(loadExamples);
    </script>
</body>
</html>
"""


if __name__ == '__main__':
    print("Multi-User Workflow Demo")
    print("========================")
    print("Open http://localhost:5003")
    print("")
    print("This demo shows 4 users working simultaneously.")
    print("Watch how context spills over between users!")
    app.run(host='0.0.0.0', port=5003, debug=True)
