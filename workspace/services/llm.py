"""
LLM service for Memex Workspace.

Handles all interactions with OpenAI API, including:
- Intent classification
- Component generation via tool calls
- Streaming responses
"""

import os
import json
from typing import Dict, Any, List, Optional, Generator, AsyncGenerator
from dataclasses import dataclass
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

from openai import OpenAI

MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")


@dataclass
class ToolCall:
    """A tool call from the LLM"""
    name: str
    arguments: Dict[str, Any]
    id: str


class LLMClient:
    """Client for LLM interactions"""

    def __init__(self, model: str = MODEL):
        self.client = OpenAI()
        self.model = model

    def classify_intent(self, user_input: str, context: List[Dict] = None) -> Dict[str, Any]:
        """
        Classify user intent and extract entities.
        Returns intent type, confidence, entities, and suggested view.
        """
        context_str = ""
        if context:
            context_str = "\nRecent context:\n" + "\n".join([
                f"- {c.get('title', '')}: {c.get('content', '')[:100]}"
                for c in context[:5]
            ])

        prompt = f"""Classify this user input into one of these intent types:

1. WORKFLOW - Multi-step process like expense reports, hiring requests, approvals, onboarding
2. DOCUMENT - Creating or editing text content, drafting messages, writing proposals
3. QUERY - Asking questions, searching for information, "what is", "who is", "show me"
4. TABLE - Viewing structured data, lists, comparisons, "list all", "show table"
5. MESSAGE - Direct communication to a person or team, notifications

User input: "{user_input}"
{context_str}

Extract any entities (amounts, names, dates, companies, etc.) from the input.

Return JSON:
{{
    "intent": "workflow|document|query|table|message",
    "confidence": 0.0-1.0,
    "title": "short title for this request",
    "summary": "what the user wants to do",
    "entities": [
        {{"type": "amount|person|company|date|vendor|etc", "value": "extracted value", "raw": "original text"}}
    ],
    "suggested_view": "form|table|kanban|editor|chat|timeline",
    "data_requirements": ["fields that will be needed"]
}}"""

        try:
            response = self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                response_format={"type": "json_object"}
            )
            return json.loads(response.choices[0].message.content)
        except Exception as e:
            print(f"Intent classification error: {e}")
            return {
                "intent": "unknown",
                "confidence": 0,
                "title": "Unknown",
                "summary": user_input,
                "entities": [],
                "suggested_view": "form"
            }

    def generate_components(
        self,
        user_input: str,
        intent: Dict[str, Any],
        context: List[Dict],
        tools: List[Dict[str, Any]]
    ) -> Generator[ToolCall, None, None]:
        """
        Generate UI components using tool calls.
        Yields ToolCall objects as they're generated.
        """
        # Build system prompt
        system_prompt = """You are a UI generator. Generate COMPLETE UI forms by calling the provided tools.

IMPORTANT: You MUST call multiple tools to create a complete form. Generate ALL of these components:

1. form_header - ALWAYS start with this for the title
2. Field tools - Call one tool for EACH piece of data needed:
   - currency_field for money amounts
   - text_field for text like vendor names, descriptions
   - date_field for dates
   - select_field for categories with options
   - file_field for receipts/attachments
3. action_bar - ALWAYS end with this for submit/save buttons

RULES:
- Pre-fill values when you can extract them from the user input
- Set done=true for fields with values, done=false for empty fields
- Set required=true for mandatory fields
- Generate AT LEAST 4-6 components for a complete form
- ALWAYS include an action_bar at the end"""

        # Build user message with context
        context_str = ""
        if context:
            context_str = "\n\nOrganizational context:\n" + "\n".join([
                f"- {c.get('title', '')}: {c.get('content', '')}"
                for c in context[:5]
            ])

        entities_str = ""
        if intent.get("entities"):
            entities_str = "\n\nExtracted entities:\n" + "\n".join([
                f"- {e.get('type', 'unknown')}: {e.get('value', '')}"
                for e in intent["entities"]
            ])

        user_message = f"""Generate UI for this request:

User input: "{user_input}"

Intent: {intent.get('intent', 'unknown')}
Title: {intent.get('title', 'Request')}
{entities_str}
{context_str}

Generate the appropriate UI components by calling the tools."""

        try:
            response = self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_message}
                ],
                tools=tools,
                tool_choice="auto"
            )

            # Process tool calls
            message = response.choices[0].message
            if message.tool_calls:
                for tool_call in message.tool_calls:
                    yield ToolCall(
                        name=tool_call.function.name,
                        arguments=json.loads(tool_call.function.arguments),
                        id=tool_call.id
                    )

        except Exception as e:
            print(f"Component generation error: {e}")
            # Yield a basic error component
            yield ToolCall(
                name="text_display",
                arguments={"content": f"Error generating UI: {str(e)}"},
                id="error"
            )

    def stream_components(
        self,
        user_input: str,
        intent: Dict[str, Any],
        context: List[Dict],
        tools: List[Dict[str, Any]]
    ) -> Generator[ToolCall, None, None]:
        """
        Stream UI components using streaming tool calls.
        Yields ToolCall objects as they complete.
        """
        system_prompt = """You are a UI generator. Generate COMPLETE UI forms by calling the provided tools.

IMPORTANT: You MUST call multiple tools to create a complete form. Generate ALL of these components:

1. form_header - ALWAYS start with this for the title
2. Field tools - Call one tool for EACH piece of data needed:
   - currency_field for money amounts
   - text_field for text like vendor names, descriptions
   - date_field for dates
   - select_field for categories with options
   - file_field for receipts/attachments
3. action_bar - ALWAYS end with this for submit/save buttons

RULES:
- Pre-fill values when you can extract them from the user input
- Set done=true for fields with values, done=false for empty fields
- Set required=true for mandatory fields
- Generate AT LEAST 4-6 components for a complete form
- ALWAYS include an action_bar at the end"""

        context_str = ""
        if context:
            context_str = "\n\nContext from organizational memory:\n" + "\n".join([
                f"- {c.get('title', '')}: {c.get('content', '')}"
                for c in context[:5]
            ])

        entities_str = ""
        if intent.get("entities"):
            entities_str = "\n\nExtracted from input:\n" + "\n".join([
                f"- {e.get('type', 'unknown')}: {e.get('value', '')}"
                for e in intent["entities"]
            ])

        user_message = f"""Build UI for:

"{user_input}"

Intent: {intent.get('intent')} | Title: {intent.get('title')}
{entities_str}
{context_str}

Call tools to generate the UI components."""

        try:
            # Use streaming for real-time component generation
            stream = self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_message}
                ],
                tools=tools,
                tool_choice="auto",
                stream=True
            )

            # Accumulate tool calls from stream
            current_tool_calls = {}

            for chunk in stream:
                delta = chunk.choices[0].delta

                if delta.tool_calls:
                    for tool_call_delta in delta.tool_calls:
                        idx = tool_call_delta.index

                        if idx not in current_tool_calls:
                            current_tool_calls[idx] = {
                                "id": "",
                                "name": "",
                                "arguments": ""
                            }

                        if tool_call_delta.id:
                            current_tool_calls[idx]["id"] = tool_call_delta.id
                        if tool_call_delta.function:
                            if tool_call_delta.function.name:
                                current_tool_calls[idx]["name"] = tool_call_delta.function.name
                            if tool_call_delta.function.arguments:
                                current_tool_calls[idx]["arguments"] += tool_call_delta.function.arguments

            # Yield completed tool calls
            for idx in sorted(current_tool_calls.keys()):
                tc = current_tool_calls[idx]
                if tc["name"] and tc["arguments"]:
                    try:
                        args = json.loads(tc["arguments"])
                        yield ToolCall(
                            name=tc["name"],
                            arguments=args,
                            id=tc["id"]
                        )
                    except json.JSONDecodeError:
                        continue

        except Exception as e:
            print(f"Stream error: {e}")
            yield ToolCall(
                name="text_display",
                arguments={"content": f"Error: {str(e)}"},
                id="error"
            )


# Global client instance
llm = LLMClient()
