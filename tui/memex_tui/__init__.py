"""Memex TUI - Interactive control plane for memex and dagit."""

from .app import MemexApp


def main():
    """Entry point for memex-tui command."""
    app = MemexApp()
    app.run()


__all__ = ["main", "MemexApp"]
