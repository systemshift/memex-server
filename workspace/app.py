"""
Memex Workspace - Flask Application

A multi-user workflow system with generative UI.
Work flows between people with full context preservation.
"""

import os
import json
import uuid
import time
from datetime import datetime
from typing import Dict, Any

from flask import Flask, request, jsonify, render_template, Response, stream_with_context
from flask_cors import CORS
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

# Import after app creation to avoid circular imports
from core.types import WorkspaceSession, ViewSpec, WorkItemStatus
from core.streaming import generative_ui, stream_ui
from core.extraction import extract_anchors, extract_with_handoff_detection
from core.notifications import notifications
from services.memex import memex
from services.llm import llm
from services.users import get_user, get_all_users, get_users_by_role, DEMO_USERS
from services.handoffs import (
    create_work_item, get_work_item, get_work_items_for_user,
    get_pending_work_items, create_handoff, get_handoff,
    get_handoffs_for_user, get_pending_handoffs, accept_handoff,
    update_work_item_status, get_handoff_chain, get_all_work_items,
    get_workflow_stats
)
from components.library import get_tools
from components.renderer import render_component, render_components
from components.role_components import get_role_config, get_role_system_prompt, get_handoff_targets, get_stage_for_role

# In-memory session storage (use Redis in production)
sessions: Dict[str, WorkspaceSession] = {}


def get_session(session_id: str, user_id: str = None) -> WorkspaceSession:
    """Get or create a workspace session"""
    if session_id not in sessions:
        sessions[session_id] = WorkspaceSession(id=session_id, user_id=user_id)
    elif user_id:
        sessions[session_id].user_id = user_id
        user = get_user(user_id)
        if user:
            sessions[session_id].user_role = user.role
    return sessions[session_id]


# ============== Routes ==============

@app.route('/')
def index():
    """Main workspace UI"""
    return render_template('main.html')


@app.route('/dashboard')
def dashboard():
    """Boss view dashboard"""
    return render_template('dashboard.html')


@app.route('/api/session', methods=['POST'])
def create_session():
    """Create a new workspace session with user context"""
    data = request.json or {}
    user_id = data.get("user_id", "alex")  # Default to alex for demo

    session_id = str(uuid.uuid4().hex[:12])
    session = get_session(session_id, user_id)

    user = get_user(user_id)
    if user:
        session.user_role = user.role

    return jsonify({
        "session_id": session.id,
        "user_id": user_id,
        "user": user.to_dict() if user else None,
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


@app.route('/api/reset', methods=['POST'])
def reset_demo():
    """
    Reset the demo state.
    Clears all work items, handoffs, and notifications from memory.
    Does NOT delete seed data from Memex.
    """
    from services.handoffs import _handoffs, _work_items
    from core.notifications import notifications

    # Clear in-memory state
    _handoffs.clear()
    _work_items.clear()
    notifications._notifications.clear()
    notifications._activity_log.clear()
    sessions.clear()

    return jsonify({
        "status": "reset",
        "message": "Demo state cleared. Work items, handoffs, and notifications reset."
    })


# ============================================
# Multi-User Workflow Endpoints
# ============================================

# --- User Endpoints ---

@app.route('/api/users', methods=['GET'])
def list_users():
    """List all demo users"""
    users = get_all_users()
    return jsonify({
        "users": [u.to_dict() for u in users]
    })


@app.route('/api/users/<user_id>', methods=['GET'])
def get_user_info(user_id):
    """Get a specific user"""
    user = get_user(user_id)
    if not user:
        return jsonify({"error": "User not found"}), 404
    return jsonify(user.to_dict())


@app.route('/api/users/role/<role>', methods=['GET'])
def get_users_for_role(role):
    """Get users with a specific role"""
    users = get_users_by_role(role)
    return jsonify({
        "users": [u.to_dict() for u in users]
    })


# --- Notification Endpoints ---

@app.route('/api/notifications/<user_id>', methods=['GET'])
def get_notifications(user_id):
    """Get all notifications for a user"""
    all_notifs = request.args.get("all", "false").lower() == "true"

    if all_notifs:
        notifs = notifications.get_all(user_id)
    else:
        notifs = notifications.get_pending(user_id)

    return jsonify({
        "notifications": [n.to_dict() for n in notifs],
        "count": len(notifs)
    })


@app.route('/api/notifications/<user_id>/count', methods=['GET'])
def get_notification_count(user_id):
    """Get count of unread notifications"""
    count = notifications.get_count(user_id)
    return jsonify({"count": count})


@app.route('/api/notifications/stream/<user_id>')
def notification_stream(user_id):
    """SSE stream for real-time notifications"""
    def generate():
        for event in notifications.stream(user_id, interval=2.0):
            yield event

    return Response(
        stream_with_context(generate()),
        mimetype='text/event-stream',
        headers={
            'Cache-Control': 'no-cache',
            'X-Accel-Buffering': 'no',
            'Connection': 'keep-alive'
        }
    )


@app.route('/api/notifications/<notification_id>/read', methods=['POST'])
def mark_notification_read(notification_id):
    """Mark a notification as read"""
    data = request.json or {}
    user_id = data.get("user_id")

    if not user_id:
        return jsonify({"error": "user_id required"}), 400

    success = notifications.mark_read(notification_id, user_id)
    return jsonify({"success": success})


@app.route('/api/notifications/<user_id>/read-all', methods=['POST'])
def mark_all_notifications_read(user_id):
    """Mark all notifications as read for a user"""
    count = notifications.mark_all_read(user_id)
    return jsonify({"marked": count})


# --- Work Item Endpoints ---

@app.route('/api/work-items', methods=['POST'])
def create_work_item_endpoint():
    """Create a new work item"""
    data = request.json or {}

    title = data.get("title", "Untitled")
    description = data.get("description", "")
    user_id = data.get("user_id")
    assigned_to = data.get("assigned_to")
    source_input = data.get("source_input")

    if not user_id:
        return jsonify({"error": "user_id required"}), 400

    # Extract anchors from source input if provided
    anchors = []
    if source_input:
        extracted = extract_with_handoff_detection(source_input)
        anchors = extracted.get("anchors", [])

    work_item = create_work_item(
        title=title,
        description=description,
        created_by=user_id,
        assigned_to=assigned_to,
        source_input=source_input,
        anchors=anchors
    )

    return jsonify({
        "work_item": work_item.to_dict(),
        "anchors_extracted": len(anchors)
    })


@app.route('/api/work-items/<work_item_id>', methods=['GET'])
def get_work_item_endpoint(work_item_id):
    """Get a specific work item"""
    work_item = get_work_item(work_item_id)
    if not work_item:
        return jsonify({"error": "Work item not found"}), 404

    # Include handoff chain
    chain = get_handoff_chain(work_item_id)

    return jsonify({
        "work_item": work_item.to_dict(),
        "handoff_chain": chain
    })


@app.route('/api/work-items/user/<user_id>', methods=['GET'])
def get_user_work_items(user_id):
    """Get all work items for a user"""
    pending_only = request.args.get("pending", "false").lower() == "true"

    if pending_only:
        items = get_pending_work_items(user_id)
    else:
        items = get_work_items_for_user(user_id)

    return jsonify({
        "work_items": [w.to_dict() for w in items],
        "count": len(items)
    })


@app.route('/api/work-items/<work_item_id>/status', methods=['PATCH'])
def update_work_item_status_endpoint(work_item_id):
    """Update work item status"""
    data = request.json or {}
    status_str = data.get("status")
    user_id = data.get("user_id")

    if not status_str or not user_id:
        return jsonify({"error": "status and user_id required"}), 400

    try:
        status = WorkItemStatus(status_str)
    except ValueError:
        return jsonify({"error": f"Invalid status: {status_str}"}), 400

    success = update_work_item_status(work_item_id, status, user_id)
    return jsonify({"success": success})


# --- Handoff Endpoints ---

@app.route('/api/handoff', methods=['POST'])
def create_handoff_endpoint():
    """Create a handoff from one user to another"""
    data = request.json or {}

    from_user_id = data.get("from_user_id")
    to_user_id = data.get("to_user_id")
    work_item_id = data.get("work_item_id")
    message = data.get("message", "")
    context = data.get("context", {})

    if not all([from_user_id, to_user_id, work_item_id]):
        return jsonify({"error": "from_user_id, to_user_id, and work_item_id required"}), 400

    handoff = create_handoff(
        from_user_id=from_user_id,
        to_user_id=to_user_id,
        work_item_id=work_item_id,
        message=message,
        context=context
    )

    if not handoff:
        return jsonify({"error": "Failed to create handoff"}), 500

    return jsonify({
        "handoff": handoff.to_dict(),
        "new_work_item_id": handoff.new_work_item_id
    })


@app.route('/api/handoff/<handoff_id>', methods=['GET'])
def get_handoff_endpoint(handoff_id):
    """Get a specific handoff"""
    handoff = get_handoff(handoff_id)
    if not handoff:
        return jsonify({"error": "Handoff not found"}), 404
    return jsonify(handoff.to_dict())


@app.route('/api/handoff/user/<user_id>', methods=['GET'])
def get_user_handoffs(user_id):
    """Get handoffs for a user"""
    pending_only = request.args.get("pending", "false").lower() == "true"

    if pending_only:
        handoffs = get_pending_handoffs(user_id)
    else:
        handoffs = get_handoffs_for_user(user_id)

    return jsonify({
        "handoffs": [h.to_dict() for h in handoffs],
        "count": len(handoffs)
    })


@app.route('/api/handoff/<handoff_id>/accept', methods=['POST'])
def accept_handoff_endpoint(handoff_id):
    """Accept a handoff"""
    data = request.json or {}
    user_id = data.get("user_id")

    if not user_id:
        return jsonify({"error": "user_id required"}), 400

    success = accept_handoff(handoff_id, user_id)
    return jsonify({"success": success})


@app.route('/api/handoff/targets/<from_user_id>', methods=['GET'])
def get_handoff_targets_endpoint(from_user_id):
    """Get valid handoff targets for a user"""
    from_user = get_user(from_user_id)
    if not from_user:
        return jsonify({"error": "User not found"}), 404

    targets = get_handoff_targets(from_user.role)

    # Get actual users for each target role
    result = []
    for target in targets:
        users = get_users_by_role(target["role"])
        for user in users:
            if user.id != from_user_id:
                result.append({
                    "id": user.id,
                    "name": user.name,
                    "role": user.role,
                    "title": user.title,
                    "description": target["description"]
                })

    return jsonify({"targets": result})


# --- Extraction Endpoints ---

@app.route('/api/extract', methods=['POST'])
def extract_anchors_endpoint():
    """Extract anchors from text using a lens"""
    data = request.json or {}
    text = data.get("text", "")
    lens_id = data.get("lens_id", "lens:deal-flow")
    store = data.get("store", False)

    if not text:
        return jsonify({"error": "text required"}), 400

    anchors = extract_anchors(text, lens_id, store_in_memex=store)

    return jsonify({
        "anchors": [a.to_dict() for a in anchors],
        "count": len(anchors)
    })


@app.route('/api/extract/with-handoff', methods=['POST'])
def extract_with_handoff_endpoint():
    """Extract anchors and detect handoff intent"""
    data = request.json or {}
    text = data.get("text", "")
    lens_id = data.get("lens_id", "lens:deal-flow")

    if not text:
        return jsonify({"error": "text required"}), 400

    result = extract_with_handoff_detection(text, lens_id)

    return jsonify({
        "anchors": [a.to_dict() for a in result["anchors"]],
        "handoff": result.get("handoff"),
        "anchor_count": result["anchor_count"],
        "patterns_matched": result["patterns_matched"]
    })


# --- Dashboard Endpoints ---

@app.route('/api/dashboard/graph', methods=['GET'])
def get_dashboard_graph():
    """Get deal flow graph for dashboard visualization"""
    graph = memex.get_deal_flow_graph()
    return jsonify(graph)


@app.route('/api/dashboard/activity', methods=['GET'])
def get_dashboard_activity():
    """Get recent activity for dashboard"""
    limit = int(request.args.get("limit", 50))
    activities = notifications.get_activity_log(limit)
    return jsonify({"activities": activities})


@app.route('/api/dashboard/activity/stream')
def activity_stream():
    """SSE stream for real-time activity updates"""
    def generate():
        for event in notifications.stream_activity(interval=2.0):
            yield event

    return Response(
        stream_with_context(generate()),
        mimetype='text/event-stream',
        headers={
            'Cache-Control': 'no-cache',
            'X-Accel-Buffering': 'no',
            'Connection': 'keep-alive'
        }
    )


@app.route('/api/dashboard/stats', methods=['GET'])
def get_dashboard_stats():
    """Get workflow statistics for dashboard"""
    stats = get_workflow_stats()
    return jsonify(stats)


@app.route('/api/dashboard/work-items', methods=['GET'])
def get_all_work_items_endpoint():
    """Get all work items for dashboard"""
    items = get_all_work_items()
    return jsonify({
        "work_items": [w.to_dict() for w in items],
        "count": len(items)
    })


# --- Lens Endpoints ---

@app.route('/api/lenses', methods=['GET'])
def list_lenses():
    """List all available lenses"""
    lenses = memex.list_lenses()
    return jsonify({"lenses": lenses})


@app.route('/api/lenses/<lens_id>', methods=['GET'])
def get_lens(lens_id):
    """Get a specific lens definition"""
    lens = memex.get_lens(lens_id)
    if not lens:
        return jsonify({"error": "Lens not found"}), 404
    return jsonify(lens)


# --- Role-Aware Input Processing ---

@app.route('/api/input/role-aware', methods=['POST'])
def handle_input_role_aware():
    """
    Process input with role-aware UI generation.
    Uses the user's role to customize the generated interface.
    """
    data = request.json or {}
    session_id = data.get("session_id", "default")
    user_id = data.get("user_id", "alex")
    user_input = data.get("input", "").strip()

    if not user_input:
        return jsonify({"error": "No input provided"}), 400

    session = get_session(session_id, user_id)
    user = get_user(user_id)

    # Record in history
    session.history.append({
        "input": user_input,
        "user_id": user_id,
        "timestamp": datetime.now().isoformat()
    })

    # Extract anchors using lens
    extraction = extract_with_handoff_detection(user_input)
    anchors = extraction.get("anchors", [])
    detected_handoff = extraction.get("handoff")

    # Get context from Memex
    context = memex.get_context_for_input(user_input)

    # Get similar work for additional context
    similar_work = memex.get_similar_work(user_input, limit=3)

    # Classify intent with role context
    intent = llm.classify_intent(user_input, context)

    # Get role-specific configuration
    role_config = get_role_config(user.role if user else "cs")

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

    # Always create work item for tracking (needed for handoffs)
    work_item = create_work_item(
        title=intent.get("title", user_input[:50]),
        description=intent.get("summary", user_input),
        created_by=user_id,
        assigned_to=user_id,
        source_input=user_input,
        anchors=anchors,
        stage=get_stage_for_role(user.role) if user else "pending"
    )

    # Create ViewSpec
    view_spec = ViewSpec(
        title=intent.get("title"),
        source_input=user_input,
        context={
            "intent": intent,
            "memex_context": context,
            "similar_work": similar_work,
            "anchors": [a.to_dict() for a in anchors],
            "role": user.role if user else None
        }
    )
    session.items.append(view_spec)
    session.active_item_id = view_spec.id

    return jsonify({
        "html": full_html,
        "view_spec_id": view_spec.id,
        "work_item_id": work_item.id,
        "intent": intent,
        "anchors": [a.to_dict() for a in anchors],
        "detected_handoff": detected_handoff,
        "similar_work": similar_work,
        "context_count": len(context),
        "components": components_data,
        "handoff_targets": get_handoff_targets(user.role) if user else []
    })


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
