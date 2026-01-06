"""
Core type definitions for Memex Workspace.

ViewSpec, ComponentSpec, and enums that define the UI generation contract.
"""

from dataclasses import dataclass, field
from enum import Enum
from typing import Dict, Any, List, Optional
from datetime import datetime
import uuid


class ViewType(str, Enum):
    """Types of views the system can generate"""
    FORM = "form"           # Structured data input (replaces Forms)
    TABLE = "table"         # Tabular data view/edit (replaces Sheets)
    KANBAN = "kanban"       # Status-based workflow (replaces Jira)
    EDITOR = "editor"       # Rich text editing (replaces Docs)
    CHAT = "chat"           # Messaging (replaces Slack)
    TIMELINE = "timeline"   # Time-based view (replaces Calendar)
    DASHBOARD = "dashboard" # Multi-widget view
    QUERY = "query"         # Search results / Q&A


class IntentType(str, Enum):
    """Classification of user intent"""
    WORKFLOW = "workflow"   # Multi-step process (expense, hiring, approval)
    DOCUMENT = "document"   # Creating/editing text content
    QUERY = "query"         # Asking questions, searching
    TABLE = "table"         # Structured data view
    MESSAGE = "message"     # Communication to person/team
    UNKNOWN = "unknown"


class FieldType(str, Enum):
    """Types of form fields"""
    TEXT = "text"
    TEXTAREA = "textarea"
    EMAIL = "email"
    CURRENCY = "currency"
    NUMBER = "number"
    DATE = "date"
    DATETIME = "datetime"
    SELECT = "select"
    MULTISELECT = "multiselect"
    CHECKBOX = "checkbox"
    RADIO = "radio"
    FILE = "file"
    HIDDEN = "hidden"


@dataclass
class FieldSuggestion:
    """A suggestion for auto-completing a field value"""
    value: Any
    source: str           # Where this suggestion came from (memex node ID)
    confidence: float     # 0.0 to 1.0
    label: Optional[str] = None  # Human-readable source description


@dataclass
class ComponentSpec:
    """Specification for a single UI component"""
    component_type: str                           # e.g., "text_input", "table", "context_card"
    props: Dict[str, Any] = field(default_factory=dict)  # Component-specific properties
    data_binding: Optional[str] = None            # Path to data source
    children: List['ComponentSpec'] = field(default_factory=list)
    id: str = field(default_factory=lambda: uuid.uuid4().hex[:8])

    # For form fields
    name: Optional[str] = None
    label: Optional[str] = None
    field_type: Optional[FieldType] = None
    value: Any = None
    done: bool = False
    required: bool = False
    hint: Optional[str] = None
    options: List[str] = field(default_factory=list)  # For select/radio
    suggestions: List[FieldSuggestion] = field(default_factory=list)
    auto_filled: bool = False

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization"""
        result = {
            "id": self.id,
            "component_type": self.component_type,
            "props": self.props,
        }
        if self.data_binding:
            result["data_binding"] = self.data_binding
        if self.children:
            result["children"] = [c.to_dict() for c in self.children]
        if self.name:
            result["name"] = self.name
        if self.label:
            result["label"] = self.label
        if self.field_type:
            result["field_type"] = self.field_type.value
        if self.value is not None:
            result["value"] = self.value
        result["done"] = self.done
        result["required"] = self.required
        if self.hint:
            result["hint"] = self.hint
        if self.options:
            result["options"] = self.options
        if self.suggestions:
            result["suggestions"] = [
                {"value": s.value, "source": s.source, "confidence": s.confidence, "label": s.label}
                for s in self.suggestions
            ]
        result["auto_filled"] = self.auto_filled
        return result


@dataclass
class LayoutSpec:
    """Specification for layout arrangement"""
    type: str = "single"  # single, split, tabs, grid
    props: Dict[str, Any] = field(default_factory=dict)


@dataclass
class ViewSpec:
    """Complete specification for a generated UI view"""
    id: str = field(default_factory=lambda: uuid.uuid4().hex[:12])
    view_type: ViewType = ViewType.FORM
    title: Optional[str] = None
    description: Optional[str] = None
    layout: LayoutSpec = field(default_factory=LayoutSpec)
    components: List[ComponentSpec] = field(default_factory=list)
    data_bindings: Dict[str, str] = field(default_factory=dict)
    interactions: List[str] = field(default_factory=list)
    context: Dict[str, Any] = field(default_factory=dict)
    complete: bool = False

    # Metadata
    created: datetime = field(default_factory=datetime.now)
    source_input: Optional[str] = None  # Original user input

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization"""
        return {
            "id": self.id,
            "view_type": self.view_type.value,
            "title": self.title,
            "description": self.description,
            "layout": {"type": self.layout.type, "props": self.layout.props},
            "components": [c.to_dict() for c in self.components],
            "data_bindings": self.data_bindings,
            "interactions": self.interactions,
            "context": self.context,
            "complete": self.complete,
            "created": self.created.isoformat(),
            "source_input": self.source_input,
        }


@dataclass
class IntentResult:
    """Result of intent classification"""
    intent_type: IntentType
    confidence: float
    title: Optional[str] = None
    summary: Optional[str] = None
    entities: List[Dict[str, Any]] = field(default_factory=list)
    suggested_view: Optional[ViewType] = None
    data_requirements: List[str] = field(default_factory=list)


@dataclass
class ContextCard:
    """A card of context information from Memex"""
    title: str
    content: str
    source_id: Optional[str] = None
    source_type: Optional[str] = None
    relevance: float = 1.0


@dataclass
class WorkspaceSession:
    """State for a workspace session"""
    id: str = field(default_factory=lambda: uuid.uuid4().hex[:12])
    created: datetime = field(default_factory=datetime.now)
    history: List[Dict[str, Any]] = field(default_factory=list)  # User inputs
    items: List[ViewSpec] = field(default_factory=list)          # Generated views
    active_item_id: Optional[str] = None
    user_id: Optional[str] = None
