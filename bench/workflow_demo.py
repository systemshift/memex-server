#!/usr/bin/env python3
"""
Memex Workflow Demo - Memory-Aware Formless Workflows

Demonstrates prose-driven workflows backed by Memex (company memory):
- User writes natural text about what they need to do
- System detects workflow type and infers stage
- Queries Memex for relevant context (people, history, policies)
- Generates appropriate form with context already populated
- Stores interaction in Memex for future context

The key insight: Memex is the persistent company memory.
Workflows are ephemeral interfaces - the graph endures.

Usage:
    python workflow_demo.py
    # Open http://localhost:5003
"""

import json
import os
import re
import requests
from flask import Flask, request, jsonify, render_template_string
from flask_cors import CORS
from openai import OpenAI
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
CORS(app)

llm_client = OpenAI()
MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")

# ============== Workflow Definitions ==============

WORKFLOWS = {
    "expense": {
        "name": "Expense Reimbursement",
        "icon": "💰",
        "description": "Track expenses from submission to payment",
        "lens": {
            "primitives": {
                "amount": "monetary value to be reimbursed",
                "category": "type of expense (travel, meals, equipment, etc.)",
                "date": "when expense occurred",
                "vendor": "who was paid",
                "receipt": "proof of purchase",
                "approver": "person who approved",
                "status": "current state of request"
            }
        },
        "stages": [
            {"id": "submitted", "name": "Submitted", "triggers": ["need reimbursement", "please reimburse", "expense report", "spent"]},
            {"id": "manager_review", "name": "Manager Review", "triggers": ["reviewing", "under review", "pending approval"]},
            {"id": "approved", "name": "Approved", "triggers": ["approved", "looks good", "go ahead"]},
            {"id": "rejected", "name": "Rejected", "triggers": ["rejected", "denied", "cannot approve"]},
            {"id": "paid", "name": "Paid", "triggers": ["paid", "reimbursed", "payment sent", "deposited"]}
        ],
        "generates": {
            "submitted": "expense_form",
            "approved": "payment_request",
            "rejected": "rejection_notice"
        }
    },
    "hiring": {
        "name": "Hiring Request",
        "icon": "👤",
        "description": "Track hiring from request to offer",
        "lens": {
            "primitives": {
                "role": "job title being hired for",
                "team": "department or team",
                "level": "seniority level",
                "salary_range": "compensation budget",
                "justification": "reason for hire",
                "candidate": "person being considered",
                "interviewer": "person conducting interview",
                "decision": "hire/no-hire decision"
            }
        },
        "stages": [
            {"id": "requested", "name": "Requested", "triggers": ["need to hire", "looking for", "open role", "headcount"]},
            {"id": "approved", "name": "Approved", "triggers": ["approved to hire", "headcount approved", "go ahead and hire"]},
            {"id": "sourcing", "name": "Sourcing", "triggers": ["posted job", "sourcing candidates", "recruiting"]},
            {"id": "interviewing", "name": "Interviewing", "triggers": ["interviewing", "spoke with candidate", "interview scheduled"]},
            {"id": "offer", "name": "Offer", "triggers": ["extending offer", "offer sent", "offer accepted"]},
            {"id": "hired", "name": "Hired", "triggers": ["start date", "onboarding", "joined the team"]}
        ],
        "generates": {
            "requested": "job_requisition",
            "approved": "job_posting",
            "offer": "offer_letter"
        }
    },
    "contract": {
        "name": "Contract Request",
        "icon": "📝",
        "description": "Track contracts from request to signature",
        "lens": {
            "primitives": {
                "contract_type": "NDA, MSA, SOW, etc.",
                "counterparty": "other party to the contract",
                "value": "contract value if applicable",
                "term": "duration of agreement",
                "purpose": "reason for contract",
                "reviewer": "legal reviewer",
                "redlines": "requested changes"
            }
        },
        "stages": [
            {"id": "requested", "name": "Requested", "triggers": ["need contract", "need NDA", "need agreement", "draft contract"]},
            {"id": "drafting", "name": "Drafting", "triggers": ["drafting", "preparing contract", "working on"]},
            {"id": "review", "name": "Legal Review", "triggers": ["legal review", "reviewing contract", "under review"]},
            {"id": "negotiation", "name": "Negotiation", "triggers": ["redlines", "changes requested", "negotiating", "counterproposal"]},
            {"id": "signing", "name": "Signing", "triggers": ["ready for signature", "sent for signing", "please sign"]},
            {"id": "executed", "name": "Executed", "triggers": ["signed", "executed", "fully executed", "countersigned"]}
        ],
        "generates": {
            "requested": "contract_intake",
            "drafting": "contract_draft",
            "signing": "signature_request"
        }
    },
    "support": {
        "name": "Support Ticket",
        "icon": "🎫",
        "description": "Track support from report to resolution",
        "lens": {
            "primitives": {
                "issue": "problem description",
                "severity": "urgency level",
                "product": "affected product/feature",
                "customer": "who reported",
                "assignee": "who is handling",
                "resolution": "how it was fixed"
            }
        },
        "stages": [
            {"id": "reported", "name": "Reported", "triggers": ["issue", "problem", "bug", "not working", "broken", "help"]},
            {"id": "triaged", "name": "Triaged", "triggers": ["assigned to", "looking into", "investigating"]},
            {"id": "in_progress", "name": "In Progress", "triggers": ["working on", "fixing", "found the issue"]},
            {"id": "resolved", "name": "Resolved", "triggers": ["fixed", "resolved", "deployed fix", "should be working"]},
            {"id": "closed", "name": "Closed", "triggers": ["confirmed fixed", "closing ticket", "thank you"]}
        ],
        "generates": {
            "reported": "ticket_form",
            "resolved": "resolution_summary"
        }
    }
}

# ============== Preset Examples ==============

EXAMPLES = {
    "expense_new": {
        "workflow": "expense",
        "title": "New Expense",
        "text": """Had dinner with the Acme Corp team last night to discuss the partnership.
Bill came to $247.50 at Marea restaurant.
Receipt attached. Need reimbursement please."""
    },
    "expense_approved": {
        "workflow": "expense",
        "title": "Expense Approved",
        "text": """Re: Dinner expense $247.50

Looks good, approved. Valid client entertainment expense.

- Sarah (Manager)"""
    },
    "hiring_new": {
        "workflow": "hiring",
        "title": "New Hire Request",
        "text": """We need to hire a Senior Backend Engineer for the Payments team.

The current team is overloaded with the new checkout flow project.
Looking for someone with 5+ years experience, strong in Go or Python.
Budget is $180-220k depending on experience.

Can we get headcount approval?"""
    },
    "hiring_interview": {
        "workflow": "hiring",
        "title": "Interview Feedback",
        "text": """Just finished interviewing Alex Chen for the Senior Backend role.

Strong technical skills - solved the system design problem elegantly.
Good culture fit, asked thoughtful questions about our architecture.
7 years experience, currently at Stripe.

Recommend moving forward. Let's schedule the team round."""
    },
    "contract_new": {
        "workflow": "contract",
        "title": "Contract Request",
        "text": """Need an NDA with TechStart Inc before our product demo next week.

They're a potential customer, Series B startup in the fintech space.
Standard mutual NDA should work - 2 year term.

Contact: Jamie Lee, jamie@techstart.io"""
    },
    "contract_redlines": {
        "workflow": "contract",
        "title": "Contract Negotiation",
        "text": """TechStart came back with redlines on the NDA:

1. Want to extend term from 2 years to 3 years
2. Requesting carve-out for information already in public domain
3. Added their standard arbitration clause

Changes 1 and 2 seem reasonable. Need legal to review the arbitration language."""
    },
    "support_new": {
        "workflow": "support",
        "title": "Support Request",
        "text": """From: angry.customer@bigcorp.com
Subject: URGENT - Dashboard not loading

Our entire team can't access the analytics dashboard since this morning.
Getting a 500 error when we try to load it.
This is blocking our quarterly review meeting at 2pm!

Please help ASAP."""
    },
    "support_resolved": {
        "workflow": "support",
        "title": "Issue Resolved",
        "text": """Found the issue - there was a database connection pool exhaustion
due to a query optimization that backfired.

Rolled back the change and dashboard is loading again.
Deployed fix to prevent connection leaks.

Customer confirmed it's working on their end."""
    }
}


# ============== Memex Integration ==============

def memex_get(path):
    """GET request to memex API"""
    try:
        resp = requests.get(f"{MEMEX_URL}{path}", timeout=5)
        resp.raise_for_status()
        return resp.json()
    except:
        return None


def memex_post(path, data):
    """POST request to memex API"""
    try:
        resp = requests.post(f"{MEMEX_URL}{path}", json=data, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except:
        return None


def fetch_context_from_memex(workflow_id: str, extracted_data: dict) -> dict:
    """
    Query memex for relevant context based on workflow and extracted entities.
    This is what makes workflows memory-aware.
    """
    context = {
        "related_entities": [],
        "recent_similar": [],
        "policies": [],
        "org_context": []
    }

    # Extract entity names from the data to search for
    search_terms = []
    for key, value in extracted_data.items():
        if isinstance(value, str) and len(value) > 2:
            search_terms.append(value)

    # Search memex for related entities
    for term in search_terms[:3]:  # Limit searches
        result = memex_get(f"/api/query/search?q={term}&limit=5")
        if result and result.get("results"):
            for node in result["results"]:
                context["related_entities"].append({
                    "id": node.get("ID"),
                    "type": node.get("Type"),
                    "name": node.get("Meta", {}).get("name", node.get("ID")),
                    "matched_term": term
                })

    # Search for recent similar workflow items
    result = memex_get(f"/api/query/search?q={workflow_id}&limit=5")
    if result and result.get("results"):
        for node in result["results"]:
            meta = node.get("Meta", {})
            context["recent_similar"].append({
                "id": node.get("ID"),
                "type": node.get("Type"),
                "summary": meta.get("summary", meta.get("name", ""))
            })

    # Look for org structure (managers, approvers)
    if workflow_id == "expense":
        # In a real system, would query for user's manager
        context["org_context"].append({
            "role": "approver",
            "note": "Expenses over $500 require VP approval"
        })
    elif workflow_id == "hiring":
        context["org_context"].append({
            "role": "recruiter",
            "note": "Contact recruiting@company.com for job postings"
        })

    return context


def store_workflow_in_memex(workflow_id: str, stage_id: str, extracted_data: dict, original_text: str) -> str:
    """Store the workflow interaction in memex for future context"""
    # Ingest the original text as a source
    source_resp = memex_post("/api/ingest", {
        "content": original_text,
        "format": "text"
    })

    if not source_resp:
        return None

    source_id = source_resp.get("source_id")

    # Create a workflow node
    workflow_node_id = f"workflow:{workflow_id}:{source_id.split(':')[-1] if source_id else 'unknown'}"
    memex_post("/api/nodes", {
        "id": workflow_node_id,
        "type": "Workflow",
        "meta": {
            "workflow_type": workflow_id,
            "stage": stage_id,
            "extracted_data": extracted_data,
            "summary": extracted_data.get("summary", "")
        }
    })

    # Link workflow to source
    if source_id:
        memex_post("/api/links", {
            "source": workflow_node_id,
            "target": source_id,
            "type": "GENERATED_FROM"
        })

    return workflow_node_id


# ============== LLM Processing ==============

def detect_workflow_and_extract(text: str) -> dict:
    """Detect workflow type, stage, and extract relevant data"""

    workflows_desc = []
    for wf_id, wf in WORKFLOWS.items():
        stage_ids = [s["id"] for s in wf["stages"]]
        workflows_desc.append(
            f"- {wf_id}: {wf['name']} - {wf['description']}\n"
            f"  Stages (use exact IDs): {', '.join(stage_ids)}"
        )
    workflows_str = "\n".join(workflows_desc)

    prompt = f"""Analyze this text and determine:
1. Which workflow it belongs to
2. What stage of the workflow it represents
3. Extract relevant structured data

AVAILABLE WORKFLOWS:
{workflows_str}

TEXT TO ANALYZE:
{text}

Return JSON:
{{
    "workflow_id": "one of: expense, hiring, contract, support",
    "confidence": 0.0-1.0,
    "stage_id": "MUST be one of the exact stage IDs listed above",
    "stage_reasoning": "why this stage",
    "extracted_data": {{
        "field1": "value1",
        "field2": "value2"
    }},
    "summary": "one line summary of what this text is about"
}}

IMPORTANT: stage_id MUST be one of the exact stage IDs listed above (e.g., "submitted" not "submission")."""

    response = llm_client.chat.completions.create(
        model=MODEL,
        messages=[{"role": "user", "content": prompt}],
        response_format={"type": "json_object"}
    )

    return json.loads(response.choices[0].message.content)


def generate_form_output(workflow_id: str, stage_id: str, extracted_data: dict) -> dict:
    """Generate the appropriate form/document based on workflow and stage"""

    workflow = WORKFLOWS.get(workflow_id, {})
    generates = workflow.get("generates", {})
    output_type = generates.get(stage_id, "generic_summary")

    # Define form templates
    form_templates = {
        "expense_form": {
            "title": "Expense Reimbursement Request",
            "fields": [
                {"name": "amount", "label": "Amount", "type": "currency"},
                {"name": "category", "label": "Category", "type": "select", "options": ["Meals", "Travel", "Equipment", "Software", "Other"]},
                {"name": "date", "label": "Date", "type": "date"},
                {"name": "vendor", "label": "Vendor/Restaurant", "type": "text"},
                {"name": "description", "label": "Description", "type": "textarea"},
                {"name": "receipt", "label": "Receipt", "type": "file"}
            ],
            "actions": ["Submit for Approval", "Save Draft"]
        },
        "payment_request": {
            "title": "Payment Authorization",
            "fields": [
                {"name": "payee", "label": "Pay To", "type": "text"},
                {"name": "amount", "label": "Amount", "type": "currency"},
                {"name": "account", "label": "Deposit Account", "type": "text"},
                {"name": "approved_by", "label": "Approved By", "type": "text"},
                {"name": "cost_center", "label": "Cost Center", "type": "text"}
            ],
            "actions": ["Process Payment", "Hold"]
        },
        "job_requisition": {
            "title": "Job Requisition Form",
            "fields": [
                {"name": "title", "label": "Job Title", "type": "text"},
                {"name": "department", "label": "Department", "type": "text"},
                {"name": "level", "label": "Level", "type": "select", "options": ["Junior", "Mid", "Senior", "Lead", "Principal"]},
                {"name": "salary_min", "label": "Salary Min", "type": "currency"},
                {"name": "salary_max", "label": "Salary Max", "type": "currency"},
                {"name": "justification", "label": "Business Justification", "type": "textarea"},
                {"name": "hiring_manager", "label": "Hiring Manager", "type": "text"}
            ],
            "actions": ["Submit for Approval", "Save Draft"]
        },
        "job_posting": {
            "title": "Job Posting",
            "fields": [
                {"name": "title", "label": "Job Title", "type": "text"},
                {"name": "description", "label": "Job Description", "type": "richtext"},
                {"name": "requirements", "label": "Requirements", "type": "textarea"},
                {"name": "benefits", "label": "Benefits", "type": "textarea"},
                {"name": "location", "label": "Location", "type": "text"},
                {"name": "remote", "label": "Remote OK", "type": "checkbox"}
            ],
            "actions": ["Publish", "Preview"]
        },
        "offer_letter": {
            "title": "Offer Letter",
            "fields": [
                {"name": "candidate_name", "label": "Candidate Name", "type": "text"},
                {"name": "title", "label": "Position", "type": "text"},
                {"name": "salary", "label": "Annual Salary", "type": "currency"},
                {"name": "start_date", "label": "Start Date", "type": "date"},
                {"name": "equity", "label": "Equity Grant", "type": "text"},
                {"name": "bonus", "label": "Signing Bonus", "type": "currency"}
            ],
            "actions": ["Generate Letter", "Send to Candidate"]
        },
        "contract_intake": {
            "title": "Contract Request",
            "fields": [
                {"name": "contract_type", "label": "Contract Type", "type": "select", "options": ["NDA", "MSA", "SOW", "DPA", "Other"]},
                {"name": "counterparty", "label": "Other Party", "type": "text"},
                {"name": "contact_email", "label": "Contact Email", "type": "email"},
                {"name": "purpose", "label": "Purpose", "type": "textarea"},
                {"name": "term", "label": "Term (months)", "type": "number"},
                {"name": "urgency", "label": "Urgency", "type": "select", "options": ["Low", "Medium", "High", "Urgent"]}
            ],
            "actions": ["Submit to Legal", "Save Draft"]
        },
        "contract_draft": {
            "title": "Contract Draft",
            "content_type": "document",
            "sections": ["Parties", "Purpose", "Confidentiality", "Term", "Governing Law", "Signatures"]
        },
        "signature_request": {
            "title": "Signature Request",
            "fields": [
                {"name": "document", "label": "Document", "type": "file"},
                {"name": "signers", "label": "Signers", "type": "multi-email"},
                {"name": "message", "label": "Message", "type": "textarea"},
                {"name": "deadline", "label": "Sign By", "type": "date"}
            ],
            "actions": ["Send for Signature", "Download PDF"]
        },
        "ticket_form": {
            "title": "Support Ticket",
            "fields": [
                {"name": "subject", "label": "Subject", "type": "text"},
                {"name": "severity", "label": "Severity", "type": "select", "options": ["Low", "Medium", "High", "Critical"]},
                {"name": "product", "label": "Product Area", "type": "select", "options": ["Dashboard", "API", "Billing", "Other"]},
                {"name": "description", "label": "Description", "type": "textarea"},
                {"name": "customer", "label": "Customer", "type": "text"},
                {"name": "contact", "label": "Contact Email", "type": "email"}
            ],
            "actions": ["Create Ticket", "Assign"]
        },
        "resolution_summary": {
            "title": "Resolution Summary",
            "fields": [
                {"name": "root_cause", "label": "Root Cause", "type": "textarea"},
                {"name": "resolution", "label": "Resolution", "type": "textarea"},
                {"name": "time_to_resolve", "label": "Time to Resolve", "type": "text"},
                {"name": "follow_up", "label": "Follow-up Actions", "type": "textarea"},
                {"name": "preventive", "label": "Preventive Measures", "type": "textarea"}
            ],
            "actions": ["Close Ticket", "Send to Customer"]
        },
        "generic_summary": {
            "title": "Status Update",
            "fields": [
                {"name": "summary", "label": "Summary", "type": "textarea"},
                {"name": "next_steps", "label": "Next Steps", "type": "textarea"}
            ],
            "actions": ["Save", "Share"]
        }
    }

    template = form_templates.get(output_type, form_templates["generic_summary"])

    # Pre-fill form with extracted data
    prefilled = {}
    for field in template.get("fields", []):
        field_name = field["name"]
        # Try to match extracted data to field names (fuzzy)
        for key, value in extracted_data.items():
            if key.lower() in field_name.lower() or field_name.lower() in key.lower():
                prefilled[field_name] = value
                break
            # Also try partial matches
            if any(word in field_name.lower() for word in key.lower().split("_")):
                prefilled[field_name] = value
                break

    return {
        "output_type": output_type,
        "template": template,
        "prefilled": prefilled
    }


# ============== Routes ==============

@app.route('/')
def index():
    return render_template_string(HTML_TEMPLATE)


@app.route('/api/examples')
def get_examples():
    """Get all preset examples"""
    return jsonify(EXAMPLES)


@app.route('/api/workflows')
def get_workflows():
    """Get all workflow definitions"""
    return jsonify(WORKFLOWS)


@app.route('/api/process', methods=['POST'])
def process_text():
    """Process text through workflow detection"""
    data = request.json
    text = data.get("text", "")
    store = data.get("store", True)  # Store in memex by default

    if not text.strip():
        return jsonify({"error": "No text provided"}), 400

    try:
        # Detect workflow and extract data
        analysis = detect_workflow_and_extract(text)

        workflow_id = analysis.get("workflow_id", "")
        stage_id = analysis.get("stage_id", "")
        extracted = analysis.get("extracted_data", {})

        # Get workflow info
        workflow = WORKFLOWS.get(workflow_id, {})

        # Find stage info
        stage_info = None
        stage_index = 0
        for i, stage in enumerate(workflow.get("stages", [])):
            if stage["id"] == stage_id:
                stage_info = stage
                stage_index = i
                break

        # Fetch context from memex (company memory)
        context = fetch_context_from_memex(workflow_id, extracted)

        # Generate form output
        form_output = generate_form_output(workflow_id, stage_id, extracted)

        # Store in memex for future context
        workflow_node_id = None
        if store:
            workflow_node_id = store_workflow_in_memex(workflow_id, stage_id, extracted, text)

        return jsonify({
            "workflow": {
                "id": workflow_id,
                "name": workflow.get("name", "Unknown"),
                "icon": workflow.get("icon", "📋"),
                "stages": workflow.get("stages", [])
            },
            "analysis": {
                "stage_id": stage_id,
                "stage_name": stage_info["name"] if stage_info else "Unknown",
                "stage_index": stage_index,
                "confidence": analysis.get("confidence", 0),
                "reasoning": analysis.get("stage_reasoning", ""),
                "summary": analysis.get("summary", "")
            },
            "extracted_data": extracted,
            "generated_form": form_output,
            "memex_context": context,
            "stored_as": workflow_node_id
        })

    except Exception as e:
        return jsonify({"error": str(e)}), 500


# ============== HTML Template ==============

HTML_TEMPLATE = """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Anchor Flow - Formless Workflows</title>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;600&family=Space+Grotesk:wght@400;500;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #0a0a0a;
            --bg-secondary: #111111;
            --bg-tertiary: #1a1a1a;
            --text: #e0e0e0;
            --text-dim: #707070;
            --accent: #00ff88;
            --accent-dim: #00aa5a;
            --border: #222;
            --expense: #ff6b6b;
            --hiring: #4ecdc4;
            --contract: #a29bfe;
            --support: #ffeaa7;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Space Grotesk', sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            min-height: 100vh;
        }

        .mono { font-family: 'JetBrains Mono', monospace; }

        header {
            border-bottom: 1px solid var(--border);
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .logo {
            font-family: 'JetBrains Mono', monospace;
            font-weight: 600;
            font-size: 1.2rem;
            color: var(--accent);
        }
        .logo span { color: var(--text-dim); }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }

        .intro {
            text-align: center;
            margin-bottom: 2rem;
        }

        .intro h1 {
            font-size: 1.8rem;
            margin-bottom: 0.5rem;
        }

        .intro p {
            color: var(--text-dim);
        }

        .main-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 2rem;
        }

        .panel {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 8px;
            overflow: hidden;
        }

        .panel-header {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.85rem;
            color: var(--accent);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .panel-body {
            padding: 1rem;
        }

        /* Input panel */
        .examples-grid {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 0.5rem;
            margin-bottom: 1rem;
        }

        .example-btn {
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            border-radius: 4px;
            cursor: pointer;
            text-align: center;
        }

        .example-btn:hover {
            border-color: var(--accent);
            color: var(--accent);
        }

        .example-btn .icon {
            font-size: 1.2rem;
            display: block;
            margin-bottom: 0.25rem;
        }

        textarea {
            width: 100%;
            min-height: 200px;
            background: var(--bg-tertiary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 1rem;
            font-family: 'Space Grotesk', sans-serif;
            font-size: 0.95rem;
            line-height: 1.6;
            border-radius: 4px;
            resize: vertical;
        }

        textarea:focus {
            outline: none;
            border-color: var(--accent);
        }

        .process-btn {
            width: 100%;
            margin-top: 1rem;
            background: var(--accent);
            color: #000;
            border: none;
            padding: 0.75rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9rem;
            font-weight: 500;
            border-radius: 4px;
            cursor: pointer;
        }

        .process-btn:hover { background: var(--accent-dim); color: #fff; }
        .process-btn:disabled { background: var(--border); cursor: not-allowed; }

        /* Output panel */
        .output-empty {
            text-align: center;
            padding: 3rem;
            color: var(--text-dim);
        }

        .workflow-detected {
            display: flex;
            align-items: center;
            gap: 1rem;
            padding: 1rem;
            background: var(--bg-tertiary);
            border-radius: 6px;
            margin-bottom: 1rem;
        }

        .workflow-icon {
            font-size: 2rem;
        }

        .workflow-info h3 {
            font-size: 1.1rem;
            margin-bottom: 0.25rem;
        }

        .workflow-info .summary {
            font-size: 0.85rem;
            color: var(--text-dim);
        }

        .confidence-badge {
            margin-left: auto;
            background: var(--accent);
            color: #000;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
        }

        /* Stage progress */
        .stage-progress {
            margin-bottom: 1.5rem;
        }

        .stage-track {
            display: flex;
            gap: 0.5rem;
            margin-top: 0.5rem;
        }

        .stage-dot {
            flex: 1;
            height: 4px;
            background: var(--border);
            border-radius: 2px;
            position: relative;
        }

        .stage-dot.active {
            background: var(--accent);
        }

        .stage-dot.current::after {
            content: '';
            position: absolute;
            top: -4px;
            left: 50%;
            transform: translateX(-50%);
            width: 12px;
            height: 12px;
            background: var(--accent);
            border-radius: 50%;
        }

        .stage-labels {
            display: flex;
            justify-content: space-between;
            margin-top: 0.75rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.65rem;
            color: var(--text-dim);
        }

        .stage-labels span.active {
            color: var(--accent);
        }

        /* Extracted data */
        .extracted-data {
            margin-bottom: 1.5rem;
        }

        .section-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.1em;
            margin-bottom: 0.75rem;
        }

        .data-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 0.5rem;
        }

        .data-item {
            background: var(--bg-tertiary);
            padding: 0.5rem 0.75rem;
            border-radius: 4px;
            font-size: 0.85rem;
        }

        .data-item .key {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--text-dim);
            text-transform: uppercase;
        }

        /* Memex context */
        .memex-context {
            margin-bottom: 1.5rem;
            padding: 1rem;
            background: rgba(0, 255, 136, 0.05);
            border: 1px solid var(--accent-dim);
            border-radius: 6px;
        }

        .context-grid {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
        }

        .context-section {
            padding: 0.5rem;
            background: var(--bg-tertiary);
            border-radius: 4px;
        }

        .context-label {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--accent);
            margin-bottom: 0.25rem;
        }

        .context-item {
            font-size: 0.85rem;
            color: var(--text-secondary);
            padding: 0.25rem 0;
        }

        .context-item.empty {
            color: var(--text-dim);
            font-style: italic;
        }

        .stored-notice {
            margin-top: 0.75rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.7rem;
            color: var(--text-dim);
        }

        /* Generated form */
        .generated-form {
            background: var(--bg-tertiary);
            border: 1px solid var(--accent-dim);
            border-radius: 6px;
            overflow: hidden;
        }

        .form-header {
            padding: 0.75rem 1rem;
            background: var(--accent);
            color: #000;
            font-family: 'JetBrains Mono', monospace;
            font-weight: 500;
        }

        .form-body {
            padding: 1rem;
        }

        .form-field {
            margin-bottom: 1rem;
        }

        .form-field label {
            display: block;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.75rem;
            color: var(--text-dim);
            margin-bottom: 0.25rem;
        }

        .form-field input,
        .form-field select,
        .form-field textarea {
            width: 100%;
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            color: var(--text);
            padding: 0.5rem;
            font-family: 'Space Grotesk', sans-serif;
            font-size: 0.9rem;
            border-radius: 4px;
        }

        .form-field input.prefilled,
        .form-field select.prefilled,
        .form-field textarea.prefilled {
            border-color: var(--accent-dim);
            background: rgba(0, 255, 136, 0.05);
        }

        .form-actions {
            display: flex;
            gap: 0.5rem;
            margin-top: 1rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }

        .form-action-btn {
            flex: 1;
            padding: 0.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8rem;
            border-radius: 4px;
            cursor: pointer;
            border: 1px solid var(--border);
            background: var(--bg-secondary);
            color: var(--text);
        }

        .form-action-btn.primary {
            background: var(--accent);
            color: #000;
            border-color: var(--accent);
        }

        .reasoning {
            margin-top: 1rem;
            padding: 0.75rem;
            background: var(--bg);
            border-radius: 4px;
            font-size: 0.8rem;
            color: var(--text-dim);
        }

        .reasoning strong {
            color: var(--accent);
        }

        @media (max-width: 1024px) {
            .main-grid {
                grid-template-columns: 1fr;
            }
            .examples-grid {
                grid-template-columns: repeat(2, 1fr);
            }
        }
    </style>
</head>
<body>
    <header>
        <div class="logo">anchor<span>.flow</span></div>
        <div class="mono" style="color: var(--text-dim); font-size: 0.8rem;">Formless Workflows</div>
    </header>

    <div class="container">
        <div class="intro">
            <h1>Write naturally. Workflows emerge.</h1>
            <p>No forms to fill. Just describe what happened - the system detects the workflow, stage, and generates what's needed.</p>
        </div>

        <div class="main-grid">
            <!-- Input Panel -->
            <div class="panel">
                <div class="panel-header">
                    <span>Input</span>
                    <span style="color: var(--text-dim);">Click an example or write your own</span>
                </div>
                <div class="panel-body">
                    <div class="examples-grid" id="examples-grid"></div>
                    <textarea id="input-text" placeholder="Describe what happened, what you need, or paste an email/message..."></textarea>
                    <button class="process-btn" id="process-btn" onclick="processText()">Process →</button>
                </div>
            </div>

            <!-- Output Panel -->
            <div class="panel">
                <div class="panel-header">
                    <span>Generated Output</span>
                </div>
                <div class="panel-body" id="output-container">
                    <div class="output-empty">
                        <p>Process some text to see the detected workflow and generated form</p>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let examples = {};

        // Load examples on start
        document.addEventListener('DOMContentLoaded', async () => {
            const resp = await fetch('/api/examples');
            examples = await resp.json();
            renderExamples();
        });

        function renderExamples() {
            const grid = document.getElementById('examples-grid');
            grid.innerHTML = Object.entries(examples).map(([id, ex]) => `
                <button class="example-btn" onclick="loadExample('${id}')">
                    <span class="icon">${getWorkflowIcon(ex.workflow)}</span>
                    ${ex.title}
                </button>
            `).join('');
        }

        function getWorkflowIcon(workflow) {
            const icons = { expense: '💰', hiring: '👤', contract: '📝', support: '🎫' };
            return icons[workflow] || '📋';
        }

        function loadExample(id) {
            const ex = examples[id];
            if (ex) {
                document.getElementById('input-text').value = ex.text;
            }
        }

        async function processText() {
            const text = document.getElementById('input-text').value;
            if (!text.trim()) {
                alert('Please enter some text');
                return;
            }

            const btn = document.getElementById('process-btn');
            const container = document.getElementById('output-container');

            btn.disabled = true;
            btn.textContent = 'Processing...';
            container.innerHTML = '<div class="output-empty">Analyzing text...</div>';

            try {
                const resp = await fetch('/api/process', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ text })
                });

                const data = await resp.json();

                if (data.error) {
                    container.innerHTML = `<div class="output-empty">Error: ${data.error}</div>`;
                    return;
                }

                renderOutput(data);

            } catch (e) {
                container.innerHTML = `<div class="output-empty">Error: ${e.message}</div>`;
            } finally {
                btn.disabled = false;
                btn.textContent = 'Process →';
            }
        }

        function renderOutput(data) {
            const container = document.getElementById('output-container');
            const { workflow, analysis, extracted_data, generated_form, memex_context, stored_as } = data;

            // Stage progress
            const stageHtml = workflow.stages.map((stage, i) => {
                const isActive = i <= analysis.stage_index;
                const isCurrent = i === analysis.stage_index;
                return `<div class="stage-dot ${isActive ? 'active' : ''} ${isCurrent ? 'current' : ''}"></div>`;
            }).join('');

            const stageLabelHtml = workflow.stages.map((stage, i) => {
                const isActive = i <= analysis.stage_index;
                return `<span class="${isActive ? 'active' : ''}">${stage.name}</span>`;
            }).join('');

            // Extracted data
            const dataHtml = Object.entries(extracted_data).map(([key, value]) => `
                <div class="data-item">
                    <div class="key">${key.replace(/_/g, ' ')}</div>
                    <div>${value}</div>
                </div>
            `).join('');

            // Form fields
            const formFieldsHtml = (generated_form.template.fields || []).map(field => {
                const value = generated_form.prefilled[field.name] || '';
                const prefilled = value ? 'prefilled' : '';

                if (field.type === 'select') {
                    const options = (field.options || []).map(opt =>
                        `<option ${opt === value ? 'selected' : ''}>${opt}</option>`
                    ).join('');
                    return `
                        <div class="form-field">
                            <label>${field.label}</label>
                            <select class="${prefilled}">${options}</select>
                        </div>
                    `;
                } else if (field.type === 'textarea') {
                    return `
                        <div class="form-field">
                            <label>${field.label}</label>
                            <textarea class="${prefilled}" rows="3">${value}</textarea>
                        </div>
                    `;
                } else {
                    return `
                        <div class="form-field">
                            <label>${field.label}</label>
                            <input type="${field.type === 'currency' ? 'text' : field.type}"
                                   class="${prefilled}"
                                   value="${value}"
                                   placeholder="${field.type === 'currency' ? '$0.00' : ''}">
                        </div>
                    `;
                }
            }).join('');

            // Form actions
            const actionsHtml = (generated_form.template.actions || []).map((action, i) => `
                <button class="form-action-btn ${i === 0 ? 'primary' : ''}">${action}</button>
            `).join('');

            container.innerHTML = `
                <div class="workflow-detected">
                    <span class="workflow-icon">${workflow.icon}</span>
                    <div class="workflow-info">
                        <h3>${workflow.name}</h3>
                        <div class="summary">${analysis.summary}</div>
                    </div>
                    <span class="confidence-badge">${Math.round(analysis.confidence * 100)}% confidence</span>
                </div>

                <div class="stage-progress">
                    <div class="section-label">Workflow Stage</div>
                    <div class="stage-track">${stageHtml}</div>
                    <div class="stage-labels">${stageLabelHtml}</div>
                </div>

                <div class="extracted-data">
                    <div class="section-label">Extracted Data</div>
                    <div class="data-grid">${dataHtml || '<div class="data-item">No structured data extracted</div>'}</div>
                </div>

                <div class="memex-context">
                    <div class="section-label">📚 Company Memory (Memex)</div>
                    <div class="context-grid">
                        ${memex_context.related_entities.length > 0 ? `
                            <div class="context-section">
                                <div class="context-label">Related Entities</div>
                                ${memex_context.related_entities.slice(0, 3).map(e => `
                                    <div class="context-item">${e.type}: ${e.name}</div>
                                `).join('')}
                            </div>
                        ` : ''}
                        ${memex_context.recent_similar.length > 0 ? `
                            <div class="context-section">
                                <div class="context-label">Recent Similar</div>
                                ${memex_context.recent_similar.slice(0, 3).map(e => `
                                    <div class="context-item">${e.summary || e.id}</div>
                                `).join('')}
                            </div>
                        ` : ''}
                        ${memex_context.org_context.length > 0 ? `
                            <div class="context-section">
                                <div class="context-label">Org Context</div>
                                ${memex_context.org_context.map(e => `
                                    <div class="context-item">${e.note}</div>
                                `).join('')}
                            </div>
                        ` : ''}
                        ${memex_context.related_entities.length === 0 &&
                          memex_context.recent_similar.length === 0 &&
                          memex_context.org_context.length === 0 ? `
                            <div class="context-item empty">No prior context found in memory</div>
                        ` : ''}
                    </div>
                    ${stored_as ? `<div class="stored-notice">Stored as: ${stored_as}</div>` : ''}
                </div>

                <div class="generated-form">
                    <div class="form-header">${generated_form.template.title}</div>
                    <div class="form-body">
                        ${formFieldsHtml}
                        <div class="form-actions">${actionsHtml}</div>
                    </div>
                </div>

                <div class="reasoning">
                    <strong>Stage reasoning:</strong> ${analysis.reasoning}
                </div>
            `;
        }
    </script>
</body>
</html>
"""


if __name__ == '__main__':
    print("Starting Anchor Flow Demo...")
    print("Open http://localhost:5003")
    app.run(host='0.0.0.0', port=5003, debug=True)
