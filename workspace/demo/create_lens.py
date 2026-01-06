"""
Create the deal-flow lens in Memex for the VC demo.

This lens defines the vocabulary for sales-to-implementation workflows:
- Primitives: company, amount, requirement, contact, owner, stage, deadline, blocker
- Patterns: deal-closed, handoff, blocked, at-risk

Run this script once to set up the lens before the demo.
"""

import requests
import json

MEMEX_URL = "http://localhost:8080"

# The Deal-Flow Lens - our domain vocabulary
DEAL_FLOW_LENS = {
    "id": "lens:deal-flow",
    "name": "Deal Flow",
    "description": "Extraction schema for sales-to-implementation workflow",
    "version": "1.0",
    "author": "memex-workspace",
    "primitives": {
        "company": "customer organization name",
        "amount": "deal value in currency (e.g., $50k ARR)",
        "requirement": "technical or business need (SSO, API integration, etc.)",
        "contact": "person at customer company",
        "owner": "internal person responsible for this stage",
        "stage": "current workflow stage: closed, onboarding, implementation, complete",
        "deadline": "target completion date",
        "blocker": "issue preventing progress"
    },
    "patterns": {
        "deal-closed": {
            "requires": ["company", "amount"],
            "description": "A completed sale ready for handoff to CS"
        },
        "handoff": {
            "requires": ["owner"],
            "description": "Work being transferred to another person"
        },
        "blocked": {
            "requires": ["blocker"],
            "description": "Work stopped by an issue"
        },
        "at-risk": {
            "requires": ["deadline", "blocker"],
            "description": "Deadline in danger due to blocker"
        },
        "implementation-ready": {
            "requires": ["company", "requirement"],
            "description": "Technical work ready to begin"
        }
    },
    "extraction_hints": """Look for:
- Company names (proper nouns, often followed by Corp, Inc, LLC)
- Dollar amounts with ARR/MRR indicators
- Technical requirements: SSO, SAML, OAuth, API, integration, HIPAA
- Person names with ownership language (assigned to, forwarded to, contact is)
- Dates and deadlines (by Friday, next week, Q1)
- Blockers: waiting on, blocked by, need, depends on"""
}


def create_lens():
    """Create the deal-flow lens in Memex"""
    print("Creating deal-flow lens in Memex...")

    try:
        response = requests.post(
            f"{MEMEX_URL}/api/lenses",
            json=DEAL_FLOW_LENS,
            timeout=10
        )

        if response.status_code == 200:
            result = response.json()
            print(f"Lens created successfully: {result.get('id')}")
            return True
        elif response.status_code == 409:
            print("Lens already exists, updating...")
            # Try to update instead
            response = requests.patch(
                f"{MEMEX_URL}/api/lenses/{DEAL_FLOW_LENS['id']}",
                json=DEAL_FLOW_LENS,
                timeout=10
            )
            if response.status_code == 200:
                print("Lens updated successfully")
                return True
            else:
                print(f"Failed to update lens: {response.status_code}")
                print(response.text)
                return False
        else:
            print(f"Failed to create lens: {response.status_code}")
            print(response.text)
            return False

    except requests.exceptions.ConnectionError:
        print(f"Could not connect to Memex at {MEMEX_URL}")
        print("Make sure Memex is running: cd /home/deocy/memex && go run cmd/memex/main.go")
        return False
    except Exception as e:
        print(f"Error: {e}")
        return False


def verify_lens():
    """Verify the lens was created correctly"""
    print("\nVerifying lens...")

    try:
        response = requests.get(
            f"{MEMEX_URL}/api/lenses/{DEAL_FLOW_LENS['id']}",
            timeout=10
        )

        if response.status_code == 200:
            lens = response.json()
            print(f"Lens: {lens.get('Meta', {}).get('name', 'Unknown')}")
            print(f"Primitives: {list(lens.get('Meta', {}).get('primitives', {}).keys())}")
            print(f"Patterns: {list(lens.get('Meta', {}).get('patterns', {}).keys())}")
            return True
        else:
            print(f"Could not retrieve lens: {response.status_code}")
            return False

    except Exception as e:
        print(f"Error: {e}")
        return False


def create_demo_users():
    """Create user nodes in Memex for the demo"""
    print("\nCreating demo users...")

    users = [
        {
            "id": "user:alex",
            "type": "User",
            "meta": {
                "name": "Alex",
                "role": "sales",
                "title": "Sales Rep",
                "email": "alex@company.com"
            }
        },
        {
            "id": "user:jordan",
            "type": "User",
            "meta": {
                "name": "Jordan",
                "role": "cs",
                "title": "Customer Success Manager",
                "email": "jordan@company.com"
            }
        },
        {
            "id": "user:sam",
            "type": "User",
            "meta": {
                "name": "Sam",
                "role": "engineering",
                "title": "Solutions Engineer",
                "email": "sam@company.com"
            }
        },
        {
            "id": "user:morgan",
            "type": "User",
            "meta": {
                "name": "Morgan",
                "role": "manager",
                "title": "VP Operations",
                "email": "morgan@company.com"
            }
        }
    ]

    for user in users:
        try:
            response = requests.post(
                f"{MEMEX_URL}/api/nodes",
                json=user,
                timeout=10
            )

            if response.status_code == 200:
                print(f"  Created user: {user['meta']['name']} ({user['meta']['role']})")
            elif response.status_code == 409:
                print(f"  User exists: {user['meta']['name']}")
            else:
                print(f"  Failed to create {user['meta']['name']}: {response.status_code}")

        except Exception as e:
            print(f"  Error creating {user['meta']['name']}: {e}")


if __name__ == "__main__":
    print("=" * 50)
    print("Memex Workspace - Demo Setup")
    print("=" * 50)

    # Create the lens
    if create_lens():
        verify_lens()

    # Create demo users
    create_demo_users()

    print("\n" + "=" * 50)
    print("Setup complete!")
    print("Next: Run seed_data.py to populate historical context")
    print("=" * 50)
