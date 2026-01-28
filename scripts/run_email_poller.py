#!/usr/bin/env python3
"""
Run the email IMAP poller.

This script starts the email polling service that:
1. Connects to the configured IMAP server
2. Polls for new emails at regular intervals
3. Ingests emails into Memex as Email nodes
4. Persists sync state for resumable operation

Configuration via environment variables:
- IMAP_HOST: IMAP server hostname (default: imap.gmail.com)
- IMAP_PORT: IMAP server port (default: 993)
- EMAIL_USERNAME: Email address/username
- EMAIL_PASSWORD: Email password (use app-specific password for Gmail)
- EMAIL_MAILBOX: Mailbox to poll (default: INBOX)
- EMAIL_POLL_INTERVAL: Seconds between polls (default: 60)
- EMAIL_BATCH_SIZE: Max emails per poll (default: 50)
"""

import sys
import os
import signal
import threading

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'workspace'))

from config.email import email_config
from services.email_ingestion import IMAPPoller
from services.email_sync import get_sync_state


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Run the email IMAP poller",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Run with environment variables
  EMAIL_USERNAME=user@gmail.com EMAIL_PASSWORD=xxxx python run_email_poller.py

  # Run once (no polling loop)
  python run_email_poller.py --once

  # Reset sync state and start fresh
  python run_email_poller.py --reset

  # Show current sync status
  python run_email_poller.py --status
"""
    )
    parser.add_argument("--once", action="store_true", help="Poll once and exit")
    parser.add_argument("--reset", action="store_true", help="Reset sync state")
    parser.add_argument("--status", action="store_true", help="Show sync status")
    parser.add_argument("--test", action="store_true", help="Test connection only")
    parser.add_argument("--verbose", "-v", action="store_true", help="Verbose output")

    args = parser.parse_args()

    # Validate configuration
    errors = email_config.validate()
    if errors and not args.status:
        print("Configuration errors:")
        for error in errors:
            print(f"  - {error}")
        print("\nSet environment variables or create a .env file")
        sys.exit(1)

    sync_state = get_sync_state()

    # Handle --status
    if args.status:
        stats = sync_state.get_stats()
        print("Email Sync Status")
        print("=" * 40)
        print(f"Account: {email_config.username or '(not configured)'}")
        print(f"IMAP Host: {email_config.imap_host}")
        print(f"Mailbox: {email_config.mailbox}")
        print(f"Last UID: {stats['last_uid']}")
        print(f"Last Sync: {stats['last_sync'] or 'never'}")
        print(f"Total Emails Ingested: {stats['total_emails_ingested']}")
        print(f"Total Errors: {stats['total_errors']}")
        return

    # Handle --reset
    if args.reset:
        print("Resetting sync state...")
        sync_state.reset()
        print("Sync state reset. Next poll will start fresh.")
        return

    # Create poller
    poller = IMAPPoller(email_config)

    # Set callbacks
    def on_email_ingested(node_id, email_msg):
        if args.verbose:
            print(f"  Ingested: {email_msg.subject[:60]}")

    def on_error(error):
        print(f"  Error: {error}")
        sync_state.record_sync(0, errors=1)

    poller.on_email_ingested = on_email_ingested
    poller.on_error = on_error

    # Restore last UID from sync state
    poller.last_uid = sync_state.get_last_uid()

    print("=" * 50)
    print("MEMEX EMAIL POLLER")
    print("=" * 50)
    print(f"Host: {email_config.imap_host}")
    print(f"User: {email_config.username}")
    print(f"Mailbox: {email_config.mailbox}")
    print(f"Poll Interval: {email_config.poll_interval}s")
    print(f"Starting from UID: {poller.last_uid}")
    print("=" * 50)

    # Handle --test
    if args.test:
        print("\nTesting IMAP connection...")
        if poller.connect():
            print("Connection successful!")
            poller.disconnect()
        else:
            print("Connection failed!")
            sys.exit(1)
        return

    # Handle --once
    if args.once:
        print("\nPolling once...")
        count = poller.poll_once()
        print(f"Ingested {count} emails")

        # Save state
        sync_state.set_last_uid(poller.last_uid)
        sync_state.record_sync(count)

        poller.disconnect()
        return

    # Setup signal handlers for graceful shutdown
    stop_event = threading.Event()

    def signal_handler(signum, frame):
        print("\nShutting down...")
        stop_event.set()
        poller.stop()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Run polling loop
    print("\nStarting poll loop (Ctrl+C to stop)...")
    print()

    try:
        while not stop_event.is_set():
            count = poller.poll_once()

            # Save state after each poll
            if count > 0 or poller.last_uid > sync_state.get_last_uid():
                sync_state.set_last_uid(poller.last_uid)
                sync_state.record_sync(count)

            # Wait for next poll
            stop_event.wait(timeout=email_config.poll_interval)

    except KeyboardInterrupt:
        pass
    finally:
        poller.disconnect()
        print("Poller stopped.")
        stats = sync_state.get_stats()
        print(f"Final UID: {stats['last_uid']}")
        print(f"Total emails ingested this session: {stats['total_emails_ingested']}")


if __name__ == "__main__":
    main()
