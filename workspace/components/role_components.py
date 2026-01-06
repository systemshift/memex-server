"""
Role-specific component configurations for Memex Workspace.

Each role gets a tailored UI experience with appropriate components
and context focus based on their workflow needs.
"""

from typing import Dict, List, Any, Optional
from dataclasses import dataclass, field


@dataclass
class RoleConfig:
    """Configuration for a user role's UI generation"""
    role: str
    primary_view: str
    components: List[str]
    context_focus: List[str]
    ui_hints: str
    stage_transitions: Dict[str, str]  # stage -> next_stage


# Role-specific component sets and configurations
ROLE_COMPONENT_SETS: Dict[str, RoleConfig] = {
    "sales": RoleConfig(
        role="sales",
        primary_view="deal_summary",
        components=[
            "form_header",
            "text_field",
            "currency_field",
            "select_field",
            "date_field",
            "context_card",
            "handoff_form",
            "action_bar"
        ],
        context_focus=["company", "amount", "contact", "deal_history", "similar_deals"],
        ui_hints="""Generate a deal summary form optimized for sales.
Focus on: company name, deal value, key requirements, customer contact.
Include context cards showing similar past deals.
Always include a handoff form to forward to Customer Success.""",
        stage_transitions={
            "closed": "onboarding",
            "negotiation": "closed"
        }
    ),

    "cs": RoleConfig(
        role="cs",
        primary_view="onboarding_checklist",
        components=[
            "form_header",
            "checklist",
            "text_field",
            "textarea_field",
            "date_field",
            "select_field",
            "context_card",
            "handoff_chain",
            "handoff_form",
            "action_bar"
        ],
        context_focus=["requirements", "timeline", "blockers", "customer_history", "onboarding_steps"],
        ui_hints="""Generate an onboarding checklist view for Customer Success.
Focus on: onboarding tasks, customer requirements, timeline.
Show the handoff chain so CS knows the deal history.
Include context cards with customer background and requirements.
Include handoff form to forward technical work to Engineering.""",
        stage_transitions={
            "onboarding": "implementation",
            "pending": "onboarding"
        }
    ),

    "engineering": RoleConfig(
        role="engineering",
        primary_view="technical_spec",
        components=[
            "form_header",
            "textarea_field",
            "checklist",
            "text_field",
            "select_field",
            "context_card",
            "handoff_chain",
            "action_bar"
        ],
        context_focus=["technical_requirements", "past_issues", "dependencies", "implementation_patterns"],
        ui_hints="""Generate a technical specification view for Engineering.
Focus on: technical requirements, implementation details, blockers.
MUST show context cards with past similar implementations and known issues.
Show the full handoff chain so engineer understands the customer journey.
Include timeline and any technical constraints.""",
        stage_transitions={
            "implementation": "complete",
            "blocked": "implementation"
        }
    ),

    "manager": RoleConfig(
        role="manager",
        primary_view="overview",
        components=[
            "form_header",
            "stats_card",
            "activity_feed",
            "team_status",
            "context_card",
            "action_bar"
        ],
        context_focus=["pipeline_status", "team_workload", "blockers", "deadlines"],
        ui_hints="""Generate an overview dashboard for management.
Focus on: high-level status, bottlenecks, team workload.
Show aggregate stats and recent activity.
Highlight any blocked or at-risk items.""",
        stage_transitions={}
    )
}


def get_role_config(role: str) -> RoleConfig:
    """Get configuration for a role, defaulting to CS if unknown"""
    return ROLE_COMPONENT_SETS.get(role, ROLE_COMPONENT_SETS["cs"])


def get_role_system_prompt(role: str, base_prompt: str) -> str:
    """
    Augment the base system prompt with role-specific instructions.
    """
    config = get_role_config(role)

    role_prompt = f"""
{base_prompt}

## Role-Specific Instructions ({role.upper()})

{config.ui_hints}

### Components Available for This Role:
{', '.join(config.components)}

### Context to Prioritize:
When retrieving context from Memex, focus on: {', '.join(config.context_focus)}

### View Type:
Generate a {config.primary_view} style interface.
"""
    return role_prompt


def get_handoff_targets(from_role: str) -> List[Dict[str, str]]:
    """
    Get valid handoff targets for a role.
    Returns list of {role, title, description} for users that can receive handoffs.
    """
    targets = []

    role_handoffs = {
        "sales": [
            {"role": "cs", "title": "Customer Success", "description": "For onboarding and customer relationship"},
        ],
        "cs": [
            {"role": "engineering", "title": "Engineering", "description": "For technical implementation"},
            {"role": "sales", "title": "Sales", "description": "For upsell or contract issues"},
        ],
        "engineering": [
            {"role": "cs", "title": "Customer Success", "description": "For customer communication"},
            {"role": "manager", "title": "Manager", "description": "For escalation"},
        ],
        "manager": [
            {"role": "cs", "title": "Customer Success", "description": "Assign to CS team"},
            {"role": "engineering", "title": "Engineering", "description": "Assign to Engineering team"},
            {"role": "sales", "title": "Sales", "description": "Assign to Sales team"},
        ]
    }

    return role_handoffs.get(from_role, [])


def get_stage_for_role(role: str) -> str:
    """Get the typical workflow stage for a role"""
    stage_map = {
        "sales": "closed",
        "cs": "onboarding",
        "engineering": "implementation",
        "manager": "review"
    }
    return stage_map.get(role, "pending")


def get_context_query_for_role(
    role: str,
    user_input: str,
    anchors: List[Dict[str, Any]]
) -> str:
    """
    Build a Memex query string optimized for the role's context needs.
    """
    config = get_role_config(role)

    # Extract key terms from anchors
    anchor_terms = []
    for anchor in anchors:
        if anchor.get("type") in config.context_focus:
            anchor_terms.append(anchor.get("text", ""))

    # Build query based on role focus
    query_parts = []

    # Add anchor terms
    if anchor_terms:
        query_parts.append(" ".join(anchor_terms))

    # Add role-specific search terms
    role_queries = {
        "sales": "deal closed won similar",
        "cs": "onboarding customer requirements timeline",
        "engineering": "implementation technical issues dependencies",
        "manager": "status pipeline blocked deadline"
    }
    query_parts.append(role_queries.get(role, ""))

    return " ".join(query_parts).strip()


# Component rendering hints for role-specific styling
ROLE_STYLING = {
    "sales": {
        "primary_color": "#10b981",  # Green - deals/money
        "accent": "emerald",
        "icon_theme": "deal"
    },
    "cs": {
        "primary_color": "#6366f1",  # Indigo - relationships
        "accent": "indigo",
        "icon_theme": "customer"
    },
    "engineering": {
        "primary_color": "#f59e0b",  # Amber - technical
        "accent": "amber",
        "icon_theme": "technical"
    },
    "manager": {
        "primary_color": "#8b5cf6",  # Purple - oversight
        "accent": "purple",
        "icon_theme": "dashboard"
    }
}


def get_role_styling(role: str) -> Dict[str, str]:
    """Get styling configuration for a role"""
    return ROLE_STYLING.get(role, ROLE_STYLING["cs"])
