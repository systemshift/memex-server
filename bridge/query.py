#!/usr/bin/env python3
"""
Bridge query processor.

Usage:
    python query.py "your question here"
    echo "your question" | python query.py

Streams response to stdout. Tool calls are shown as [tool_name: args].
"""

import sys
import os
import json

# Add bridge dir to path for imports
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from dotenv import load_dotenv
load_dotenv()

from providers import get_provider, Chunk, ToolCall
from tools import TOOLS, execute

SYSTEM_PROMPT = """You are a knowledge graph assistant for Memex.

Help users find information by querying the graph. You have tools to search, traverse, and retrieve nodes.

When answering:
1. Use search to find relevant nodes
2. Use traverse or get_links to explore relationships
3. Synthesize findings into a clear response
4. Cite node IDs when relevant

Node types: Person, Company, Document, Task, Project, Message, Event

Be concise. If you can't find something, say so."""


def run(query: str, history: list = None):
    """Run a query and stream output to stdout"""
    provider = get_provider()

    # Build messages
    messages = history or []
    messages.append({"role": "user", "content": query})

    # Agent loop
    max_turns = 10
    for _ in range(max_turns):
        text_buffer = ""
        tool_calls = []

        for chunk in provider.stream(SYSTEM_PROMPT, messages, TOOLS):
            if chunk.type == "text":
                print(chunk.text, end="", flush=True)
                text_buffer += chunk.text

            elif chunk.type == "tool_call":
                tool_calls.append(chunk.tool_call)
                # Show tool execution
                print(f"\n[{chunk.tool_call.name}]", flush=True)

            elif chunk.type == "error":
                print(f"\nError: {chunk.error}", file=sys.stderr)
                return

            elif chunk.type == "done":
                pass

        # If tool calls, execute and continue
        if tool_calls:
            # Add assistant message
            messages.append({
                "role": "assistant",
                "content": text_buffer or None,
                "tool_calls": [{
                    "id": tc.id,
                    "type": "function",
                    "function": {"name": tc.name, "arguments": json.dumps(tc.arguments)}
                } for tc in tool_calls]
            })

            # Execute tools
            for tc in tool_calls:
                result = execute(tc.name, tc.arguments)
                messages.append({
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": result
                })

            continue

        # No tool calls - done
        print()  # Final newline
        return

    print("\nMax turns reached", file=sys.stderr)


def main():
    # Get query from args or stdin
    if len(sys.argv) > 1:
        query = " ".join(sys.argv[1:])
    else:
        query = sys.stdin.read().strip()

    if not query:
        print("Usage: python query.py \"your question\"", file=sys.stderr)
        sys.exit(1)

    run(query)


if __name__ == "__main__":
    main()
