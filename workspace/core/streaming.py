"""
Streaming UI Generation for Memex Workspace.

Orchestrates the generative UI pipeline:
1. Classify intent
2. Fetch Memex context
3. Stream component generation
4. Render HTML fragments
"""

from typing import Generator, Dict, Any, List
from dataclasses import dataclass

from core.types import ViewSpec, ComponentSpec, IntentResult, ContextCard, ViewType
from services.llm import llm, ToolCall
from services.memex import memex
from components.library import get_tools
from components.renderer import render_component


@dataclass
class StreamedComponent:
    """A component streamed from the generation pipeline"""
    html: str
    component_name: str
    arguments: Dict[str, Any]
    is_complete: bool = False


class GenerativeUI:
    """Orchestrates generative UI pipeline"""

    def __init__(self):
        self.llm = llm
        self.memex = memex
        self.tools = get_tools()

    def generate(self, user_input: str) -> Generator[StreamedComponent, None, ViewSpec]:
        """
        Generate UI from user input.
        Yields HTML fragments as they're generated.
        Returns final ViewSpec.
        """
        # Step 1: Get Memex context
        context = self.memex.get_context_for_input(user_input)

        # Step 2: Classify intent
        intent = self.llm.classify_intent(user_input, context)

        # Step 3: Stream component generation
        components = []
        view_type = self._intent_to_view_type(intent.get("intent", "workflow"))

        for tool_call in self.llm.stream_components(user_input, intent, context, self.tools):
            # Enrich with Memex suggestions if it's a field
            if self._is_field_component(tool_call.name):
                tool_call = self._enrich_with_suggestions(tool_call, user_input)

            # Render to HTML
            html = render_component(tool_call.name, tool_call.arguments)

            # Track component
            components.append({
                "name": tool_call.name,
                "arguments": tool_call.arguments,
                "id": tool_call.id
            })

            # Yield streamed component
            yield StreamedComponent(
                html=html,
                component_name=tool_call.name,
                arguments=tool_call.arguments
            )

        # Build final ViewSpec
        view_spec = ViewSpec(
            view_type=view_type,
            title=intent.get("title"),
            source_input=user_input,
            components=[
                ComponentSpec(
                    component_type=c["name"],
                    props=c["arguments"],
                    name=c["arguments"].get("name"),
                    label=c["arguments"].get("label"),
                    value=c["arguments"].get("value"),
                    done=c["arguments"].get("done", False)
                )
                for c in components
            ],
            context={
                "intent": intent,
                "memex_context": context
            },
            complete=self._check_complete(components)
        )

        # Yield completion marker
        yield StreamedComponent(
            html="",
            component_name="_complete",
            arguments={"view_spec_id": view_spec.id},
            is_complete=True
        )

        return view_spec

    def generate_html(self, user_input: str) -> Generator[str, None, Dict[str, Any]]:
        """
        Generate HTML fragments from user input.
        Simpler interface - just yields HTML strings.
        """
        result = {
            "components": [],
            "view_spec": None
        }

        gen = self.generate(user_input)
        for component in gen:
            if component.is_complete:
                continue
            result["components"].append({
                "name": component.component_name,
                "args": component.arguments
            })
            yield component.html

        # Get final view spec from generator
        try:
            result["view_spec"] = gen.send(None)
        except StopIteration as e:
            result["view_spec"] = e.value

        return result

    def _intent_to_view_type(self, intent: str) -> ViewType:
        """Convert intent string to ViewType"""
        mapping = {
            "workflow": ViewType.FORM,
            "document": ViewType.EDITOR,
            "query": ViewType.QUERY,
            "table": ViewType.TABLE,
            "message": ViewType.CHAT,
        }
        return mapping.get(intent.lower(), ViewType.FORM)

    def _is_field_component(self, name: str) -> bool:
        """Check if component is a form field"""
        return name.endswith("_field")

    def _enrich_with_suggestions(self, tool_call: ToolCall, context: str) -> ToolCall:
        """Add Memex suggestions to a field component"""
        field_name = tool_call.arguments.get("name", "")
        field_value = tool_call.arguments.get("value")

        if field_name:
            suggestions = self.memex.get_suggestions_for_field(
                field_name,
                field_value,
                context
            )
            if suggestions:
                tool_call.arguments["suggestions"] = suggestions
                if not field_value and suggestions:
                    # Auto-fill with top suggestion
                    tool_call.arguments["value"] = suggestions[0]["value"]
                    tool_call.arguments["auto_filled"] = True
                    tool_call.arguments["done"] = True

        return tool_call

    def _check_complete(self, components: List[Dict]) -> bool:
        """Check if all required fields are complete"""
        for comp in components:
            args = comp.get("arguments", {})
            if args.get("required") and not args.get("done"):
                return False
        return True


# Global instance
generative_ui = GenerativeUI()


def stream_ui(user_input: str) -> Generator[str, None, Dict[str, Any]]:
    """Stream UI HTML fragments from user input"""
    return generative_ui.generate_html(user_input)
