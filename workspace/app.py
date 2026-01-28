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
    """Redirect to Journal - the main entry point"""
    from flask import redirect
    return redirect('/journal')


@app.route('/journal')
def journal():
    """Journal - Write prose, build knowledge graph automatically"""
    return render_template('apps/journal.html')


@app.route('/dashboard')
def dashboard():
    """Boss view dashboard"""
    return render_template('dashboard.html')


# ============== Memex-Native Productivity Suite Routes ==============

@app.route('/home')
def graph_home():
    """Graph Home - Knowledge graph visualization and navigation"""
    return render_template('apps/home.html')


@app.route('/docs')
def docs_new():
    """Create a new document"""
    return render_template('apps/docs.html', doc_id=None)


@app.route('/docs/<doc_id>')
def docs_edit(doc_id):
    """Edit an existing document"""
    return render_template('apps/docs.html', doc_id=doc_id)


@app.route('/sheets')
def sheets_new():
    """Create a new spreadsheet"""
    return render_template('apps/sheets.html', sheet_id=None)


@app.route('/sheets/<sheet_id>')
def sheets_edit(sheet_id):
    """Edit an existing spreadsheet"""
    return render_template('apps/sheets.html', sheet_id=sheet_id)


# ============== Node API Endpoints ==============

@app.route('/api/nodes', methods=['GET'])
def get_nodes():
    """Get all nodes for the knowledge graph"""
    try:
        all_nodes = []
        all_links = []
        seen_ids = set()

        # Search for different types of content
        # Use broader search terms to find more nodes
        search_queries = [
            ("*", ["Document"]),
            ("*", ["Data"]),
            ("*", ["Project"]),
            ("*", ["Person"]),
            ("document", None),
            ("sheet", None),
            ("project", None),
        ]

        for query, types in search_queries:
            try:
                results = memex.search(query, limit=50, types=types)
                for node in results:
                    if node.id not in seen_ids:
                        seen_ids.add(node.id)
                        all_nodes.append({
                            "id": node.id,
                            "type": node.type,
                            "title": node.meta.get("title") or node.meta.get("name", "Untitled"),
                            "created": node.meta.get("created"),
                            "updated": node.meta.get("updated")
                        })
            except Exception as e:
                print(f"[get_nodes] Search error for {query}: {e}")

        # Get links between nodes
        for node in all_nodes:
            try:
                node_links = memex.get_node_links(node["id"])
                for link in node_links:
                    all_links.append({
                        "source": link.get("source"),
                        "target": link.get("target"),
                        "type": link.get("type")
                    })
            except Exception as e:
                pass  # Skip link errors silently

        return jsonify({
            "nodes": all_nodes,
            "links": all_links
        })
    except Exception as e:
        print(f"[get_nodes] Error: {e}")
        return jsonify({"nodes": [], "links": []})


@app.route('/api/docs', methods=['POST'])
def create_doc():
    """Create a new document node"""
    data = request.json or {}
    title = data.get("title", "Untitled Document")
    content = data.get("content", {})

    try:
        doc_id = memex.create_node(
            node_type="Document",
            meta={
                "title": title,
                "content": content,
                "content_type": "doc",
                "owner": data.get("owner", "anonymous")
            }
        )
        return jsonify({"id": doc_id, "title": title})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/docs/<doc_id>', methods=['GET'])
def get_doc(doc_id):
    """Get a document by ID"""
    try:
        doc = memex.get_node(doc_id)
        if doc:
            return jsonify({
                "id": doc.id,
                "title": doc.meta.get("title", "Untitled"),
                "content": doc.meta.get("content", {}),
                "links": memex.get_node_links(doc_id),
                "created": doc.meta.get("created"),
                "updated": doc.meta.get("updated")
            })
        return jsonify({"error": "Document not found"}), 404
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/docs/<doc_id>', methods=['PUT'])
def update_doc(doc_id):
    """Update a document"""
    data = request.json or {}

    try:
        memex._patch(f"/api/nodes/{doc_id}", {
            "meta": {
                "title": data.get("title"),
                "content": data.get("content")
            }
        })
        return jsonify({"success": True, "id": doc_id})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/sheets', methods=['POST'])
def create_sheet():
    """Create a new spreadsheet node"""
    data = request.json or {}
    title = data.get("title", "Untitled Spreadsheet")
    sheet_data = data.get("data", {"columns": [], "rows": []})

    try:
        sheet_id = memex.create_node(
            node_type="Data",
            meta={
                "title": title,
                "data": sheet_data,
                "content_type": "sheet",
                "owner": data.get("owner", "anonymous")
            }
        )
        return jsonify({"id": sheet_id, "title": title})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/sheets/<sheet_id>', methods=['GET'])
def get_sheet(sheet_id):
    """Get a spreadsheet by ID"""
    try:
        sheet = memex.get_node(sheet_id)
        if sheet:
            return jsonify({
                "id": sheet.id,
                "title": sheet.meta.get("title", "Untitled"),
                "data": sheet.meta.get("data", {"columns": [], "rows": []}),
                "links": memex.get_node_links(sheet_id),
                "created": sheet.meta.get("created"),
                "updated": sheet.meta.get("updated")
            })
        return jsonify({"error": "Spreadsheet not found"}), 404
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/sheets/<sheet_id>', methods=['PUT'])
def update_sheet(sheet_id):
    """Update a spreadsheet"""
    data = request.json or {}

    try:
        memex._patch(f"/api/nodes/{sheet_id}", {
            "meta": {
                "title": data.get("title"),
                "data": data.get("data")
            }
        })
        return jsonify({"success": True, "id": sheet_id})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/links', methods=['POST'])
def create_link():
    """Create a link between nodes"""
    data = request.json or {}
    source = data.get("source")
    target = data.get("target")
    link_type = data.get("type", "REFERENCES")

    if not source or not target:
        return jsonify({"error": "source and target required"}), 400

    try:
        link_id = memex.create_link(
            source=source,
            target=target,
            link_type=link_type,
            meta=data.get("meta", {})
        )
        return jsonify({"id": link_id})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/context/<node_id>', methods=['GET'])
def get_node_context(node_id):
    """Get context for a node - linked nodes and related content"""
    try:
        context = memex.get_context_for_input(node_id)
        links = memex.get_node_links(node_id)

        # Get linked node details
        linked_nodes = []
        for link in links:
            target_id = link.get("target") if link.get("source") == node_id else link.get("source")
            target_node = memex.get_node(target_id)
            if target_node:
                linked_nodes.append({
                    "id": target_id,
                    "type": target_node.type,
                    "title": target_node.meta.get("title") or target_node.meta.get("name"),
                    "link_type": link.get("type")
                })

        return jsonify({
            "context": context,
            "linked_nodes": linked_nodes,
            "links": links
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500


# ============== Journal API Endpoints ==============

# In-memory storage for journal entries (use DB in production)
_journal_entries = []


@app.route('/api/journal/extract', methods=['POST'])
def journal_extract():
    """Extract entities from prose text using pattern matching"""
    import re
    data = request.json or {}
    text = data.get("text", "")

    if not text:
        return jsonify({"entities": []})

    entities = []
    seen_values = set()

    # Skip common words that aren't entities
    skip_words = {'I', 'We', 'They', 'He', 'She', 'It', 'The', 'This', 'That',
                  'Today', 'Tomorrow', 'Yesterday', 'Monday', 'Tuesday',
                  'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'}

    def add_entity(etype, value):
        value = value.strip()
        if value and len(value) > 2 and value not in seen_values and value not in skip_words:
            seen_values.add(value)
            entities.append({"type": etype, "value": value})

    # Companies: Names followed by Corp, Inc, Ltd, or common business suffixes
    company_patterns = [
        r'\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)\s+(?:Corp|Inc|Ltd|LLC|Co)\.?\b',
        r'\b([A-Z][A-Za-z]+)\s+(?:deal|project|account|client)\b',
    ]
    for pattern in company_patterns:
        matches = re.findall(pattern, text)
        for match in matches:
            add_entity("company", match)

    # People: Names that appear with context clues (but not company names)
    person_patterns = [
        r'\bwith\s+([A-Z][a-z]+)\b(?!\s+(?:Corp|Inc|Ltd|LLC|Co|deal|project))',
        r'\b([A-Z][a-z]+)\s+(?:said|mentioned|asked|told|needs|wants|is|was)\b',
        r'\b(?:loop(?:ing)?\s+in|cc|contact|reach out to|email|call)\s+([A-Z][a-z]+)\b',
        r'\b(?:from|to|by)\s+([A-Z][a-z]+)\b(?!\s+(?:Corp|Inc|Ltd|LLC|Co))',
    ]
    for pattern in person_patterns:
        matches = re.findall(pattern, text)
        for match in matches:
            # Don't add if already a company
            if match not in [e['value'] for e in entities if e['type'] == 'company']:
                add_entity("person", match)

    # Money amounts
    money_pattern = r'\$[\d,]+(?:\.\d{2})?k?|\b\d+k\b'
    money_matches = re.findall(money_pattern, text, re.IGNORECASE)
    for match in money_matches:
        add_entity("money", match)

    # Dates: Various date formats and relative dates
    date_patterns = [
        r'\bby\s+(Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday)\b',
        r'\bby\s+(tomorrow|next week|end of (?:day|week|month))\b',
        r'\bon\s+(Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday)\b',
        r'\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2}(?:st|nd|rd|th)?\b',
    ]
    for pattern in date_patterns:
        matches = re.findall(pattern, text, re.IGNORECASE)
        for match in matches:
            add_entity("date", match)

    # Tasks: Action items and to-dos
    task_patterns = [
        r'\bneed to\s+([^.!?,]+)',
        r'\bwill\s+([^.!?,]+)',
        r'\bshould\s+([^.!?,]+)',
    ]
    for pattern in task_patterns:
        matches = re.findall(pattern, text, re.IGNORECASE)
        for match in matches:
            task = match.strip()[:50]
            if len(task) > 5 and not any(task == e['value'] for e in entities):
                add_entity("task", task)
                break

    return jsonify({"entities": entities[:12]})


@app.route('/api/journal/save', methods=['POST'])
def journal_save():
    """Save journal entry and create nodes/links in Memex"""
    data = request.json or {}
    text = data.get("text", "")
    entities = data.get("entities", [])

    if not text:
        return jsonify({"error": "No text provided"}), 400

    entry_id = str(uuid.uuid4().hex[:12])
    now = datetime.now().isoformat()

    # Create the journal entry node in Memex
    try:
        journal_node_id = memex.create_node(
            node_type="Journal",
            meta={
                "title": text[:50] + "..." if len(text) > 50 else text,
                "content": text,
                "entities": entities,
                "created": now
            }
        )
    except Exception as e:
        print(f"[journal_save] Error creating journal node: {e}")
        journal_node_id = entry_id

    nodes_created = 1  # The journal entry itself
    links_created = 0

    # Create nodes for extracted entities and link them
    entity_nodes = {}
    for entity in entities:
        etype = entity.get("type")
        value = entity.get("value")

        if not value:
            continue

        # Map entity types to Memex node types
        node_type_map = {
            "person": "Person",
            "company": "Company",
            "task": "Task",
            "project": "Project",
            "date": "Event",
            "money": "Amount"
        }

        memex_type = node_type_map.get(etype, "Entity")

        # Check if entity already exists
        try:
            existing = memex.search(value, limit=1, types=[memex_type])
            if existing and len(existing) > 0:
                entity_node_id = existing[0].id
            else:
                # Create new entity node
                entity_node_id = memex.create_node(
                    node_type=memex_type,
                    meta={
                        "name": value,
                        "title": value,
                        "created": now
                    }
                )
                nodes_created += 1
        except Exception as e:
            print(f"[journal_save] Error with entity {value}: {e}")
            entity_node_id = None

        if entity_node_id:
            entity_nodes[value] = entity_node_id

            # Create link from journal entry to entity
            try:
                link_type_map = {
                    "person": "MENTIONS",
                    "company": "ABOUT",
                    "task": "CONTAINS_TASK",
                    "project": "RELATED_TO",
                    "date": "SCHEDULED",
                    "money": "HAS_AMOUNT"
                }
                link_type = link_type_map.get(etype, "REFERENCES")

                memex.create_link(
                    source=journal_node_id,
                    target=entity_node_id,
                    link_type=link_type,
                    meta={"extracted_from": "journal", "created": now}
                )
                links_created += 1
            except Exception as e:
                print(f"[journal_save] Error creating link: {e}")

    # Store entry in memory for timeline
    entry = {
        "id": entry_id,
        "node_id": journal_node_id,
        "text": text,
        "entities": entities,
        "created": now
    }
    _journal_entries.insert(0, entry)

    return jsonify({
        "success": True,
        "entry_id": entry_id,
        "node_id": journal_node_id,
        "nodes_created": nodes_created,
        "links_created": links_created
    })


@app.route('/api/journal/entries', methods=['GET'])
def journal_entries():
    """Get past journal entries"""
    limit = int(request.args.get("limit", 20))

    # First try to get from Memex
    try:
        results = memex.search("*", limit=limit, types=["Journal"])
        entries = []
        for node in results:
            entries.append({
                "id": node.id,
                "text": node.meta.get("content", node.meta.get("title", "")),
                "entities": node.meta.get("entities", []),
                "created": node.meta.get("created")
            })
        if entries:
            return jsonify({"entries": entries})
    except Exception as e:
        print(f"[journal_entries] Memex search error: {e}")

    # Fall back to in-memory entries
    return jsonify({"entries": _journal_entries[:limit]})


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


# ============================================
# Email Inbox Endpoints
# ============================================

@app.route('/inbox')
def inbox():
    """Inbox - Email list view"""
    return render_template('apps/inbox.html')


@app.route('/inbox/<email_id>')
def email_detail(email_id):
    """Email detail view with anchors"""
    return render_template('apps/email.html', email_id=email_id)


@app.route('/api/emails', methods=['GET'])
def list_emails():
    """List emails with pagination and search"""
    page = int(request.args.get("page", 1))
    limit = int(request.args.get("limit", 20))
    query = request.args.get("q", "")

    try:
        # Search for Email nodes in Memex
        if query:
            results = memex.search(query, limit=limit * 2, types=["Email"])
        else:
            results = memex.search("*", limit=limit * 2, types=["Email"])

        # Sort by date descending
        emails = []
        for node in results:
            meta = node.meta
            emails.append({
                "id": node.id,
                "subject": meta.get("subject", "(No subject)"),
                "from_name": meta.get("from_name", ""),
                "from_email": meta.get("from_email", ""),
                "to": meta.get("to", []),
                "date": meta.get("date"),
                "body_preview": meta.get("body_preview", "")[:200],
                "thread_id": meta.get("thread_id"),
                "processed": meta.get("processed", False),
                "anchor_count": meta.get("anchor_count", 0),
                "unread": not meta.get("read", False)
            })

        # Sort by date
        emails.sort(key=lambda e: e.get("date") or "", reverse=True)

        # Paginate
        start = (page - 1) * limit
        end = start + limit
        paginated = emails[start:end]

        return jsonify({
            "emails": paginated,
            "total": len(emails),
            "page": page,
            "limit": limit
        })

    except Exception as e:
        print(f"[list_emails] Error: {e}")
        return jsonify({"emails": [], "total": 0, "error": str(e)})


@app.route('/api/emails/<email_id>', methods=['GET'])
def get_email(email_id):
    """Get a single email with its anchors"""
    try:
        # Get email node
        email_node = memex.get_node(email_id)
        if not email_node:
            return jsonify({"error": "Email not found"}), 404

        meta = email_node.meta

        # Get full body from source node if available
        body = meta.get("body_preview", "")
        source_id = meta.get("source_id")
        if source_id:
            source_node = memex.get_node(source_id)
            if source_node and source_node.meta.get("content"):
                content = source_node.meta.get("content", "")
                if content.startswith("Subject:"):
                    body = content.split("\n\n", 1)[-1]
                else:
                    body = content

        # Get anchors linked to this email
        anchors = []
        try:
            links = memex.get_node_links(email_id)
            for link in links:
                if link.get("type") == "EXTRACTED_FROM":
                    anchor_id = link.get("source")
                    if anchor_id and anchor_id != email_id:
                        anchor_node = memex.get_node(anchor_id)
                        if anchor_node:
                            anchors.append({
                                "id": anchor_node.id,
                                "type": anchor_node.type.lower(),
                                "text": anchor_node.meta.get("text", ""),
                                "start": anchor_node.meta.get("start", 0),
                                "end": anchor_node.meta.get("end", 0),
                                "zone": anchor_node.meta.get("zone", "body"),
                                "properties": anchor_node.meta.get("properties", {}),
                                "matched_patterns": anchor_node.meta.get("matched_patterns", []),
                                "confidence": anchor_node.meta.get("confidence", 0.8)
                            })
        except Exception as e:
            print(f"[get_email] Error getting anchors: {e}")

        email_data = {
            "id": email_id,
            "subject": meta.get("subject", "(No subject)"),
            "from": {
                "name": meta.get("from_name", ""),
                "address": meta.get("from_email", "")
            },
            "to": meta.get("to", []),
            "cc": meta.get("cc", []),
            "date": meta.get("date"),
            "body": body,
            "body_preview": meta.get("body_preview", ""),
            "thread_id": meta.get("thread_id"),
            "in_reply_to": meta.get("in_reply_to"),
            "processed": meta.get("processed", False)
        }

        return jsonify({
            "email": email_data,
            "anchors": anchors
        })

    except Exception as e:
        print(f"[get_email] Error: {e}")
        return jsonify({"error": str(e)}), 500


@app.route('/api/emails/<email_id>/reprocess', methods=['POST'])
def reprocess_email(email_id):
    """Re-run extraction on an email"""
    from services.email_extractor import email_extractor

    try:
        result = email_extractor.reprocess_email(email_id)
        return jsonify(result)
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route('/api/webhooks/extract-email', methods=['POST'])
def extract_email_webhook():
    """Webhook endpoint for email extraction subscription"""
    from services.email_extractor import email_extractor

    data = request.json or {}

    try:
        result = email_extractor.handle_webhook(data)
        return jsonify(result)
    except Exception as e:
        print(f"[extract_email_webhook] Error: {e}")
        return jsonify({"status": "error", "reason": str(e)}), 500


@app.route('/api/anchors/<anchor_id>/connections', methods=['GET'])
def get_anchor_connections(anchor_id):
    """Get related nodes for an anchor"""
    try:
        connections = []

        # Get links from the anchor
        links = memex.get_node_links(anchor_id)

        for link in links:
            # Get the other node in the link
            other_id = link.get("target") if link.get("source") == anchor_id else link.get("source")
            if not other_id:
                continue

            # Skip the email itself
            if other_id.startswith("email:") or other_id.startswith("sha256:"):
                continue

            # Skip lens nodes
            if other_id.startswith("lens:"):
                continue

            other_node = memex.get_node(other_id)
            if other_node:
                connections.append({
                    "id": other_id,
                    "type": other_node.type,
                    "title": other_node.meta.get("text") or other_node.meta.get("title") or other_node.meta.get("name") or other_id,
                    "link_type": link.get("type", "RELATED_TO")
                })

        return jsonify({"connections": connections})

    except Exception as e:
        print(f"[get_anchor_connections] Error: {e}")
        return jsonify({"connections": [], "error": str(e)})


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
