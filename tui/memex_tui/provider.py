"""Chat provider for TUI."""

import os
import json
from typing import Generator
from dataclasses import dataclass

from dotenv import load_dotenv

load_dotenv()


@dataclass
class ToolCall:
    """A tool call request from the model."""

    id: str
    name: str
    arguments: dict


@dataclass
class Chunk:
    """Stream chunk from model."""

    type: str  # "text" | "tool_call" | "done" | "error"
    text: str = ""
    tool_call: ToolCall | None = None
    error: str = ""


class ChatProvider:
    """OpenAI-compatible chat provider with streaming and tool support."""

    def __init__(self, model: str | None = None):
        self._client = None
        self.model = model or os.getenv("OPENAI_MODEL", "gpt-4o")

    @property
    def client(self):
        """Lazy-initialize OpenAI client."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI()
        return self._client

    def stream(
        self,
        system: str,
        messages: list[dict],
        tools: list[dict] | None = None,
    ) -> Generator[Chunk, None, None]:
        """Stream response with tool support.

        Args:
            system: System prompt
            messages: Conversation messages
            tools: OpenAI-format tool definitions

        Yields:
            Chunk objects with text, tool_calls, or completion status
        """
        api_messages = []
        if system:
            api_messages.append({"role": "system", "content": system})
        api_messages.extend(messages)

        try:
            kwargs = {
                "model": self.model,
                "messages": api_messages,
                "stream": True,
            }
            if tools:
                kwargs["tools"] = tools

            stream = self.client.chat.completions.create(**kwargs)

            # Accumulate tool calls across chunks
            tool_calls: dict[int, dict] = {}

            for chunk in stream:
                if not chunk.choices:
                    continue

                delta = chunk.choices[0].delta
                finish_reason = chunk.choices[0].finish_reason

                # Text content
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

                # Finish with tool calls
                if finish_reason == "tool_calls":
                    for idx in sorted(tool_calls.keys()):
                        tc = tool_calls[idx]
                        try:
                            args = json.loads(tc["args"]) if tc["args"] else {}
                            yield Chunk(
                                type="tool_call",
                                tool_call=ToolCall(tc["id"], tc["name"], args),
                            )
                        except json.JSONDecodeError:
                            yield Chunk(
                                type="error",
                                error=f"Failed to parse tool args: {tc['args']}",
                            )
                    tool_calls.clear()

                # Normal completion
                elif finish_reason == "stop":
                    yield Chunk(type="done")

        except Exception as e:
            yield Chunk(type="error", error=str(e))
