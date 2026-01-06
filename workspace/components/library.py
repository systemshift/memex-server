"""
Component Library for Memex Workspace.

Defines UI components as OpenAI function tools.
The LLM calls these tools to generate the UI.
"""

from typing import List, Dict, Any


# Tool definitions for LLM to call
COMPONENT_TOOLS: List[Dict[str, Any]] = [
    {
        "type": "function",
        "function": {
            "name": "form_header",
            "description": "Create a form header with title and optional description",
            "parameters": {
                "type": "object",
                "properties": {
                    "title": {
                        "type": "string",
                        "description": "The form title"
                    },
                    "description": {
                        "type": "string",
                        "description": "Optional description or instructions"
                    }
                },
                "required": ["title"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "text_field",
            "description": "Create a single-line text input field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string", "description": "Field name/key"},
                    "label": {"type": "string", "description": "Display label"},
                    "value": {"type": "string", "description": "Pre-filled value if any"},
                    "hint": {"type": "string", "description": "Help text or placeholder"},
                    "required": {"type": "boolean", "description": "Is this field required?"},
                    "done": {"type": "boolean", "description": "Is this field complete?"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "textarea_field",
            "description": "Create a multi-line text area field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "value": {"type": "string"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"},
                    "rows": {"type": "integer", "description": "Number of rows"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "currency_field",
            "description": "Create a currency/money input field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "value": {"type": "number", "description": "Numeric value"},
                    "currency": {"type": "string", "description": "Currency symbol (default $)"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "date_field",
            "description": "Create a date picker field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "value": {"type": "string", "description": "Date value (ISO format or natural)"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "select_field",
            "description": "Create a dropdown select field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "options": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "List of options"
                    },
                    "value": {"type": "string", "description": "Selected value"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"}
                },
                "required": ["name", "label", "options"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "email_field",
            "description": "Create an email input field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "value": {"type": "string"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "file_field",
            "description": "Create a file upload field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "accept": {"type": "string", "description": "Accepted file types (e.g., image/*, .pdf)"},
                    "hint": {"type": "string"},
                    "required": {"type": "boolean"},
                    "done": {"type": "boolean"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "checkbox_field",
            "description": "Create a checkbox field",
            "parameters": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "label": {"type": "string"},
                    "checked": {"type": "boolean"},
                    "required": {"type": "boolean"}
                },
                "required": ["name", "label"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "context_card",
            "description": "Display relevant context from organizational memory",
            "parameters": {
                "type": "object",
                "properties": {
                    "title": {"type": "string", "description": "Card title"},
                    "content": {"type": "string", "description": "Context information"},
                    "type": {
                        "type": "string",
                        "enum": ["info", "policy", "related", "warning"],
                        "description": "Type of context"
                    }
                },
                "required": ["title", "content"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "action_bar",
            "description": "Create action buttons (submit, save, etc.)",
            "parameters": {
                "type": "object",
                "properties": {
                    "primary_action": {"type": "string", "description": "Primary button text"},
                    "secondary_actions": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Secondary button texts"
                    },
                    "primary_disabled": {"type": "boolean", "description": "Disable primary button"}
                },
                "required": ["primary_action"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "text_display",
            "description": "Display read-only text content",
            "parameters": {
                "type": "object",
                "properties": {
                    "content": {"type": "string"},
                    "style": {
                        "type": "string",
                        "enum": ["normal", "heading", "subheading", "muted"],
                        "description": "Text style"
                    }
                },
                "required": ["content"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "divider",
            "description": "Add a visual divider/separator",
            "parameters": {
                "type": "object",
                "properties": {
                    "label": {"type": "string", "description": "Optional label for divider"}
                }
            }
        }
    }
]


def get_tools() -> List[Dict[str, Any]]:
    """Get all component tools for LLM"""
    return COMPONENT_TOOLS


def get_tool_names() -> List[str]:
    """Get list of tool names"""
    return [tool["function"]["name"] for tool in COMPONENT_TOOLS]
