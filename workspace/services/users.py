"""
Simple user management for Memex Workspace demo.

No authentication - just role selection for the VC demo.
In production, this would integrate with a real auth system.
"""

from typing import Dict, Optional, List
from dataclasses import dataclass


@dataclass
class DemoUser:
    """A demo user for the workspace"""
    id: str
    name: str
    role: str
    title: str
    email: str


# Demo users for the VC presentation
DEMO_USERS: Dict[str, DemoUser] = {
    "alex": DemoUser(
        id="alex",
        name="Alex",
        role="sales",
        title="Sales Rep",
        email="alex@company.com"
    ),
    "jordan": DemoUser(
        id="jordan",
        name="Jordan",
        role="cs",
        title="Customer Success Manager",
        email="jordan@company.com"
    ),
    "sam": DemoUser(
        id="sam",
        name="Sam",
        role="engineering",
        title="Solutions Engineer",
        email="sam@company.com"
    ),
    "morgan": DemoUser(
        id="morgan",
        name="Morgan",
        role="manager",
        title="VP Operations",
        email="morgan@company.com"
    )
}


# Role descriptions for UI and prompts
ROLE_INFO = {
    "sales": {
        "name": "Sales",
        "description": "Close deals and hand off to Customer Success",
        "primary_actions": ["Close Deal", "Forward to CS"],
        "can_handoff_to": ["cs"]
    },
    "cs": {
        "name": "Customer Success",
        "description": "Onboard customers and coordinate implementation",
        "primary_actions": ["Start Onboarding", "Forward to Engineering"],
        "can_handoff_to": ["engineering", "sales"]
    },
    "engineering": {
        "name": "Solutions Engineering",
        "description": "Implement technical requirements",
        "primary_actions": ["Start Implementation", "Mark Complete"],
        "can_handoff_to": ["cs"]
    },
    "manager": {
        "name": "Manager",
        "description": "Oversee deal pipeline and team activity",
        "primary_actions": ["View Dashboard", "Review Status"],
        "can_handoff_to": ["sales", "cs", "engineering"]
    }
}


def get_user(user_id: str) -> Optional[DemoUser]:
    """Get a user by ID"""
    return DEMO_USERS.get(user_id)


def get_all_users() -> List[DemoUser]:
    """Get all demo users"""
    return list(DEMO_USERS.values())


def get_users_by_role(role: str) -> List[DemoUser]:
    """Get all users with a specific role"""
    return [u for u in DEMO_USERS.values() if u.role == role]


def get_role_info(role: str) -> Optional[Dict]:
    """Get role information"""
    return ROLE_INFO.get(role)


def get_handoff_targets(from_user_id: str) -> List[DemoUser]:
    """Get users that this user can hand off work to"""
    user = get_user(from_user_id)
    if not user:
        return []

    role_info = get_role_info(user.role)
    if not role_info:
        return []

    target_roles = role_info.get("can_handoff_to", [])
    targets = []

    for target_role in target_roles:
        targets.extend(get_users_by_role(target_role))

    return targets


def to_dict(user: DemoUser) -> Dict:
    """Convert user to dictionary"""
    return {
        "id": user.id,
        "name": user.name,
        "role": user.role,
        "title": user.title,
        "email": user.email
    }


def get_all_users_dict() -> List[Dict]:
    """Get all users as dictionaries"""
    return [to_dict(u) for u in get_all_users()]


def get_handoff_targets_dict(from_user_id: str) -> List[Dict]:
    """Get handoff targets as dictionaries"""
    return [to_dict(u) for u in get_handoff_targets(from_user_id)]
