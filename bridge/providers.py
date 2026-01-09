"""
LLM Provider abstraction.

Swap between OpenAI, Anthropic, or others without changing the rest of the code.
"""

import os
import json
from abc import ABC, abstractmethod
from typing import Dict, Any, List, Generator, Callable
from dataclasses import dataclass


@dataclass
class Tool:
    """Tool definition for function calling"""
    name: str
    description: str
    parameters: Dict[str, Any]


@dataclass
class ToolCall:
    """A tool call request from the LLM"""
    id: str
    name: str
    arguments: Dict[str, Any]


@dataclass
class Chunk:
    """Stream chunk from LLM"""
    type: str  # "text" | "tool_call" | "done" | "error"
    text: str = ""
    tool_call: ToolCall = None
    error: str = ""


class Provider(ABC):
    """Abstract LLM provider"""

    @abstractmethod
    def stream(
        self,
        system: str,
        messages: List[Dict],
        tools: List[Tool]
    ) -> Generator[Chunk, None, None]:
        """Stream response with tool support"""
        pass


class OpenAIProvider(Provider):
    """OpenAI provider (GPT-4, etc.)"""

    def __init__(self, model: str = None):
        from openai import OpenAI
        self.client = OpenAI()
        self.model = model or os.getenv("OPENAI_MODEL", "gpt-4o")

    def stream(
        self,
        system: str,
        messages: List[Dict],
        tools: List[Tool]
    ) -> Generator[Chunk, None, None]:

        # Build messages
        api_messages = [{"role": "system", "content": system}] if system else []
        api_messages.extend(messages)

        # Build tools
        api_tools = [{
            "type": "function",
            "function": {
                "name": t.name,
                "description": t.description,
                "parameters": t.parameters
            }
        } for t in tools] if tools else None

        try:
            stream = self.client.chat.completions.create(
                model=self.model,
                messages=api_messages,
                tools=api_tools,
                stream=True
            )

            # Accumulate tool calls
            tool_calls: Dict[int, Dict] = {}

            for chunk in stream:
                delta = chunk.choices[0].delta

                # Text
                if delta.content:
                    yield Chunk(type="text", text=delta.content)

                # Tool calls (accumulate across chunks)
                if delta.tool_calls:
                    for tc in delta.tool_calls:
                        idx = tc.index
                        if idx not in tool_calls:
                            tool_calls[idx] = {"id": "", "name": "", "args": ""}
                        if tc.id:
                            tool_calls[idx]["id"] = tc.id
                        if tc.function:
                            if tc.function.name:
                                tool_calls[idx]["name"] = tc.function.name
                            if tc.function.arguments:
                                tool_calls[idx]["args"] += tc.function.arguments

                # Finish
                if chunk.choices[0].finish_reason == "tool_calls":
                    for idx in sorted(tool_calls.keys()):
                        tc = tool_calls[idx]
                        try:
                            args = json.loads(tc["args"]) if tc["args"] else {}
                            yield Chunk(
                                type="tool_call",
                                tool_call=ToolCall(tc["id"], tc["name"], args)
                            )
                        except json.JSONDecodeError:
                            pass
                    tool_calls.clear()

                elif chunk.choices[0].finish_reason == "stop":
                    yield Chunk(type="done")

        except Exception as e:
            yield Chunk(type="error", error=str(e))


def get_provider(name: str = None) -> Provider:
    """Get provider by name or auto-detect from env"""
    name = name or os.getenv("LLM_PROVIDER", "openai")

    if name == "openai":
        return OpenAIProvider()
    # Add more providers here:
    # elif name == "anthropic":
    #     return AnthropicProvider()
    else:
        raise ValueError(f"Unknown provider: {name}")
