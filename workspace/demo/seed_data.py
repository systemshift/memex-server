"""
Seed historical data into Memex for the VC demo.

Creates:
- Past companies (HealthCo, MedTech) with deal history
- Past SSO implementations with issues/patterns
- Workflow history that Memex can reference

This data makes the demo "smart" - it can reference past similar work.
"""

import requests
import json
from datetime import datetime, timedelta

MEMEX_URL = "http://localhost:8080"


def create_node(node_data):
    """Create a node in Memex"""
    try:
        response = requests.post(
            f"{MEMEX_URL}/api/nodes",
            json=node_data,
            timeout=10
        )
        if response.status_code == 200:
            return True
        elif response.status_code == 409:
            return True  # Already exists
        else:
            print(f"Failed to create node {node_data.get('id')}: {response.status_code}")
            return False
    except Exception as e:
        print(f"Error: {e}")
        return False


def create_link(source, target, link_type, meta=None):
    """Create a link between nodes"""
    try:
        response = requests.post(
            f"{MEMEX_URL}/api/links",
            json={
                "source": source,
                "target": target,
                "type": link_type,
                "meta": meta or {}
            },
            timeout=10
        )
        return response.status_code == 200
    except Exception as e:
        print(f"Error creating link: {e}")
        return False


def seed_companies():
    """Create past customer companies"""
    print("Creating companies...")

    companies = [
        {
            "id": "company:healthco",
            "type": "Company",
            "meta": {
                "name": "HealthCo",
                "industry": "Healthcare",
                "size": "500-1000 employees",
                "compliance": ["HIPAA", "SOC2"],
                "notes": "Healthcare company, requires HIPAA compliance for all integrations"
            }
        },
        {
            "id": "company:medtech",
            "type": "Company",
            "meta": {
                "name": "MedTech Solutions",
                "industry": "Healthcare Technology",
                "size": "200-500 employees",
                "compliance": ["HIPAA"],
                "notes": "Medical device software, strict security requirements"
            }
        },
        {
            "id": "company:finserv",
            "type": "Company",
            "meta": {
                "name": "FinServ Inc",
                "industry": "Financial Services",
                "size": "1000+ employees",
                "compliance": ["SOC2", "PCI-DSS"],
                "notes": "Financial services, requires SOC2 compliance"
            }
        }
    ]

    for company in companies:
        if create_node(company):
            print(f"  Created: {company['meta']['name']}")


def seed_past_deals():
    """Create past deal records"""
    print("\nCreating past deals...")

    deals = [
        {
            "id": "deal:healthco-2025",
            "type": "Deal",
            "meta": {
                "company": "HealthCo",
                "company_id": "company:healthco",
                "amount": 75000,
                "amount_display": "$75k ARR",
                "requirement": "SSO integration with Okta",
                "contact": "Dr. Sarah Miller",
                "closed_by": "alex",
                "closed_date": (datetime.now() - timedelta(days=60)).isoformat(),
                "status": "complete",
                "implementation_time": "3 weeks",
                "notes": "SSO implementation had token expiry issues - took extra week to resolve"
            }
        },
        {
            "id": "deal:medtech-2025",
            "type": "Deal",
            "meta": {
                "company": "MedTech Solutions",
                "company_id": "company:medtech",
                "amount": 45000,
                "amount_display": "$45k ARR",
                "requirement": "API integration",
                "contact": "James Wong",
                "closed_by": "alex",
                "closed_date": (datetime.now() - timedelta(days=30)).isoformat(),
                "status": "complete",
                "implementation_time": "2 weeks",
                "notes": "Smooth implementation, good documentation helped"
            }
        },
        {
            "id": "deal:finserv-2024",
            "type": "Deal",
            "meta": {
                "company": "FinServ Inc",
                "company_id": "company:finserv",
                "amount": 120000,
                "amount_display": "$120k ARR",
                "requirement": "SSO with Azure AD",
                "contact": "Michael Chen",
                "closed_by": "alex",
                "closed_date": (datetime.now() - timedelta(days=90)).isoformat(),
                "status": "complete",
                "implementation_time": "4 weeks",
                "notes": "Complex SSO setup with multiple IdP configurations"
            }
        }
    ]

    for deal in deals:
        if create_node(deal):
            print(f"  Created: {deal['meta']['company']} - {deal['meta']['amount_display']}")
            # Link deal to company
            create_link(deal["id"], deal["meta"]["company_id"], "RELATES_TO")


def seed_implementations():
    """Create past implementation records with lessons learned"""
    print("\nCreating implementation records...")

    implementations = [
        {
            "id": "impl:healthco-sso",
            "type": "Implementation",
            "meta": {
                "title": "HealthCo SSO Implementation",
                "company": "HealthCo",
                "company_id": "company:healthco",
                "deal_id": "deal:healthco-2025",
                "type": "SSO",
                "idp": "Okta",
                "started": (datetime.now() - timedelta(days=55)).isoformat(),
                "completed": (datetime.now() - timedelta(days=34)).isoformat(),
                "duration_days": 21,
                "implemented_by": "sam",
                "status": "complete",
                "issues": [
                    {
                        "type": "token_expiry",
                        "description": "SAML token expiry was set to 5 minutes, caused session drops",
                        "resolution": "Extended token lifetime to 8 hours, added refresh logic",
                        "time_lost_days": 5
                    },
                    {
                        "type": "attribute_mapping",
                        "description": "User email attribute was named differently in Okta",
                        "resolution": "Added custom attribute mapping in SAML config",
                        "time_lost_days": 1
                    }
                ],
                "lessons_learned": [
                    "Always verify token expiry settings upfront",
                    "Request IdP attribute mapping documentation before starting",
                    "Healthcare companies often have stricter session requirements"
                ],
                "tips_for_next_time": "Check token expiry and attribute mappings first - these caused 80% of delays"
            }
        },
        {
            "id": "impl:finserv-sso",
            "type": "Implementation",
            "meta": {
                "title": "FinServ SSO Implementation",
                "company": "FinServ Inc",
                "company_id": "company:finserv",
                "deal_id": "deal:finserv-2024",
                "type": "SSO",
                "idp": "Azure AD",
                "started": (datetime.now() - timedelta(days=85)).isoformat(),
                "completed": (datetime.now() - timedelta(days=57)).isoformat(),
                "duration_days": 28,
                "implemented_by": "sam",
                "status": "complete",
                "issues": [
                    {
                        "type": "multi_tenant",
                        "description": "Azure AD multi-tenant setup required extra configuration",
                        "resolution": "Configured tenant-specific endpoints",
                        "time_lost_days": 3
                    }
                ],
                "lessons_learned": [
                    "Azure AD multi-tenant requires explicit tenant ID handling",
                    "Financial services need SOC2 audit trail for auth events"
                ],
                "tips_for_next_time": "Ask about multi-tenant vs single-tenant Azure AD setup early"
            }
        }
    ]

    for impl in implementations:
        if create_node(impl):
            print(f"  Created: {impl['meta']['title']}")
            # Link to deal and company
            create_link(impl["id"], impl["meta"]["deal_id"], "IMPLEMENTS")
            create_link(impl["id"], impl["meta"]["company_id"], "RELATES_TO")


def seed_knowledge_patterns():
    """Create knowledge patterns that help with future work"""
    print("\nCreating knowledge patterns...")

    patterns = [
        {
            "id": "pattern:sso-timeline",
            "type": "Pattern",
            "meta": {
                "title": "SSO Implementation Timeline",
                "category": "implementation",
                "content": "SSO implementations typically take 2-3 weeks. Healthcare/Finance add 1 week for compliance.",
                "based_on": ["impl:healthco-sso", "impl:finserv-sso"],
                "confidence": 0.85,
                "tags": ["sso", "timeline", "estimation"]
            }
        },
        {
            "id": "pattern:sso-issues",
            "type": "Pattern",
            "meta": {
                "title": "Common SSO Issues",
                "category": "troubleshooting",
                "content": "Most SSO delays come from: 1) Token expiry misconfiguration 2) Attribute mapping 3) Multi-tenant setup",
                "based_on": ["impl:healthco-sso", "impl:finserv-sso"],
                "confidence": 0.9,
                "tags": ["sso", "issues", "troubleshooting"]
            }
        },
        {
            "id": "pattern:healthcare-compliance",
            "type": "Pattern",
            "meta": {
                "title": "Healthcare Compliance Requirements",
                "category": "compliance",
                "content": "Healthcare customers require HIPAA compliance. Plan for: audit logging, encryption at rest, BAA agreement.",
                "based_on": ["company:healthco", "company:medtech"],
                "confidence": 0.95,
                "tags": ["healthcare", "hipaa", "compliance"]
            }
        }
    ]

    for pattern in patterns:
        if create_node(pattern):
            print(f"  Created: {pattern['meta']['title']}")


def seed_workflow_context():
    """Create context that links everything together"""
    print("\nCreating workflow context...")

    # Create a WorkflowType node that describes the deal-flow process
    workflow_type = {
        "id": "workflow:deal-flow",
        "type": "WorkflowType",
        "meta": {
            "name": "Deal Flow",
            "description": "Sales to implementation workflow",
            "stages": [
                {"name": "closed", "owner_role": "sales", "next": "onboarding"},
                {"name": "onboarding", "owner_role": "cs", "next": "implementation"},
                {"name": "implementation", "owner_role": "engineering", "next": "complete"},
                {"name": "complete", "owner_role": None, "next": None}
            ],
            "typical_duration_days": 21,
            "lens_id": "lens:deal-flow"
        }
    }

    if create_node(workflow_type):
        print(f"  Created: {workflow_type['meta']['name']} workflow type")

    # Link workflow to lens
    create_link(workflow_type["id"], "lens:deal-flow", "USES_LENS")


def verify_seed_data():
    """Verify seed data was created"""
    print("\nVerifying seed data...")

    checks = [
        ("Companies", "/api/query/search?q=Company&types=Company"),
        ("Deals", "/api/query/search?q=Deal&types=Deal"),
        ("Implementations", "/api/query/search?q=Implementation&types=Implementation"),
        ("Patterns", "/api/query/search?q=Pattern&types=Pattern"),
    ]

    for name, endpoint in checks:
        try:
            response = requests.get(f"{MEMEX_URL}{endpoint}", timeout=10)
            if response.status_code == 200:
                data = response.json()
                count = len(data.get("nodes", []))
                print(f"  {name}: {count} nodes")
            else:
                print(f"  {name}: query failed")
        except Exception as e:
            print(f"  {name}: error - {e}")


if __name__ == "__main__":
    print("=" * 50)
    print("Memex Workspace - Seed Historical Data")
    print("=" * 50)

    seed_companies()
    seed_past_deals()
    seed_implementations()
    seed_knowledge_patterns()
    seed_workflow_context()
    verify_seed_data()

    print("\n" + "=" * 50)
    print("Seed data complete!")
    print("")
    print("The system now knows about:")
    print("  - HealthCo: SSO implementation with token expiry issues")
    print("  - MedTech: Smooth API integration")
    print("  - FinServ: Azure AD multi-tenant complexity")
    print("")
    print("This context will appear in the demo when similar work comes up.")
    print("=" * 50)
