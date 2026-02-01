"""Chat panel widget for Memex TUI."""

import json
from typing import Callable, Awaitable

from textual.widgets import RichLog
from rich.markdown import Markdown
from rich.text import Text

from .provider import ChatProvider, Chunk, ToolCall
from .tools import get_all_tools, execute_tool


SYSTEM_PROMPT = """You are Memex, an intelligent assistant with access to a knowledge graph (memex) and a decentralized social network (dagit).

Available capabilities:
- Search and query the knowledge graph (memex_search, memex_get_node, memex_traverse, etc.)
- Create notes and save information (memex_create_node)
- Post to the dagit social network (dagit_post)
- Read posts from dagit (dagit_read)
- Check your identity (dagit_whoami)

When users ask questions:
1. Use the appropriate tools to find information
2. Synthesize the results into a helpful response
3. If saving information, confirm what was saved

Be concise but helpful. Use the tools proactively when they would help answer the user's question."""


class ChatEngine:
    """Manages chat state and model interactions."""

    def __init__(self):
        self.provider = ChatProvider()
        self.messages: list[dict] = []
        self.tools = get_all_tools()

    async def send(
        self,
        user_input: str,
        on_text: Callable[[str], Awaitable[None]],
        on_tool: Callable[[str], Awaitable[None]],
    ) -> None:
        """Send a message and stream the response.

        Args:
            user_input: User's message
            on_text: Callback for text chunks
            on_tool: Callback for tool call notifications
        """
        self.messages.append({"role": "user", "content": user_input})

        max_turns = 10
        for _ in range(max_turns):
            text_buffer = ""
            tool_calls: list[ToolCall] = []

            for chunk in self.provider.stream(
                SYSTEM_PROMPT, self.messages, self.tools
            ):
                if chunk.type == "text":
                    await on_text(chunk.text)
                    text_buffer += chunk.text

                elif chunk.type == "tool_call":
                    tool_calls.append(chunk.tool_call)
                    await on_tool(f"[{chunk.tool_call.name}]")

                elif chunk.type == "error":
                    await on_text(f"\nError: {chunk.error}")
                    return

            # If there were tool calls, execute them and continue
            if tool_calls:
                # Add assistant message with tool calls
                self.messages.append(
                    {
                        "role": "assistant",
                        "content": text_buffer or None,
                        "tool_calls": [
                            {
                                "id": tc.id,
                                "type": "function",
                                "function": {
                                    "name": tc.name,
                                    "arguments": json.dumps(tc.arguments),
                                },
                            }
                            for tc in tool_calls
                        ],
                    }
                )

                # Execute tools and add results
                for tc in tool_calls:
                    result = execute_tool(tc.name, tc.arguments)
                    self.messages.append(
                        {
                            "role": "tool",
                            "tool_call_id": tc.id,
                            "content": result,
                        }
                    )
                continue

            # No tool calls - conversation turn complete
            if text_buffer:
                self.messages.append({"role": "assistant", "content": text_buffer})
            return

    def clear(self) -> None:
        """Clear conversation history."""
        self.messages.clear()


class ChatPanel(RichLog):
    """Chat display panel with rich formatting."""

    def __init__(self, **kwargs):
        super().__init__(markup=True, wrap=True, **kwargs)

    def add_user_message(self, text: str) -> None:
        """Add a user message to the display."""
        self.write(Text.from_markup(f"[bold cyan]You:[/bold cyan] {text}"))

    def add_assistant_text(self, text: str) -> None:
        """Add assistant text (streaming)."""
        # For streaming, we append to the current line
        self.write(text, scroll_end=True)

    def start_assistant_response(self) -> None:
        """Start a new assistant response line."""
        self.write(Text.from_markup("[bold green]Memex:[/bold green] "), scroll_end=True)

    def add_tool_indicator(self, tool_name: str) -> None:
        """Show a tool being called."""
        self.write(Text.from_markup(f"[dim]{tool_name}[/dim]"))

    def add_error(self, error: str) -> None:
        """Display an error message."""
        self.write(Text.from_markup(f"[bold red]Error:[/bold red] {error}"))

    def add_system_message(self, text: str) -> None:
        """Display a system message."""
        self.write(Text.from_markup(f"[dim italic]{text}[/dim italic]"))
