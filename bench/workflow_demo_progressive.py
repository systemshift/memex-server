#!/usr/bin/env python3
"""
Memex Progressive Workflow Demo

A single screen that evolves as the user converses. The form emerges from
conversation - not "here's a blank form, fill it" but "let's figure out
what you need together."

Key features:
- No pre-defined form templates - LLM decides what fields are needed
- State evolves with each message
- Full workflow process stored in memex as a graph
- Context cards appear as relevant information is found

Usage:
    python workflow_demo_progressive.py
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


def get_or_create_session(session_id: str) -> Dict:
    """Get existing session or create a new one"""
    if session_id not in sessions:
        sessions[session_id] = {
            "id": session_id,
            "created": datetime.now().isoformat(),
            "messages": [],
            "state": {
                "title": None,
                "fields": {},       # {name: {value, done, type, label, hint}}
                "pending": [],      # Questions to ask user
                "context": [],      # Cards from memex
                "actions": [],      # Available actions
                "complete": False
            },
            "state_history": []     # Snapshots of each state transition
        }
    return sessions[session_id]


def clear_session(session_id: str) -> None:
    """Clear a session to start fresh"""
    if session_id in sessions:
        del sessions[session_id]


# ============== Memex Integration ==============

def memex_get(path: str) -> Optional[Dict]:
    """GET request to memex API"""
    try:
        resp = requests.get(f"{MEMEX_URL}{path}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception:
        return None


def memex_post(path: str, data: Dict) -> Optional[Dict]:
    """POST request to memex API"""
    try:
        resp = requests.post(f"{MEMEX_URL}{path}", json=data, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception:
        return None


def fetch_context_from_memex(user_message: str, current_state: Dict) -> List[Dict]:
    """
    Query memex for relevant context based on the message and current state.
    Returns context cards to display to user.
    """
    context_cards = []

    # Extract search terms from message and state
    search_terms = []

    # From message
    words = user_message.split()
    for word in words:
        if len(word) > 3 and word[0].isupper():  # Likely a name/company
            search_terms.append(word)

    # From filled fields
    for field_name, field_data in current_state.get("fields", {}).items():
        if field_data.get("done") and field_data.get("value"):
            value = str(field_data["value"])
            if len(value) > 2:
                search_terms.append(value)

    # Search memex for each term
    seen_ids = set()
    for term in search_terms[:5]:  # Limit searches
        result = memex_get(f"/api/query/search?q={term}&limit=3")
        if result and result.get("results"):
            for node in result["results"]:
                node_id = node.get("ID")
                if node_id and node_id not in seen_ids:
                    seen_ids.add(node_id)
                    meta = node.get("Meta", {})
                    context_cards.append({
                        "id": node_id,
                        "title": meta.get("name", node.get("Type", "Related")),
                        "content": meta.get("summary", meta.get("description", f"Found: {term}")),
                        "type": node.get("Type", "Entity"),
                        "source": "memex"
                    })

    return context_cards[:5]  # Limit total context cards


def save_workflow_to_memex(session: Dict) -> Optional[str]:
    """
    Save the entire workflow process to memex as a graph.
    Creates: Workflow node + Turn nodes + relationships
    """
    workflow_id = f"workflow:{uuid.uuid4().hex[:12]}"

    # Create main workflow node
    workflow_resp = memex_post("/api/nodes", {
        "id": workflow_id,
        "type": "Workflow",
        "meta": {
            "title": session["state"].get("title", "Workflow"),
            "status": "complete" if session["state"].get("complete") else "in_progress",
            "created": session["created"],
            "completed": datetime.now().isoformat() if session["state"].get("complete") else None,
            "final_state": session["state"],
            "turn_count": len(session["messages"])
        }
    })

    if not workflow_resp:
        return None

    # Create turn nodes and link them
    prev_turn_id = None
    for i, msg in enumerate(session["messages"]):
        turn_id = f"turn:{workflow_id.split(':')[1]}:{i}"

        # Get state snapshot for this turn
        state_snapshot = None
        if i < len(session.get("state_history", [])):
            state_snapshot = session["state_history"][i]

        memex_post("/api/nodes", {
            "id": turn_id,
            "type": "WorkflowTurn",
            "meta": {
                "turn_number": i + 1,
                "role": msg.get("role", "user"),
                "content": msg.get("content", ""),
                "timestamp": msg.get("timestamp"),
                "state_snapshot": state_snapshot
            }
        })

        # Link turn to workflow
        memex_post("/api/links", {
            "source": workflow_id,
            "target": turn_id,
            "type": "HAS_TURN"
        })

        # Link to previous turn
        if prev_turn_id:
            memex_post("/api/links", {
                "source": prev_turn_id,
                "target": turn_id,
                "type": "NEXT"
            })
        prev_turn_id = turn_id

    # Link to related entities mentioned in fields
    for field_name, field_data in session["state"].get("fields", {}).items():
        if field_data.get("entity_id"):
            memex_post("/api/links", {
                "source": workflow_id,
                "target": field_data["entity_id"],
                "type": "RELATES_TO"
            })

    return workflow_id


# ============== LLM Processing ==============

def update_workflow_state(
    user_message: str,
    current_state: Dict,
    memex_context: List[Dict]
) -> Dict:
    """
    LLM updates the workflow state based on new user input.
    This is the core function - no pre-defined forms, the LLM decides
    what fields are needed based on what the user is trying to do.
    """

    # Format context for the prompt
    context_str = ""
    if memex_context:
        context_str = "Relevant context from company memory:\n"
        for card in memex_context:
            context_str += f"- {card['title']}: {card['content']}\n"

    prompt = f"""You are a workflow assistant. The user is building a request through natural conversation.
Your job is to understand what they're trying to do and help them complete it by updating the workflow state.

Current state:
{json.dumps(current_state, indent=2)}

{context_str}

User says: "{user_message}"

Update the workflow state based on what the user said. Follow these rules:

1. TITLE: Set a clear, short title for what the user is trying to do (e.g., "Expense Reimbursement", "Hire Request", "Contract Request")

2. FIELDS: Add/update fields based on the conversation:
   - Extract any information the user provided and mark those fields as done:true
   - Add fields that are still needed with done:false and a helpful hint
   - Each field needs: label, type (text/currency/date/select/checkbox/textarea/email/file), done (bool), value (if known), hint (if not done)
   - For select fields, include options array

3. PENDING: List 1-2 natural questions to ask about missing required information

4. CONTEXT: Include relevant context cards that would help the user:
   - Policies that apply (e.g., "Expenses under $500 auto-approved")
   - Related history (e.g., "Last similar request was 2 weeks ago")
   - Helpful contacts (e.g., "Legal team handles contracts")
   - Keep existing relevant context, add new ones from company memory

5. ACTIONS: List available actions based on completeness:
   - If complete: ["Submit", "Save Draft"]
   - If not complete: ["Save Draft"]

6. COMPLETE: Set to true only when all essential fields are filled

Return ONLY valid JSON with this structure:
{{
  "title": "string",
  "fields": {{
    "field_name": {{
      "label": "Display Label",
      "type": "text|currency|date|select|checkbox|textarea|email|file",
      "value": "extracted value or null",
      "done": true|false,
      "hint": "helpful hint if not done",
      "options": ["only", "for", "select", "type"]
    }}
  }},
  "pending": ["Question 1?", "Question 2?"],
  "context": [
    {{"title": "Card Title", "content": "Helpful information"}}
  ],
  "actions": ["Action 1", "Action 2"],
  "complete": true|false
}}

IMPORTANT: Adapt the fields to what the user actually needs. Don't use fixed templates.
If they mention an expense, include expense-related fields. If they mention hiring, include hiring fields.
Be smart about inferring context - if they say "last Tuesday" convert it to a date."""

    try:
        response = llm_client.chat.completions.create(
            model=MODEL,
            messages=[
                {"role": "system", "content": "You are a helpful workflow assistant. Return only valid JSON."},
                {"role": "user", "content": prompt}
            ],
            response_format={"type": "json_object"}
        )
        return json.loads(response.choices[0].message.content)
    except Exception as e:
        # Return current state with error context
        return {
            **current_state,
            "context": current_state.get("context", []) + [
                {"title": "Error", "content": f"Failed to process: {str(e)}"}
            ]
        }


# ============== Routes ==============

@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/session', methods=['POST'])
def create_session():
    """Create a new session or get existing one"""
    data = request.json or {}
    session_id = data.get("session_id", str(uuid.uuid4().hex[:12]))
    session = get_or_create_session(session_id)
    return jsonify({
        "session_id": session["id"],
        "state": session["state"]
    })


@app.route('/api/session/<session_id>', methods=['DELETE'])
def delete_session(session_id: str):
    """Clear a session"""
    clear_session(session_id)
    return jsonify({"status": "cleared"})


@app.route('/api/message', methods=['POST'])
def handle_message():
    """Handle a message in the conversation - this updates the workflow state"""
    data = request.json
    session_id = data.get("session_id", "default")
    user_message = data.get("message", "").strip()

    if not user_message:
        return jsonify({"error": "No message provided"}), 400

    session = get_or_create_session(session_id)

    # Save current state to history before updating
    session["state_history"].append(copy.deepcopy(session["state"]))

    # Add user message to conversation
    session["messages"].append({
        "role": "user",
        "content": user_message,
        "timestamp": datetime.now().isoformat()
    })

    # Fetch context from memex
    memex_context = fetch_context_from_memex(user_message, session["state"])

    # Merge memex context with existing context
    existing_context = session["state"].get("context", [])
    all_context = existing_context + [c for c in memex_context if c not in existing_context]

    # LLM updates the state
    new_state = update_workflow_state(user_message, session["state"], all_context)

    # Merge new memex context into state context
    if memex_context:
        state_context = new_state.get("context", [])
        for mc in memex_context:
            if mc not in state_context:
                state_context.append(mc)
        new_state["context"] = state_context

    session["state"] = new_state

    return jsonify({
        "session_id": session_id,
        "state": new_state,
        "message_count": len(session["messages"]),
        "memex_context_added": len(memex_context)
    })


@app.route('/api/submit', methods=['POST'])
def submit_workflow():
    """Submit a completed workflow - saves full process to memex"""
    data = request.json
    session_id = data.get("session_id", "default")

    session = sessions.get(session_id)
    if not session:
        return jsonify({"error": "Session not found"}), 404

    # Save to memex
    workflow_id = save_workflow_to_memex(session)

    if workflow_id:
        return jsonify({
            "status": "submitted",
            "workflow_id": workflow_id,
            "message": "Workflow saved to memex with full conversation history"
        })
    else:
        return jsonify({
            "status": "submitted_local",
            "message": "Memex not available, workflow processed locally"
        })


@app.route('/api/history/<session_id>')
def get_history(session_id: str):
    """Get the full conversation history for a session"""
    session = sessions.get(session_id)
    if not session:
        return jsonify({"error": "Session not found"}), 404

    return jsonify({
        "messages": session["messages"],
        "state_history": session["state_history"],
        "current_state": session["state"]
    })


# ============== HTML Template ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Anchor Flow - Progressive Workflows</title>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;600&family=Space+Grotesk:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #0a0a0a;
            --bg-secondary: #111111;
            --bg-tertiary: #1a1a1a;
            --text: #e0e0e0;
            --text-dim: #707070;
            --accent: #00ff88;
            --accent-dim: #00aa5a;
            --border: #222;
            --done: #00ff88;
            --pending: #ffa500;
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
        }
        .logo span { color: var(--text-dim); }

        .container {
            max-width: 900px;
            margin: 0 auto;
            padding: 2rem;
        }

        .intro {
            text-align: center;
            margin-bottom: 2rem;
        }

        .intro h1 {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
        }

        .intro p {
            color: var(--text-dim);
            font-size: 0.9rem;
        }

        /* Main layout - single column */
        .main-panel {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
        }

        .panel-header {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.85rem;
            color: var(--accent);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        /* Workflow state display */
        .workflow-title {
            font-size: 1.3rem;
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .workflow-title .status {
            margin-left: auto;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
        }

        .workflow-title .status.complete {
            background: var(--done);
            color: #000;
        }

        .workflow-title .status.in-progress {
            background: var(--pending);
            color: #000;
        }

        /* Fields display */
        .fields-section {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
        }

        .section-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.1em;
            margin-bottom: 0.75rem;
        }

        .field-row {
            display: flex;
            align-items: center;
            padding: 0.5rem 0.75rem;
            background: var(--bg-tertiary);
            border-radius: 4px;
            margin-bottom: 0.5rem;
            gap: 1rem;
        }

        .field-row.done {
            border-left: 3px solid var(--done);
        }

        .field-row.pending {
            border-left: 3px solid var(--pending);
        }

        .field-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            color: var(--text-dim);
            min-width: 120px;
        }

        .field-value {
            flex: 1;
            font-size: 0.95rem;
        }

        .field-value.empty {
            color: var(--text-dim);
            font-style: italic;
        }

        .field-status {
            font-size: 1rem;
        }

        /* Context cards */
        .context-section {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            background: rgba(0, 255, 136, 0.03);
        }

        .context-cards {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .context-card {
            padding: 0.75rem;
            background: var(--bg-tertiary);
            border-radius: 4px;
            border-left: 3px solid var(--accent-dim);
        }

        .context-card-title {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--accent);
            margin-bottom: 0.25rem;
        }

        .context-card-content {
            font-size: 0.85rem;
            color: var(--text);
        }

        /* Pending questions */
        .pending-section {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
        }

        .pending-question {
            padding: 0.5rem 0.75rem;
            background: rgba(255, 165, 0, 0.1);
            border-radius: 4px;
            margin-bottom: 0.5rem;
            font-size: 0.9rem;
            color: var(--pending);
        }

        /* Actions */
        .actions-section {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            gap: 0.5rem;
        }

        .action-btn {
            flex: 1;
            padding: 0.75rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.85rem;
            border-radius: 4px;
            cursor: pointer;
            border: 1px solid var(--border);
            background: var(--bg-tertiary);
            color: var(--text);
        }

        .action-btn.primary {
            background: var(--accent);
            color: #000;
            border-color: var(--accent);
        }

        .action-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        /* Chat input */
        .chat-section {
            padding: 1rem;
        }

        .chat-input-row {
            display: flex;
            gap: 0.5rem;
        }

        .chat-input {
            flex: 1;
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.75rem 1rem;
            font-family: 'Space Grotesk', sans-serif;
            font-size: 0.95rem;
            border-radius: 4px;
        }

        .chat-input:focus {
            outline: none;
            border-color: var(--accent);
        }

        .send-btn {
            background: var(--accent);
            color: #000;
            border: none;
            padding: 0.75rem 1.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9rem;
            font-weight: 500;
            border-radius: 4px;
            cursor: pointer;
        }

        .send-btn:hover { background: var(--accent-dim); color: #fff; }
        .send-btn:disabled { background: var(--border); cursor: not-allowed; }

        /* Empty state */
        .empty-state {
            padding: 3rem;
            text-align: center;
            color: var(--text-dim);
        }

        .empty-state h3 {
            margin-bottom: 0.5rem;
            color: var(--text);
        }

        /* Conversation history toggle */
        .history-toggle {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            cursor: pointer;
            padding: 0.5rem;
        }

        .history-toggle:hover {
            color: var(--accent);
        }

        .conversation-history {
            padding: 1rem;
            border-top: 1px solid var(--border);
            display: none;
        }

        .conversation-history.visible {
            display: block;
        }

        .message {
            padding: 0.5rem 0.75rem;
            border-radius: 4px;
            margin-bottom: 0.5rem;
            font-size: 0.85rem;
        }

        .message.user {
            background: var(--bg-tertiary);
            margin-left: 2rem;
        }

        .message.system {
            background: rgba(0, 255, 136, 0.1);
            margin-right: 2rem;
            color: var(--accent);
        }

        /* New session button */
        .new-session-btn {
            background: transparent;
            border: 1px solid var(--border);
            color: var(--text-dim);
            padding: 0.5rem 1rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            border-radius: 4px;
            cursor: pointer;
        }

        .new-session-btn:hover {
            border-color: var(--accent);
            color: var(--accent);
        }

        /* Example prompts */
        .examples {
            display: flex;
            gap: 0.5rem;
            flex-wrap: wrap;
            margin-bottom: 1rem;
        }

        .example-chip {
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text-dim);
            padding: 0.4rem 0.8rem;
            font-size: 0.8rem;
            border-radius: 20px;
            cursor: pointer;
        }

        .example-chip:hover {
            border-color: var(--accent);
            color: var(--accent);
        }
    </style>
</head>
<body>
    <header>
        <div class="logo">anchor<span>.flow</span></div>
        <button class="new-session-btn" onclick="newSession()">+ New Workflow</button>
    </header>

    <div class="container">
        <div class="intro">
            <h1>Describe what you need. The form emerges.</h1>
            <p>No blank forms to fill - just talk naturally and watch the workflow take shape.</p>
        </div>

        <div class="main-panel">
            <div class="panel-header">
                <span id="session-indicator">New Workflow</span>
                <span class="history-toggle" onclick="toggleHistory()">Show History</span>
            </div>

            <!-- Workflow content - updates dynamically -->
            <div id="workflow-content">
                <div class="empty-state">
                    <h3>Start a new workflow</h3>
                    <p>Try one of these examples or describe what you need:</p>
                    <div class="examples" style="margin-top: 1rem; justify-content: center;">
                        <span class="example-chip" onclick="useExample('expense')">Expense reimbursement</span>
                        <span class="example-chip" onclick="useExample('hire')">Hire request</span>
                        <span class="example-chip" onclick="useExample('contract')">Contract review</span>
                        <span class="example-chip" onclick="useExample('support')">Support ticket</span>
                    </div>
                </div>
            </div>

            <!-- Conversation history (hidden by default) -->
            <div id="conversation-history" class="conversation-history"></div>

            <!-- Chat input -->
            <div class="chat-section">
                <div class="chat-input-row">
                    <input type="text" id="chat-input" class="chat-input"
                           placeholder="Describe what you need..."
                           onkeypress="if(event.key==='Enter')sendMessage()">
                    <button class="send-btn" id="send-btn" onclick="sendMessage()">Send</button>
                </div>
            </div>
        </div>
    </div>

    <script>
        let sessionId = null;
        let currentState = null;
        let messages = [];

        // Initialize session on load
        document.addEventListener('DOMContentLoaded', () => {
            initSession();
        });

        async function initSession() {
            const resp = await fetch('/api/session', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({})
            });
            const data = await resp.json();
            sessionId = data.session_id;
            currentState = data.state;
            document.getElementById('session-indicator').textContent = `Session: ${sessionId}`;
        }

        async function newSession() {
            if (sessionId) {
                await fetch(`/api/session/${sessionId}`, { method: 'DELETE' });
            }
            messages = [];
            document.getElementById('conversation-history').innerHTML = '';
            document.getElementById('workflow-content').innerHTML = `
                <div class="empty-state">
                    <h3>Start a new workflow</h3>
                    <p>Try one of these examples or describe what you need:</p>
                    <div class="examples" style="margin-top: 1rem; justify-content: center;">
                        <span class="example-chip" onclick="useExample('expense')">Expense reimbursement</span>
                        <span class="example-chip" onclick="useExample('hire')">Hire request</span>
                        <span class="example-chip" onclick="useExample('contract')">Contract review</span>
                        <span class="example-chip" onclick="useExample('support')">Support ticket</span>
                    </div>
                </div>
            `;
            await initSession();
        }

        function useExample(type) {
            const examples = {
                expense: "I need to get reimbursed for a client dinner at Marea restaurant, $247",
                hire: "We need to hire a senior backend engineer for the payments team",
                contract: "Need an NDA with TechStart Inc before our product demo next week",
                support: "Customer BigCorp can't access their dashboard - getting 500 errors"
            };
            document.getElementById('chat-input').value = examples[type] || '';
            document.getElementById('chat-input').focus();
        }

        async function sendMessage() {
            const input = document.getElementById('chat-input');
            const message = input.value.trim();
            if (!message) return;

            const btn = document.getElementById('send-btn');
            btn.disabled = true;
            btn.textContent = '...';

            // Add to local messages
            messages.push({ role: 'user', content: message });
            updateHistoryDisplay();

            try {
                const resp = await fetch('/api/message', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ session_id: sessionId, message })
                });
                const data = await resp.json();

                if (data.error) {
                    alert('Error: ' + data.error);
                    return;
                }

                currentState = data.state;
                renderWorkflowState(currentState);
                input.value = '';

            } catch (e) {
                alert('Error: ' + e.message);
            } finally {
                btn.disabled = false;
                btn.textContent = 'Send';
            }
        }

        function renderWorkflowState(state) {
            const container = document.getElementById('workflow-content');

            if (!state.title) {
                container.innerHTML = `<div class="empty-state">Processing...</div>`;
                return;
            }

            // Build fields HTML
            const fieldsHtml = Object.entries(state.fields || {}).map(([name, field]) => {
                const isDone = field.done;
                const value = field.value || field.hint || 'Not provided';
                return `
                    <div class="field-row ${isDone ? 'done' : 'pending'}">
                        <span class="field-label">${field.label || name}</span>
                        <span class="field-value ${isDone ? '' : 'empty'}">${isDone ? value : field.hint || 'Needed'}</span>
                        <span class="field-status">${isDone ? '✓' : '?'}</span>
                    </div>
                `;
            }).join('');

            // Build context cards HTML
            const contextHtml = (state.context || []).map(card => `
                <div class="context-card">
                    <div class="context-card-title">${card.title}</div>
                    <div class="context-card-content">${card.content}</div>
                </div>
            `).join('');

            // Build pending questions HTML
            const pendingHtml = (state.pending || []).map(q => `
                <div class="pending-question">${q}</div>
            `).join('');

            // Build actions HTML
            const actionsHtml = (state.actions || []).map((action, i) => `
                <button class="action-btn ${i === 0 ? 'primary' : ''}"
                        onclick="handleAction('${action}')"
                        ${!state.complete && action === 'Submit' ? 'disabled' : ''}>
                    ${action}
                </button>
            `).join('');

            container.innerHTML = `
                <div class="workflow-title">
                    <span>${state.title}</span>
                    <span class="status ${state.complete ? 'complete' : 'in-progress'}">
                        ${state.complete ? 'Ready' : 'In Progress'}
                    </span>
                </div>

                ${Object.keys(state.fields || {}).length > 0 ? `
                    <div class="fields-section">
                        <div class="section-label">Information</div>
                        ${fieldsHtml}
                    </div>
                ` : ''}

                ${(state.context || []).length > 0 ? `
                    <div class="context-section">
                        <div class="section-label">Context from Company Memory</div>
                        <div class="context-cards">${contextHtml}</div>
                    </div>
                ` : ''}

                ${(state.pending || []).length > 0 ? `
                    <div class="pending-section">
                        <div class="section-label">Questions</div>
                        ${pendingHtml}
                    </div>
                ` : ''}

                ${(state.actions || []).length > 0 ? `
                    <div class="actions-section">
                        ${actionsHtml}
                    </div>
                ` : ''}
            `;
        }

        async function handleAction(action) {
            if (action === 'Submit') {
                const resp = await fetch('/api/submit', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ session_id: sessionId })
                });
                const data = await resp.json();
                alert(`Workflow submitted!\\n\\n${data.workflow_id ? 'Saved as: ' + data.workflow_id : data.message}`);
            } else if (action === 'Save Draft') {
                alert('Draft saved locally. Continue when ready.');
            }
        }

        function toggleHistory() {
            const history = document.getElementById('conversation-history');
            history.classList.toggle('visible');
            const toggle = document.querySelector('.history-toggle');
            toggle.textContent = history.classList.contains('visible') ? 'Hide History' : 'Show History';
        }

        function updateHistoryDisplay() {
            const container = document.getElementById('conversation-history');
            container.innerHTML = messages.map(m => `
                <div class="message ${m.role}">
                    <strong>${m.role === 'user' ? 'You' : 'System'}:</strong> ${m.content}
                </div>
            `).join('');
        }
    </script>
</body>
</html>
"""


if __name__ == '__main__':
    print("Starting Anchor Flow Progressive Demo...")
    print("Open http://localhost:5003")
    print("\nThis demo features:")
    print("  - Progressive form building through conversation")
    print("  - No pre-defined templates - forms emerge from context")
    print("  - Full workflow process saved to Memex")
    app.run(host='0.0.0.0', port=5003, debug=True)
