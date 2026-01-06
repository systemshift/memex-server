"""
Memex Workspace - Flask Application

A single adaptive interface that generates UI dynamically from natural language.
"""

import os
import json
import uuid
from datetime import datetime
from typing import Dict, Any

from flask import Flask, request, jsonify, render_template, Response, stream_with_context
from flask_cors import CORS
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# Import after app creation to avoid circular imports
from core.types import WorkspaceSession, ViewSpec
from core.streaming import generative_ui, stream_ui
from services.memex import memex
from services.llm import llm
from components.library import get_tools
from components.renderer import render_component, render_components

# In-memory session storage (use Redis in production)
sessions: Dict[str, WorkspaceSession] = {}


def get_session(session_id: str) -> WorkspaceSession:
    """Get or create a workspace session"""
    if session_id not in sessions:
        sessions[session_id] = WorkspaceSession(id=session_id)
    return sessions[session_id]


# ============== Routes ==============

@app.route('/')
def index():
    """Main workspace UI"""
    return render_template('main.html')


@app.route('/api/session', methods=['POST'])
def create_session():
    """Create a new workspace session"""
    session_id = str(uuid.uuid4().hex[:12])
    session = get_session(session_id)
    return jsonify({
        "session_id": session.id,
        "created": session.created.isoformat()
    })


@app.route('/api/input', methods=['POST'])
def handle_input():
    """
    Main entry point - process user input and return generated UI.
    Returns complete HTML for the generated view.
    """
    data = request.json or {}
    session_id = data.get("session_id", "default")
    user_input = data.get("input", "").strip()

    if not user_input:
        return jsonify({"error": "No input provided"}), 400

    session = get_session(session_id)

    # Record in history
    session.history.append({
        "input": user_input,
        "timestamp": datetime.now().isoformat()
    })

    # Get context from Memex
    context = memex.get_context_for_input(user_input)

    # Classify intent
    intent = llm.classify_intent(user_input, context)

    # Generate components
    components_html = []
    components_data = []

    for tool_call in llm.stream_components(user_input, intent, context, get_tools()):
        html = render_component(tool_call.name, tool_call.arguments)
        components_html.append(html)
        components_data.append({
            "name": tool_call.name,
            "arguments": tool_call.arguments
        })

    # Build response
    full_html = "\n".join(components_html)

    # Create ViewSpec
    view_spec = ViewSpec(
        title=intent.get("title"),
        source_input=user_input,
        context={"intent": intent, "memex_context": context}
    )
    session.items.append(view_spec)
    session.active_item_id = view_spec.id

    return jsonify({
        "html": full_html,
        "view_spec_id": view_spec.id,
        "intent": intent,
        "context_count": len(context),
        "components": components_data
    })


@app.route('/api/input/stream', methods=['POST'])
def handle_input_stream():
    """
    Streaming version - returns Server-Sent Events with HTML fragments.
    Each fragment can be appended to the UI as it arrives.
    """
    data = request.json or {}
    session_id = data.get("session_id", "default")
    user_input = data.get("input", "").strip()

    if not user_input:
        return jsonify({"error": "No input provided"}), 400

    def generate():
        session = get_session(session_id)

        # Record in history
        session.history.append({
            "input": user_input,
            "timestamp": datetime.now().isoformat()
        })

        # Get context
        context = memex.get_context_for_input(user_input)

        # Classify intent
        intent = llm.classify_intent(user_input, context)

        # Send intent first
        yield f"data: {json.dumps({'type': 'intent', 'data': intent})}\n\n"

        # Stream components
        components_data = []
        for tool_call in llm.stream_components(user_input, intent, context, get_tools()):
            html = render_component(tool_call.name, tool_call.arguments)
            components_data.append({
                "name": tool_call.name,
                "arguments": tool_call.arguments
            })

            yield f"data: {json.dumps({'type': 'component', 'html': html, 'name': tool_call.name})}\n\n"

        # Send completion
        view_spec = ViewSpec(
            title=intent.get("title"),
            source_input=user_input
        )
        session.items.append(view_spec)

        yield f"data: {json.dumps({'type': 'complete', 'view_spec_id': view_spec.id})}\n\n"

    return Response(
        stream_with_context(generate()),
        mimetype='text/event-stream',
        headers={
            'Cache-Control': 'no-cache',
            'X-Accel-Buffering': 'no'
        }
    )


@app.route('/api/update', methods=['POST'])
def update_fields():
    """Update field values in a view"""
    data = request.json or {}
    session_id = data.get("session_id", "default")
    view_spec_id = data.get("view_spec_id")
    field_updates = data.get("fields", {})

    session = get_session(session_id)

    # Find the view spec
    for item in session.items:
        if item.id == view_spec_id:
            # Update fields
            for component in item.components:
                if component.name in field_updates:
                    component.value = field_updates[component.name]
                    component.done = True

            # Check if complete
            item.complete = all(
                c.done or not c.required
                for c in item.components
                if hasattr(c, 'required')
            )

            return jsonify({
                "status": "updated",
                "complete": item.complete,
                "view_spec": item.to_dict()
            })

    return jsonify({"error": "View not found"}), 404


@app.route('/api/save', methods=['POST'])
def save_to_memex():
    """Save completed view to Memex"""
    data = request.json or {}
    session_id = data.get("session_id", "default")
    view_spec_id = data.get("view_spec_id")

    session = get_session(session_id)

    # Find the view spec
    for item in session.items:
        if item.id == view_spec_id:
            # Save to Memex
            memex_id = memex.save_workspace_item(item.to_dict())

            return jsonify({
                "status": "saved",
                "memex_id": memex_id
            })

    return jsonify({"error": "View not found"}), 404


@app.route('/api/context', methods=['GET'])
def get_context():
    """Get Memex context for a query"""
    query = request.args.get("q", "")
    if not query:
        return jsonify({"context": []})

    context = memex.get_context_for_input(query)
    return jsonify({"context": context})


@app.route('/api/search', methods=['GET'])
def search():
    """Search Memex"""
    query = request.args.get("q", "")
    limit = int(request.args.get("limit", 10))

    if not query:
        return jsonify({"results": []})

    results = memex.search(query, limit=limit)
    return jsonify({
        "results": [
            {"id": r.id, "type": r.type, "meta": r.meta, "score": r.score}
            for r in results
        ]
    })


@app.route('/health')
def health():
    """Health check"""
    return jsonify({"status": "ok"})


if __name__ == '__main__':
    port = int(os.getenv("PORT", 5002))
    print("=" * 50)
    print("MEMEX WORKSPACE")
    print("=" * 50)
    print("")
    print("A single interface that adapts to what you need.")
    print("")
    print(f"Open http://localhost:{port}")
    print("")
    print("Try:")
    print("  - 'Submit an expense for client dinner $247 at Marea'")
    print("  - 'Draft a proposal for Q2 marketing campaign'")
    print("  - 'What expenses did we have last month?'")
    print("")
    app.run(host='0.0.0.0', port=port, debug=True)
