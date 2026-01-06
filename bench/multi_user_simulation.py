#!/usr/bin/env python3
"""
Multi-User Workflow Simulation

Demonstrates how multiple users in an organization benefit from shared context:
- Each user's workflow becomes context for future users
- Patterns emerge from collective usage
- Context "spills over" between related workflows

Run this after starting workflow_demo_progressive.py
"""

import json
import time
import requests
from datetime import datetime

API_URL = "http://localhost:5003"

# Simulated users with different workflows
SIMULATION_SCENARIOS = [
    # === SCENARIO 1: Expense workflows - context builds over time ===
    {
        "name": "Expense Reimbursements",
        "description": "Watch how context accumulates as multiple users submit expenses",
        "users": [
            {
                "user": "Alice (Engineering)",
                "messages": [
                    "I need reimbursement for a team lunch at Chipotle, $89",
                    "It was yesterday with the backend team"
                ],
                "note": "First expense - system has no prior context"
            },
            {
                "user": "Bob (Engineering)",
                "messages": [
                    "Expense for team dinner at the same Chipotle, $156",
                    "Last Friday with the frontend team, celebrating the launch"
                ],
                "note": "Should see Alice's recent expense as context"
            },
            {
                "user": "Carol (Sales)",
                "messages": [
                    "Client dinner expense - took Acme Corp to Nobu, $847",
                    "Tuesday night, discussing Q2 partnership"
                ],
                "note": "Different department, but should see expense policy patterns"
            },
            {
                "user": "Dave (Engineering)",
                "messages": [
                    "Chipotle lunch expense $67",
                ],
                "note": "By now, system knows Chipotle pattern from Alice & Bob"
            }
        ]
    },

    # === SCENARIO 2: Hiring workflows - cross-team learning ===
    {
        "name": "Hiring Requests",
        "description": "Teams learn from each other's hiring processes",
        "users": [
            {
                "user": "Sarah (Engineering Manager)",
                "messages": [
                    "Need to hire a senior backend engineer for the payments team",
                    "Budget is 180-220k, need someone with Go experience",
                    "Urgent - we're blocked on the checkout rewrite"
                ],
                "note": "First hiring request"
            },
            {
                "user": "Mike (Product Manager)",
                "messages": [
                    "Looking to hire a product designer for the mobile team",
                    "Mid-level, 120-150k range"
                ],
                "note": "Different role, but should see hiring process patterns"
            },
            {
                "user": "Lisa (Engineering Manager)",
                "messages": [
                    "Need another backend engineer, this time for the API team",
                    "Similar to what Sarah hired for - senior level, Go preferred"
                ],
                "note": "Should see Sarah's hiring request as relevant context"
            }
        ]
    },

    # === SCENARIO 3: Contract workflows - legal patterns emerge ===
    {
        "name": "Contract Requests",
        "description": "Legal patterns emerge from multiple contract requests",
        "users": [
            {
                "user": "Tom (BD)",
                "messages": [
                    "Need an NDA with TechStart before our demo",
                    "Standard mutual NDA, 2 year term",
                    "Contact is jamie@techstart.io"
                ],
                "note": "First NDA request"
            },
            {
                "user": "Jenny (BD)",
                "messages": [
                    "NDA needed with CloudCo for partnership discussion",
                    "Similar to the TechStart one Tom just did"
                ],
                "note": "Should see Tom's NDA as template/reference"
            },
            {
                "user": "Tom (BD)",
                "messages": [
                    "TechStart wants to move forward - need an MSA now",
                    "Following up from the NDA we signed last week"
                ],
                "note": "Same user, related workflow - should see full history"
            }
        ]
    }
]


def create_session(user_name: str) -> str:
    """Create a new session for a user"""
    resp = requests.post(f"{API_URL}/api/session", json={})
    return resp.json()["session_id"]


def send_message(session_id: str, message: str) -> dict:
    """Send a message and get updated state"""
    resp = requests.post(
        f"{API_URL}/api/message",
        json={"session_id": session_id, "message": message}
    )
    return resp.json()


def submit_workflow(session_id: str) -> dict:
    """Submit completed workflow to memex"""
    resp = requests.post(
        f"{API_URL}/api/submit",
        json={"session_id": session_id}
    )
    return resp.json()


def print_state(state: dict, indent: str = "    "):
    """Pretty print the workflow state"""
    print(f"{indent}Title: {state.get('title', 'Unknown')}")
    print(f"{indent}Complete: {state.get('complete', False)}")

    fields = state.get("fields", {})
    if fields:
        print(f"{indent}Fields:")
        for name, field in fields.items():
            status = "✓" if field.get("done") else "?"
            value = field.get("value", field.get("hint", ""))
            print(f"{indent}  {status} {field.get('label', name)}: {value}")

    context = state.get("context", [])
    if context:
        print(f"{indent}Context from memory:")
        for card in context:
            print(f"{indent}  📋 {card['title']}: {card['content'][:60]}...")

    pending = state.get("pending", [])
    if pending:
        print(f"{indent}Questions: {pending[0]}")


def run_user_workflow(user_info: dict, scenario_num: int, user_num: int):
    """Run a single user's workflow"""
    user = user_info["user"]
    messages = user_info["messages"]
    note = user_info.get("note", "")

    print(f"\n{'='*60}")
    print(f"USER {user_num}: {user}")
    print(f"{'='*60}")
    if note:
        print(f"📝 {note}")

    # Create session
    session_id = create_session(user)
    print(f"\nSession: {session_id}")

    # Send each message
    for i, message in enumerate(messages):
        print(f"\n💬 Message {i+1}: \"{message}\"")

        result = send_message(session_id, message)

        if "error" in result:
            print(f"   Error: {result['error']}")
            continue

        state = result.get("state", {})
        context_added = result.get("memex_context_added", 0)

        print(f"\n   State after message:")
        print_state(state, "      ")

        if context_added > 0:
            print(f"\n   🔗 {context_added} context items found in memex!")

        time.sleep(0.5)  # Small delay between messages

    # Submit if complete
    state = result.get("state", {})
    if state.get("complete"):
        print(f"\n✅ Workflow complete - submitting to memex...")
        submit_result = submit_workflow(session_id)
        print(f"   Stored as: {submit_result.get('workflow_id', 'local only')}")
    else:
        print(f"\n⏳ Workflow incomplete - saving draft...")
        submit_result = submit_workflow(session_id)
        print(f"   Stored as: {submit_result.get('workflow_id', 'local only')}")

    return session_id


def run_scenario(scenario: dict, scenario_num: int):
    """Run a full scenario with multiple users"""
    print(f"\n{'#'*60}")
    print(f"# SCENARIO {scenario_num}: {scenario['name']}")
    print(f"# {scenario['description']}")
    print(f"{'#'*60}")

    workflow_ids = []

    for i, user_info in enumerate(scenario["users"]):
        session_id = run_user_workflow(user_info, scenario_num, i + 1)
        workflow_ids.append(session_id)

        # Pause between users to simulate real usage
        if i < len(scenario["users"]) - 1:
            print(f"\n{'─'*40}")
            print("⏰ [Time passes... next user starts their workflow]")
            print(f"{'─'*40}")
            time.sleep(1)

    return workflow_ids


def check_memex_context():
    """Check what's accumulated in memex"""
    print(f"\n{'='*60}")
    print("MEMEX STATE: What's accumulated?")
    print(f"{'='*60}")

    try:
        # Search for workflows
        resp = requests.get(f"http://localhost:8080/api/query/search?q=Workflow&limit=10")
        data = resp.json()

        workflows = [n for n in data.get("nodes", []) if n.get("Type") == "Workflow"]
        print(f"\nWorkflows stored: {len(workflows)}")

        for w in workflows[:5]:
            meta = w.get("Meta", {})
            print(f"  - {w['ID']}: {meta.get('title', 'Unknown')} ({meta.get('status', '?')})")

        # Search for turns
        resp = requests.get(f"http://localhost:8080/api/query/search?q=WorkflowTurn&limit=20")
        data = resp.json()
        turns = [n for n in data.get("nodes", []) if n.get("Type") == "WorkflowTurn"]
        print(f"\nTotal conversation turns stored: {len(turns)}")

    except Exception as e:
        print(f"Could not query memex: {e}")


def main():
    print("""
╔══════════════════════════════════════════════════════════════╗
║         MULTI-USER WORKFLOW SIMULATION                       ║
║                                                              ║
║  Demonstrating context spillover between users:              ║
║  - Each workflow becomes context for future workflows        ║
║  - Patterns emerge from collective usage                     ║
║  - Users benefit from organizational knowledge               ║
╚══════════════════════════════════════════════════════════════╝
    """)

    # Check API is running
    try:
        resp = requests.get(f"{API_URL}/")
        if resp.status_code != 200:
            raise Exception("API not responding")
    except:
        print("ERROR: Progressive workflow demo not running!")
        print("Start it with: python bench/workflow_demo_progressive.py")
        return

    print("API is running. Starting simulation...\n")
    print("Watch how context accumulates as users complete workflows.\n")

    input("Press Enter to start Scenario 1 (Expenses)...")

    # Run just the first scenario for demo
    run_scenario(SIMULATION_SCENARIOS[0], 1)

    # Check memex state
    check_memex_context()

    print(f"\n{'='*60}")
    print("SIMULATION COMPLETE")
    print(f"{'='*60}")
    print("""
Key observations:
1. Later users saw context from earlier workflows
2. Similar workflows (Chipotle expenses) got linked
3. Policies surfaced once get reused
4. The system "learned" typical expense fields

Run again to see even more context accumulation!
    """)

    # Offer to run more scenarios
    more = input("\nRun Scenario 2 (Hiring)? [y/N]: ")
    if more.lower() == 'y':
        run_scenario(SIMULATION_SCENARIOS[1], 2)
        check_memex_context()

    more = input("\nRun Scenario 3 (Contracts)? [y/N]: ")
    if more.lower() == 'y':
        run_scenario(SIMULATION_SCENARIOS[2], 3)
        check_memex_context()


if __name__ == "__main__":
    main()
