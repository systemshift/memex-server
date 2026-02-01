"""Memex TUI Application."""

from textual.app import App, ComposeResult
from textual.widgets import Input, Footer, Header
from textual.containers import Container
from textual.binding import Binding

from .chat import ChatPanel, ChatEngine


class MemexApp(App):
    """Interactive TUI for memex and dagit."""

    CSS = """
    Screen {
        layout: grid;
        grid-size: 1;
        grid-rows: 1fr auto;
    }

    #chat-container {
        height: 100%;
        border: solid $primary;
        padding: 1;
    }

    #chat-log {
        height: 100%;
        scrollbar-gutter: stable;
    }

    #input-container {
        height: auto;
        padding: 1;
    }

    #input {
        dock: bottom;
    }

    Footer {
        height: auto;
    }
    """

    BINDINGS = [
        Binding("ctrl+c", "quit", "Quit"),
        Binding("ctrl+l", "clear", "Clear"),
        Binding("escape", "focus_input", "Focus Input", show=False),
    ]

    TITLE = "Memex"

    def __init__(self):
        super().__init__()
        self.chat_engine = ChatEngine()
        self._current_response = ""

    def compose(self) -> ComposeResult:
        """Create the UI layout."""
        yield Header()
        with Container(id="chat-container"):
            yield ChatPanel(id="chat-log")
        with Container(id="input-container"):
            yield Input(placeholder="Ask anything... (Ctrl+C to quit)", id="input")
        yield Footer()

    def on_mount(self) -> None:
        """Focus input on start."""
        self.query_one("#input", Input).focus()
        chat = self.query_one("#chat-log", ChatPanel)
        chat.add_system_message(
            "Welcome to Memex TUI. Ask questions about your knowledge graph or dagit network."
        )
        chat.add_system_message('Type "help" for commands, Ctrl+C to quit.')

    async def on_input_submitted(self, event: Input.Submitted) -> None:
        """Handle user input."""
        user_input = event.value.strip()
        if not user_input:
            return

        # Clear input
        input_widget = self.query_one("#input", Input)
        input_widget.value = ""

        chat = self.query_one("#chat-log", ChatPanel)

        # Handle special commands
        if user_input.lower() in ("exit", "quit"):
            self.exit()
            return

        if user_input.lower() == "clear":
            self.action_clear()
            return

        if user_input.lower() == "help":
            self._show_help(chat)
            return

        # Show user message
        chat.add_user_message(user_input)

        # Start assistant response
        chat.start_assistant_response()
        self._current_response = ""

        # Stream response
        async def on_text(text: str) -> None:
            self._current_response += text
            chat.add_assistant_text(text)

        async def on_tool(tool_name: str) -> None:
            chat.add_tool_indicator(tool_name)

        try:
            await self.chat_engine.send(user_input, on_text, on_tool)
        except Exception as e:
            chat.add_error(str(e))

        # Ensure we end on a new line
        if self._current_response and not self._current_response.endswith("\n"):
            chat.write("")

    def _show_help(self, chat: ChatPanel) -> None:
        """Show help message."""
        chat.add_system_message("Commands:")
        chat.add_system_message("  help  - Show this help")
        chat.add_system_message("  clear - Clear chat history")
        chat.add_system_message("  exit  - Quit the application")
        chat.add_system_message("")
        chat.add_system_message("Examples:")
        chat.add_system_message('  "search for notes about topic"')
        chat.add_system_message('  "what\'s my dagit identity"')
        chat.add_system_message('  "save this as a note: <your content>"')
        chat.add_system_message('  "post to dagit: <your message>"')

    def action_clear(self) -> None:
        """Clear chat history."""
        self.chat_engine.clear()
        chat = self.query_one("#chat-log", ChatPanel)
        chat.clear()
        chat.add_system_message("Chat cleared.")

    def action_focus_input(self) -> None:
        """Focus the input field."""
        self.query_one("#input", Input).focus()

    def action_quit(self) -> None:
        """Quit the application."""
        self.exit()
