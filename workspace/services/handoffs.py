"""
Handoff workflow logic for Memex Workspace.

Manages the transfer of work items between users,
creating proper graph links and notifications.
"""

from datetime import datetime
from typing import Dict, Any, List, Optional
from dataclasses import dataclass

from core.types import Handoff, WorkItem, WorkItemStatus, Anchor, NotificationType
from core.notifications import notifications
from services.memex import memex
from services.users import get_user, DEMO_USERS


# In-memory storage for demo (use database in production)
_handoffs: Dict[str, Handoff] = {}
_work_items: Dict[str, WorkItem] = {}


def create_work_item(
    title: str,
    description: str,
    created_by: str,
    assigned_to: Optional[str] = None,
    stage: str = "pending",
    source_input: Optional[str] = None,
    anchors: List[Anchor] = None,
    context: Dict[str, Any] = None,
    parent_work_item_id: Optional[str] = None
) -> WorkItem:
    """
    Create a new work item and store it in Memex.

    Returns the created WorkItem.
    """
    work_item = WorkItem(
        title=title,
        description=description,
        status=WorkItemStatus.PENDING,
        stage=stage,
        assigned_to=assigned_to or created_by,
        created_by=created_by,
        source_input=source_input,
        anchors=anchors or [],
        context=context or {},
        parent_work_item_id=parent_work_item_id
    )

    # Store in Memex
    memex_id = memex.create_node(
        node_type="WorkItem",
        meta={
            "title": title,
            "description": description,
            "status": work_item.status.value,
            "stage": stage,
            "assigned_to": work_item.assigned_to,
            "created_by": created_by,
            "source_input": source_input,
            "anchors": [a.to_dict() for a in (anchors or [])],
            "context": context or {}
        },
        node_id=work_item.id
    )

    work_item.memex_node_id = memex_id

    # Create links in Memex
    if memex_id:
        # Link to creator
        memex.create_link(
            source=memex_id,
            target=f"user:{created_by}",
            link_type="CREATED_BY"
        )

        # Link to assignee
        if assigned_to:
            memex.create_link(
                source=memex_id,
                target=f"user:{assigned_to}",
                link_type="ASSIGNED_TO"
            )

        # Link to parent if this is from a handoff
        if parent_work_item_id:
            memex.create_link(
                source=parent_work_item_id,
                target=memex_id,
                link_type="HANDOFF"
            )

        # Link to anchors
        for anchor in (anchors or []):
            if anchor.id:
                memex.create_link(
                    source=memex_id,
                    target=anchor.id,
                    link_type="REFERENCES"
                )

    # Store in local cache
    _work_items[work_item.id] = work_item

    # Log activity
    notifications.log_activity(
        activity_type="work_item_created",
        user_id=created_by,
        title=f"Created: {title}",
        details={"work_item_id": work_item.id, "stage": stage}
    )

    return work_item


def get_work_item(work_item_id: str) -> Optional[WorkItem]:
    """Get a work item by ID"""
    return _work_items.get(work_item_id)


def get_work_items_for_user(user_id: str) -> List[WorkItem]:
    """Get all work items assigned to a user"""
    return [w for w in _work_items.values() if w.assigned_to == user_id]


def get_pending_work_items(user_id: str) -> List[WorkItem]:
    """Get pending work items for a user"""
    return [
        w for w in _work_items.values()
        if w.assigned_to == user_id and w.status == WorkItemStatus.PENDING
    ]


def create_handoff(
    from_user_id: str,
    to_user_id: str,
    work_item_id: str,
    message: str = "",
    context: Dict[str, Any] = None
) -> Optional[Handoff]:
    """
    Create a handoff from one user to another.

    This:
    1. Creates a new work item for the recipient
    2. Links the work items in Memex with HANDOFF relationship
    3. Sends a notification to the recipient
    4. Logs the activity

    Returns the Handoff object.
    """
    # Get the source work item
    source_item = get_work_item(work_item_id)
    if not source_item:
        print(f"Work item not found: {work_item_id}")
        return None

    # Get user info
    from_user = get_user(from_user_id)
    to_user = get_user(to_user_id)

    if not from_user or not to_user:
        print(f"User not found: from={from_user_id}, to={to_user_id}")
        return None

    # Determine new stage based on recipient's role
    stage_mapping = {
        "sales": "closed",
        "cs": "onboarding",
        "engineering": "implementation",
        "manager": "review"
    }
    new_stage = stage_mapping.get(to_user.role, "pending")

    # Create new work item for recipient
    new_title = f"{source_item.title}"
    if message:
        new_title = f"{source_item.title} - {message[:50]}"

    new_work_item = create_work_item(
        title=new_title,
        description=source_item.description or message,
        created_by=from_user_id,
        assigned_to=to_user_id,
        stage=new_stage,
        source_input=source_item.source_input,
        anchors=source_item.anchors,
        context={
            **source_item.context,
            **(context or {}),
            "handoff_from": from_user_id,
            "handoff_message": message,
            "previous_stage": source_item.stage
        },
        parent_work_item_id=work_item_id
    )

    # Create handoff record
    handoff = Handoff(
        from_user_id=from_user_id,
        to_user_id=to_user_id,
        work_item_id=work_item_id,
        new_work_item_id=new_work_item.id,
        message=message,
        context=context or {}
    )

    # Update source work item
    source_item.status = WorkItemStatus.COMPLETE
    source_item.handoff_chain.append(handoff.id)

    # Store handoff
    _handoffs[handoff.id] = handoff

    # Create HANDOFF link in Memex (additional metadata)
    if source_item.memex_node_id and new_work_item.memex_node_id:
        memex.create_link(
            source=source_item.memex_node_id,
            target=new_work_item.memex_node_id,
            link_type="HANDOFF",
            meta={
                "from_user": from_user_id,
                "to_user": to_user_id,
                "message": message,
                "timestamp": datetime.now().isoformat()
            }
        )

    # Send notification to recipient
    notifications.notify_handoff(
        from_user_id=from_user_id,
        to_user_id=to_user_id,
        work_item_id=new_work_item.id,
        title=f"New work from {from_user.name}",
        message=f"{source_item.title}: {message}" if message else source_item.title
    )

    # Log activity
    notifications.log_activity(
        activity_type="handoff",
        user_id=from_user_id,
        title=f"Handed off to {to_user.name}",
        details={
            "from_user": from_user_id,
            "to_user": to_user_id,
            "work_item": new_work_item.title,
            "stage": new_stage
        }
    )

    return handoff


def get_handoff(handoff_id: str) -> Optional[Handoff]:
    """Get a handoff by ID"""
    return _handoffs.get(handoff_id)


def get_handoffs_for_user(user_id: str) -> List[Handoff]:
    """Get all handoffs to a user"""
    return [h for h in _handoffs.values() if h.to_user_id == user_id]


def get_pending_handoffs(user_id: str) -> List[Handoff]:
    """Get pending (unaccepted) handoffs for a user"""
    return [
        h for h in _handoffs.values()
        if h.to_user_id == user_id and not h.accepted
    ]


def accept_handoff(handoff_id: str, user_id: str) -> bool:
    """Accept a handoff"""
    handoff = get_handoff(handoff_id)
    if not handoff or handoff.to_user_id != user_id:
        return False

    handoff.accepted = True
    handoff.accepted_at = datetime.now()

    # Update work item status
    work_item = get_work_item(handoff.new_work_item_id)
    if work_item:
        work_item.status = WorkItemStatus.IN_PROGRESS

    # Log activity
    user = get_user(user_id)
    notifications.log_activity(
        activity_type="handoff_accepted",
        user_id=user_id,
        title=f"{user.name if user else user_id} accepted handoff",
        details={"handoff_id": handoff_id}
    )

    return True


def update_work_item_status(
    work_item_id: str,
    status: WorkItemStatus,
    user_id: str
) -> bool:
    """Update the status of a work item"""
    work_item = get_work_item(work_item_id)
    if not work_item:
        return False

    work_item.status = status
    work_item.updated = datetime.now()

    # Update in Memex
    if work_item.memex_node_id:
        memex._patch(f"/api/nodes/{work_item.memex_node_id}", {
            "meta": {"status": status.value}
        })

    # Log activity
    user = get_user(user_id)
    status_names = {
        WorkItemStatus.PENDING: "marked pending",
        WorkItemStatus.IN_PROGRESS: "started working on",
        WorkItemStatus.BLOCKED: "marked as blocked",
        WorkItemStatus.COMPLETE: "completed",
        WorkItemStatus.CANCELLED: "cancelled"
    }

    notifications.log_activity(
        activity_type="status_change",
        user_id=user_id,
        title=f"{user.name if user else user_id} {status_names.get(status, 'updated')} {work_item.title}",
        details={"work_item_id": work_item_id, "new_status": status.value}
    )

    # Notify relevant users
    if status == WorkItemStatus.COMPLETE:
        # Notify creator that work is done
        if work_item.created_by and work_item.created_by != user_id:
            notifications.notify(
                to_user_id=work_item.created_by,
                notification_type=NotificationType.COMPLETE,
                title=f"{work_item.title} completed",
                message=f"Completed by {user.name if user else user_id}",
                from_user_id=user_id,
                work_item_id=work_item_id
            )

    return True


def get_handoff_chain(work_item_id: str) -> List[Dict[str, Any]]:
    """
    Get the full handoff chain for a work item.
    Traces back through parent work items.
    """
    chain = []
    current_id = work_item_id

    while current_id:
        work_item = get_work_item(current_id)
        if not work_item:
            break

        user = get_user(work_item.assigned_to) if work_item.assigned_to else None

        chain.append({
            "work_item_id": work_item.id,
            "title": work_item.title,
            "stage": work_item.stage,
            "status": work_item.status.value,
            "assigned_to": work_item.assigned_to,
            "assigned_to_name": user.name if user else None,
            "assigned_to_role": user.role if user else None,
            "created": work_item.created.isoformat()
        })

        current_id = work_item.parent_work_item_id

    # Reverse to show oldest first
    return list(reversed(chain))


def get_all_work_items() -> List[WorkItem]:
    """Get all work items (for dashboard)"""
    return list(_work_items.values())


def get_workflow_stats() -> Dict[str, Any]:
    """Get workflow statistics for dashboard"""
    items = list(_work_items.values())

    by_status = {}
    by_stage = {}
    by_user = {}

    for item in items:
        # Count by status
        status = item.status.value
        by_status[status] = by_status.get(status, 0) + 1

        # Count by stage
        stage = item.stage or "unknown"
        by_stage[stage] = by_stage.get(stage, 0) + 1

        # Count by user
        user = item.assigned_to or "unassigned"
        by_user[user] = by_user.get(user, 0) + 1

    return {
        "total": len(items),
        "by_status": by_status,
        "by_stage": by_stage,
        "by_user": by_user,
        "handoffs_total": len(_handoffs)
    }
