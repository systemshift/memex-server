#!/usr/bin/env python3
"""
Create the email auto-extraction subscription in Memex.

This subscription triggers a webhook when Email nodes are created,
allowing automatic anchor extraction.
"""

import sys
import os
import requests

sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'workspace'))

from config.email import email_config

MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")

SUBSCRIPTION = {
    "id": "sub:email-auto-extract",
    "name": "Email Auto-Extraction",
    "description": "Automatically extract anchors when emails are ingested",
    "pattern": {
        "event_types": ["node.created"],
        "node_types": ["Email"]
    },
    "webhook": {
        "url": email_config.extraction_webhook,
        "method": "POST",
        "headers": {
            "Content-Type": "application/json"
        },
        "retry": {
            "max_attempts": 3,
            "backoff_ms": 1000
        }
    },
    "enabled": True
}


def create_subscription():
    """Create the email extraction subscription"""
    print("Creating email auto-extraction subscription...")
    print(f"  Memex URL: {MEMEX_URL}")
    print(f"  Webhook URL: {SUBSCRIPTION['webhook']['url']}")

    try:
        # Check if subscription exists
        resp = requests.get(f"{MEMEX_URL}/api/subscriptions/{SUBSCRIPTION['id']}")
        if resp.status_code == 200:
            print(f"  Subscription {SUBSCRIPTION['id']} already exists, updating...")
            resp = requests.put(
                f"{MEMEX_URL}/api/subscriptions/{SUBSCRIPTION['id']}",
                json=SUBSCRIPTION
            )
        else:
            print(f"  Creating new subscription...")
            resp = requests.post(
                f"{MEMEX_URL}/api/subscriptions",
                json=SUBSCRIPTION
            )

        if resp.status_code in [200, 201]:
            print(f"  Successfully created/updated subscription: {SUBSCRIPTION['id']}")
            print("\nSubscription details:")
            print(f"  - Triggers on: {SUBSCRIPTION['pattern']['event_types']}")
            print(f"  - For node types: {SUBSCRIPTION['pattern']['node_types']}")
            print(f"  - Webhook: {SUBSCRIPTION['webhook']['url']}")
            return True
        else:
            print(f"  Failed to create subscription: {resp.status_code}")
            print(f"  Response: {resp.text}")
            return False

    except requests.exceptions.ConnectionError:
        print(f"  Error: Could not connect to Memex at {MEMEX_URL}")
        print("  Is memex-server running?")
        return False
    except Exception as e:
        print(f"  Error: {e}")
        return False


def list_subscriptions():
    """List all subscriptions"""
    try:
        resp = requests.get(f"{MEMEX_URL}/api/subscriptions")
        if resp.status_code == 200:
            data = resp.json()
            subs = data.get("subscriptions", [])
            print(f"\nExisting subscriptions ({len(subs)}):")
            for sub in subs:
                status = "enabled" if sub.get("enabled") else "disabled"
                print(f"  - {sub.get('id')}: {sub.get('name')} [{status}]")
        else:
            print("Could not list subscriptions")
    except Exception as e:
        print(f"Error listing subscriptions: {e}")


def main():
    import argparse

    parser = argparse.ArgumentParser(description="Manage email extraction subscription")
    parser.add_argument("--list", action="store_true", help="List all subscriptions")
    parser.add_argument("--delete", action="store_true", help="Delete the subscription")

    args = parser.parse_args()

    if args.list:
        list_subscriptions()
    elif args.delete:
        try:
            resp = requests.delete(f"{MEMEX_URL}/api/subscriptions/{SUBSCRIPTION['id']}")
            if resp.status_code in [200, 204]:
                print(f"Deleted subscription: {SUBSCRIPTION['id']}")
            else:
                print(f"Failed to delete: {resp.status_code}")
        except Exception as e:
            print(f"Error: {e}")
    else:
        success = create_subscription()
        if success:
            list_subscriptions()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
