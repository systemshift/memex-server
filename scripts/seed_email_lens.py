#!/usr/bin/env python3
"""
Seed the email communication lens for extracting anchors from emails.

Run this script to create the lens:email in Memex before processing emails.
"""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'workspace'))

from services.memex import memex

EMAIL_LENS = {
    "id": "lens:email",
    "name": "Email Communication",
    "description": "Extract meaningful anchors from email conversations",
    "primitives": {
        "person": "A person mentioned by name or email address",
        "organization": "A company, team, or organization",
        "action_item": "A task, commitment, or to-do item",
        "decision": "A decision, agreement, or conclusion reached",
        "date": "A deadline, date reference, or time-sensitive mention",
        "topic": "A subject, project, or theme being discussed"
    },
    "patterns": {
        "commitment": {
            "description": "Someone committed to doing something",
            "required": ["person", "action_item"],
            "optional": ["date"]
        },
        "request": {
            "description": "A request or ask for someone to do something",
            "required": ["action_item"],
            "optional": ["person", "date"]
        },
        "meeting": {
            "description": "A scheduled meeting or call",
            "required": ["date"],
            "optional": ["person", "topic"]
        },
        "decision": {
            "description": "A decision or agreement made",
            "required": ["decision"],
            "optional": ["person", "topic"]
        },
        "introduction": {
            "description": "People or organizations being introduced",
            "required": ["person"],
            "optional": ["organization", "topic"]
        }
    },
    "extraction_hints": """
When extracting from emails:
- Look for action verbs: "will", "need to", "should", "must", "please"
- Identify commitments: "I'll", "we'll", "I will send", "I'll follow up"
- Find deadlines: "by Friday", "EOD", "next week", "ASAP"
- Recognize decisions: "we decided", "agreed to", "going with", "confirmed"
- Extract names from greetings, signatures, and CC lists
- Note topics from subject lines and key phrases
- Capture meeting references: "let's sync", "schedule a call", "meet"
""",
    "scope": "email",
    "version": "1.0"
}


def main():
    print("Seeding Email Communication lens...")

    # Check if lens already exists
    existing = memex.get_lens("lens:email")
    if existing:
        print("Lens lens:email already exists. Updating...")

    # Create the lens
    result = memex.create_lens(EMAIL_LENS)

    if result:
        print(f"Successfully created/updated lens: {result}")
        print("\nLens primitives:")
        for name, desc in EMAIL_LENS["primitives"].items():
            print(f"  - {name}: {desc}")
        print("\nLens patterns:")
        for name, pattern in EMAIL_LENS["patterns"].items():
            print(f"  - {name}: {pattern['description']}")
    else:
        print("Failed to create lens. Is memex-server running?")
        sys.exit(1)


if __name__ == "__main__":
    main()
