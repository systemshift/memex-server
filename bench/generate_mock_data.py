#!/usr/bin/env python3
"""
Mock Company Data Generator for Memex Demo

Generates interconnected mock data simulating a tech company's communications:
- Emails
- Slack messages
- Documents (meeting notes, proposals, specs)
- Calendar events
- Simple ERP-like data (invoices, purchase orders)

All data tells coherent "stories" with connected people, projects, and decisions.
"""

import json
import random
from datetime import datetime, timedelta
from pathlib import Path

OUTPUT_DIR = Path(__file__).parent / "mock_data"

# Company: "Nexus Technologies" - a B2B SaaS startup
COMPANY = "Nexus Technologies"

# People (employees)
EMPLOYEES = [
    {"name": "Sarah Chen", "email": "sarah.chen@nexustech.io", "role": "CEO", "slack": "sarah.chen"},
    {"name": "Marcus Williams", "email": "marcus.williams@nexustech.io", "role": "CTO", "slack": "marcus.w"},
    {"name": "Emily Rodriguez", "email": "emily.rodriguez@nexustech.io", "role": "VP Sales", "slack": "emily.r"},
    {"name": "David Kim", "email": "david.kim@nexustech.io", "role": "Engineering Lead", "slack": "david.kim"},
    {"name": "Rachel Foster", "email": "rachel.foster@nexustech.io", "role": "Product Manager", "slack": "rachel.f"},
    {"name": "James Mitchell", "email": "james.mitchell@nexustech.io", "role": "Finance Director", "slack": "james.m"},
    {"name": "Lisa Park", "email": "lisa.park@nexustech.io", "role": "Customer Success", "slack": "lisa.park"},
    {"name": "Alex Thompson", "email": "alex.thompson@nexustech.io", "role": "Senior Engineer", "slack": "alex.t"},
    {"name": "Michelle Lee", "email": "michelle.lee@nexustech.io", "role": "Marketing Lead", "slack": "michelle.l"},
    {"name": "Robert Garcia", "email": "robert.garcia@nexustech.io", "role": "DevOps Engineer", "slack": "robert.g"},
]

# External contacts (clients, vendors, partners)
EXTERNALS = [
    {"name": "John Harrison", "email": "jharrison@acmecorp.com", "company": "Acme Corporation", "role": "VP Engineering"},
    {"name": "Patricia Wong", "email": "pwong@acmecorp.com", "company": "Acme Corporation", "role": "Procurement Lead"},
    {"name": "Michael Stone", "email": "mstone@globalretail.com", "company": "Global Retail Inc", "role": "CIO"},
    {"name": "Jennifer Adams", "email": "jadams@techventures.vc", "company": "Tech Ventures", "role": "Partner"},
    {"name": "Thomas Brown", "email": "tbrown@cloudservices.net", "company": "CloudServices", "role": "Account Manager"},
    {"name": "Amanda White", "email": "awhite@legalfirm.com", "company": "Legal Partners LLP", "role": "Corporate Counsel"},
]

# Projects/initiatives
PROJECTS = [
    {"name": "Project Phoenix", "description": "Platform rewrite with new architecture", "lead": "Marcus Williams"},
    {"name": "Acme Enterprise Deal", "description": "500-seat enterprise deployment", "lead": "Emily Rodriguez"},
    {"name": "Q4 Budget Planning", "description": "Annual budget and hiring plan", "lead": "James Mitchell"},
    {"name": "Series A Fundraise", "description": "Raising $15M Series A", "lead": "Sarah Chen"},
    {"name": "SOC2 Compliance", "description": "Security certification process", "lead": "Robert Garcia"},
]

# Slack channels
CHANNELS = ["#general", "#engineering", "#sales", "#product", "#leadership", "#random"]

# Base date for mock data
BASE_DATE = datetime(2024, 10, 1)


def random_date(days_range=90):
    """Generate random date within range."""
    return BASE_DATE + timedelta(days=random.randint(0, days_range), hours=random.randint(8, 18), minutes=random.randint(0, 59))


def format_date(dt):
    return dt.strftime("%Y-%m-%d %H:%M:%S")


# ============================================================================
# STORY 1: Acme Corporation Enterprise Deal
# ============================================================================

def generate_acme_deal_story():
    """Generate the Acme Corp deal negotiation story."""
    emails = []
    slacks = []
    docs = []
    events = []

    # Email thread 1: Initial inquiry
    emails.append({
        "id": "email-acme-001",
        "type": "Email",
        "from": "jharrison@acmecorp.com",
        "to": ["emily.rodriguez@nexustech.io"],
        "cc": [],
        "subject": "Nexus Platform - Enterprise Inquiry",
        "body": """Hi Emily,

We spoke briefly at the SaaS Connect conference last week. I've been looking into your platform for our engineering teams.

Acme Corp has about 500 engineers across 3 offices. We're currently using a mix of tools for knowledge management but nothing integrated. Your demo impressed our team.

Can we schedule a call to discuss enterprise pricing and deployment options?

Best,
John Harrison
VP Engineering, Acme Corporation""",
        "date": format_date(BASE_DATE + timedelta(days=1)),
        "thread_id": "thread-acme-initial",
    })

    emails.append({
        "id": "email-acme-002",
        "type": "Email",
        "from": "emily.rodriguez@nexustech.io",
        "to": ["jharrison@acmecorp.com"],
        "cc": ["sarah.chen@nexustech.io"],
        "subject": "Re: Nexus Platform - Enterprise Inquiry",
        "body": """John,

Great to hear from you! It was a pleasure meeting at SaaS Connect.

A 500-seat deployment is exactly the kind of enterprise use case we've been building for. I'd love to walk you through our enterprise features:
- SSO integration (Okta, Azure AD)
- On-premise or private cloud deployment
- Advanced admin controls and audit logging
- Dedicated support and SLA guarantees

Let me know your availability next week. I'll loop in our CTO Marcus for the technical discussion.

Best,
Emily Rodriguez
VP Sales, Nexus Technologies""",
        "date": format_date(BASE_DATE + timedelta(days=2)),
        "thread_id": "thread-acme-initial",
    })

    # Slack discussion about the deal
    slacks.extend([
        {
            "id": "slack-acme-001",
            "type": "Slack",
            "channel": "#sales",
            "author": "emily.r",
            "content": "Big opportunity incoming! Acme Corp just reached out - 500 seat enterprise deal. @sarah.chen @marcus.w let's sync on this",
            "date": format_date(BASE_DATE + timedelta(days=2, hours=2)),
            "thread_id": "slack-thread-acme-1",
        },
        {
            "id": "slack-acme-002",
            "type": "Slack",
            "channel": "#sales",
            "author": "sarah.chen",
            "content": "This is huge! Acme is a Fortune 500. Let's make sure we nail this. What's their timeline?",
            "date": format_date(BASE_DATE + timedelta(days=2, hours=2, minutes=15)),
            "thread_id": "slack-thread-acme-1",
        },
        {
            "id": "slack-acme-003",
            "type": "Slack",
            "channel": "#sales",
            "author": "emily.r",
            "content": "They want to deploy by Q1. I'm setting up a technical deep dive with their VP Eng John Harrison next week",
            "date": format_date(BASE_DATE + timedelta(days=2, hours=2, minutes=30)),
            "thread_id": "slack-thread-acme-1",
        },
    ])

    # Technical call meeting notes
    docs.append({
        "id": "doc-acme-tech-call",
        "type": "Document",
        "title": "Acme Corp Technical Discovery Call - Meeting Notes",
        "author": "Marcus Williams",
        "content": """# Acme Corp Technical Discovery Call
**Date:** October 10, 2024
**Attendees:** John Harrison (Acme), Patricia Wong (Acme), Marcus Williams, Emily Rodriguez

## Current State
- Acme uses Confluence + custom wiki + scattered docs
- 500 engineers across San Francisco, Austin, and NYC offices
- Pain points: information silos, onboarding takes 3+ months, tribal knowledge loss

## Technical Requirements
- Must integrate with their existing Okta SSO
- Need to support their GitHub Enterprise instance
- Compliance requirement: data must stay in US region
- Want ability to connect Slack and email archives

## Architecture Discussion
- Discussed our knowledge graph approach vs traditional RAG
- John was impressed by the entity linking and relationship discovery
- They want a POC with a subset of their docs (engineering handbook)

## Next Steps
- [ ] Send POC proposal by Oct 15
- [ ] Marcus to prepare technical architecture doc
- [ ] Emily to draft pricing for 500 seats

## Concerns Raised
- Patricia asked about SOC2 - we told her it's in progress (Q1 target)
- Data migration complexity from their existing wiki""",
        "date": format_date(BASE_DATE + timedelta(days=10)),
        "folder": "Sales/Acme Corp",
    })

    # Calendar event for the call
    events.append({
        "id": "cal-acme-tech-001",
        "type": "Calendar",
        "title": "Acme Corp Technical Deep Dive",
        "organizer": "emily.rodriguez@nexustech.io",
        "attendees": ["marcus.williams@nexustech.io", "jharrison@acmecorp.com", "pwong@acmecorp.com"],
        "date": format_date(BASE_DATE + timedelta(days=10)),
        "duration_minutes": 60,
        "location": "Zoom",
        "description": "Technical discovery call with Acme Corp engineering leadership to discuss enterprise deployment requirements.",
    })

    # Pricing negotiation emails
    emails.append({
        "id": "email-acme-003",
        "type": "Email",
        "from": "emily.rodriguez@nexustech.io",
        "to": ["jharrison@acmecorp.com", "pwong@acmecorp.com"],
        "cc": ["sarah.chen@nexustech.io"],
        "subject": "Acme Corp - Enterprise Proposal",
        "body": """John, Patricia,

Thanks for the great conversation yesterday. Attached is our enterprise proposal for a 500-seat deployment.

Summary:
- Annual contract: $180,000/year ($30/seat/month)
- Includes: SSO integration, dedicated support, 99.9% SLA
- Implementation: 4-6 weeks with dedicated onboarding team
- Training: 2 days on-site training included

We're also offering a 10% discount for a 2-year commitment, bringing the annual cost to $162,000.

POC timeline: We can start a 30-day proof of concept next week with your engineering handbook.

Let me know if you have questions. Happy to jump on a call.

Best,
Emily""",
        "date": format_date(BASE_DATE + timedelta(days=15)),
        "thread_id": "thread-acme-pricing",
    })

    emails.append({
        "id": "email-acme-004",
        "type": "Email",
        "from": "pwong@acmecorp.com",
        "to": ["emily.rodriguez@nexustech.io"],
        "cc": ["jharrison@acmecorp.com"],
        "subject": "Re: Acme Corp - Enterprise Proposal",
        "body": """Emily,

Thanks for the proposal. I've reviewed it with our procurement team.

A few points:
1. The $30/seat is above our budget. We were thinking closer to $20/seat given the volume.
2. We'd need the SOC2 certification before we can sign - is there flexibility on timing?
3. Can you include unlimited Slack integration in the base price?

Let me know if there's room to discuss.

Patricia Wong
Procurement Lead, Acme Corporation""",
        "date": format_date(BASE_DATE + timedelta(days=18)),
        "thread_id": "thread-acme-pricing",
    })

    # Internal slack about negotiation
    slacks.extend([
        {
            "id": "slack-acme-004",
            "type": "Slack",
            "channel": "#leadership",
            "author": "emily.r",
            "content": "Acme is pushing back on pricing. They want $20/seat vs our $30. That's $60k less annually. @sarah.chen @james.m thoughts?",
            "date": format_date(BASE_DATE + timedelta(days=18, hours=3)),
            "thread_id": "slack-thread-acme-pricing",
        },
        {
            "id": "slack-acme-005",
            "type": "Slack",
            "channel": "#leadership",
            "author": "james.m",
            "content": "Our margins are tight at $20. Best case I could see is $25/seat with a 2-year commitment. That still gives us healthy unit economics",
            "date": format_date(BASE_DATE + timedelta(days=18, hours=3, minutes=20)),
            "thread_id": "slack-thread-acme-pricing",
        },
        {
            "id": "slack-acme-006",
            "type": "Slack",
            "channel": "#leadership",
            "author": "sarah.chen",
            "content": "This is a strategic account. Landing a Fortune 500 logo matters for the Series A. Let's do $24/seat for 3 years. That's $144k ARR and a reference customer. Worth it.",
            "date": format_date(BASE_DATE + timedelta(days=18, hours=4)),
            "thread_id": "slack-thread-acme-pricing",
        },
    ])

    # Final agreement email
    emails.append({
        "id": "email-acme-005",
        "type": "Email",
        "from": "emily.rodriguez@nexustech.io",
        "to": ["pwong@acmecorp.com", "jharrison@acmecorp.com"],
        "cc": ["sarah.chen@nexustech.io", "awhite@legalfirm.com"],
        "subject": "Acme Corp - Revised Proposal (Final)",
        "body": """Patricia, John,

We've reviewed your feedback with our leadership team. Here's our best offer:

- 3-year commitment: $24/seat/month ($144,000/year)
- Includes: Full enterprise features, SSO, Slack integration
- SOC2 commitment: We'll have certification by Feb 2025 (before your deployment)
- Added: Quarterly business reviews and dedicated customer success manager

This represents significant value vs. our standard pricing. We believe this partnership can be transformational for both companies.

If agreeable, I'll have contracts ready by end of week.

Best,
Emily""",
        "date": format_date(BASE_DATE + timedelta(days=22)),
        "thread_id": "thread-acme-pricing",
    })

    emails.append({
        "id": "email-acme-006",
        "type": "Email",
        "from": "jharrison@acmecorp.com",
        "to": ["emily.rodriguez@nexustech.io"],
        "cc": ["pwong@acmecorp.com", "sarah.chen@nexustech.io"],
        "subject": "Re: Acme Corp - Revised Proposal (Final)",
        "body": """Emily,

We're in. The $24/seat works for us and the SOC2 timeline addresses our compliance needs.

Let's proceed with contracts. Patricia will handle the procurement side. I'm excited to get this deployed - our teams have been asking for a better solution for months.

John""",
        "date": format_date(BASE_DATE + timedelta(days=24)),
        "thread_id": "thread-acme-pricing",
    })

    # Celebration slack
    slacks.append({
        "id": "slack-acme-007",
        "type": "Slack",
        "channel": "#general",
        "author": "emily.r",
        "content": ":tada: HUGE NEWS! We just closed Acme Corporation - our first Fortune 500 customer! 500 seats, 3-year deal, $432k TCV. This is a massive milestone for Nexus! Thanks @sarah.chen @marcus.w @james.m for helping make this happen!",
        "date": format_date(BASE_DATE + timedelta(days=24, hours=5)),
        "thread_id": "slack-thread-acme-win",
    })

    return emails, slacks, docs, events


# ============================================================================
# STORY 2: Project Phoenix (Platform Rewrite)
# ============================================================================

def generate_phoenix_story():
    """Generate the Project Phoenix technical story."""
    emails = []
    slacks = []
    docs = []
    events = []

    # Architecture proposal doc
    docs.append({
        "id": "doc-phoenix-proposal",
        "type": "Document",
        "title": "Project Phoenix - Architecture Proposal",
        "author": "Marcus Williams",
        "content": """# Project Phoenix: Next-Gen Platform Architecture

## Executive Summary
Our current monolithic architecture is hitting scaling limits. Project Phoenix proposes a complete platform rewrite using modern microservices architecture with a knowledge graph at its core.

## Current Problems
1. Monolith deployments take 45+ minutes
2. Single database bottleneck - can't scale horizontally
3. No real-time capabilities
4. Search is basic keyword matching

## Proposed Architecture

### Core Components
- **Knowledge Graph Engine**: Neo4j-based graph database for entity/relationship storage
- **Event Bus**: Kafka for real-time data streaming
- **API Gateway**: Kong for rate limiting and authentication
- **Search Service**: Elasticsearch with vector embeddings

### Key Innovations
- **Graph-RAG Hybrid**: Combine knowledge graph traversal with RAG for superior retrieval
- **Attention Mechanism**: Track what users focus on to improve relevance
- **Real-time Sync**: Live updates across all connected clients

## Timeline
- Phase 1 (Q4 2024): Core graph engine and API rewrite
- Phase 2 (Q1 2025): Migration tooling and enterprise features
- Phase 3 (Q2 2025): Full customer migration

## Resource Requirements
- 2 additional senior engineers
- $50k cloud infrastructure budget increase
- 3rd party security audit ($30k)

## Risks
- Customer migration complexity
- Team learning curve on new stack
- Parallel system maintenance during transition

## Recommendation
Approve Phase 1 immediately. This architecture will support 10x scale and enable features competitors can't match.""",
        "date": format_date(BASE_DATE + timedelta(days=5)),
        "folder": "Engineering/Project Phoenix",
    })

    # Leadership discussion emails
    emails.append({
        "id": "email-phoenix-001",
        "type": "Email",
        "from": "marcus.williams@nexustech.io",
        "to": ["sarah.chen@nexustech.io", "james.mitchell@nexustech.io"],
        "cc": ["rachel.foster@nexustech.io"],
        "subject": "Project Phoenix - Requesting Approval",
        "body": """Sarah, James,

Attached is the Project Phoenix architecture proposal. I'm requesting approval to proceed with Phase 1.

Key points:
- Current system can't scale past 50 enterprise customers
- Acme deal will push us past that limit
- Phoenix gives us 10x headroom

The $80k investment (infra + audit) pays for itself if we close 2 more enterprise deals.

Can we discuss at tomorrow's leadership sync?

Marcus""",
        "date": format_date(BASE_DATE + timedelta(days=6)),
        "thread_id": "thread-phoenix-approval",
    })

    emails.append({
        "id": "email-phoenix-002",
        "type": "Email",
        "from": "james.mitchell@nexustech.io",
        "to": ["marcus.williams@nexustech.io", "sarah.chen@nexustech.io"],
        "cc": ["rachel.foster@nexustech.io"],
        "subject": "Re: Project Phoenix - Requesting Approval",
        "body": """Marcus,

I've reviewed the numbers. A few concerns:

1. The $50k infra increase is ongoing, not one-time. That's $600k over a year.
2. Two senior engineers at market rate is ~$500k/year fully loaded.
3. Total burn rate increase: ~$1.2M/year

That said, I understand the strategic necessity. If Emily closes Acme and the Global Retail lead, the ROI is there.

My recommendation: Approve Phase 1, but tie Phase 2 to hitting Q4 revenue targets.

James""",
        "date": format_date(BASE_DATE + timedelta(days=6, hours=4)),
        "thread_id": "thread-phoenix-approval",
    })

    # Slack engineering discussions
    slacks.extend([
        {
            "id": "slack-phoenix-001",
            "type": "Slack",
            "channel": "#engineering",
            "author": "marcus.w",
            "content": "Team, Project Phoenix got approved! :rocket: We're moving forward with the platform rewrite. @david.kim @alex.t @robert.g let's sync tomorrow on sprint planning",
            "date": format_date(BASE_DATE + timedelta(days=8)),
            "thread_id": "slack-thread-phoenix-kickoff",
        },
        {
            "id": "slack-phoenix-002",
            "type": "Slack",
            "channel": "#engineering",
            "author": "david.kim",
            "content": "Exciting! I've been wanting to move to a proper graph architecture for months. What's our timeline for the first milestone?",
            "date": format_date(BASE_DATE + timedelta(days=8, minutes=15)),
            "thread_id": "slack-thread-phoenix-kickoff",
        },
        {
            "id": "slack-phoenix-003",
            "type": "Slack",
            "channel": "#engineering",
            "author": "marcus.w",
            "content": "End of November for core graph engine. We need to have the new retrieval system ready before Acme onboards in January",
            "date": format_date(BASE_DATE + timedelta(days=8, minutes=30)),
            "thread_id": "slack-thread-phoenix-kickoff",
        },
        {
            "id": "slack-phoenix-004",
            "type": "Slack",
            "channel": "#engineering",
            "author": "alex.t",
            "content": "Aggressive but doable. I'll start on the graph schema design. Should we use Neo4j or something else?",
            "date": format_date(BASE_DATE + timedelta(days=8, minutes=45)),
            "thread_id": "slack-thread-phoenix-kickoff",
        },
        {
            "id": "slack-phoenix-005",
            "type": "Slack",
            "channel": "#engineering",
            "author": "marcus.w",
            "content": "Neo4j Community Edition for now. We can evaluate Enterprise later if we need clustering. The licensing is cleaner.",
            "date": format_date(BASE_DATE + timedelta(days=8, hours=1)),
            "thread_id": "slack-thread-phoenix-kickoff",
        },
    ])

    # Sprint planning doc
    docs.append({
        "id": "doc-phoenix-sprint1",
        "type": "Document",
        "title": "Project Phoenix - Sprint 1 Planning",
        "author": "David Kim",
        "content": """# Phoenix Sprint 1 Planning
**Sprint Duration:** Oct 14 - Oct 28, 2024
**Sprint Goal:** Core graph engine foundation

## Team
- Marcus Williams (Tech Lead)
- David Kim (Graph Engine)
- Alex Thompson (API Layer)
- Robert Garcia (Infrastructure)

## Stories

### Graph Engine Core (David)
- [x] Set up Neo4j instance with Docker
- [x] Design node/edge schema for entities
- [x] Implement CRUD operations for nodes
- [ ] Add relationship types and properties
- [ ] Write migration script from old schema

### API Layer (Alex)
- [x] New API gateway setup (Kong)
- [ ] Node creation endpoint
- [ ] Link creation endpoint
- [ ] Query endpoint with graph traversal
- [ ] Authentication middleware

### Infrastructure (Robert)
- [x] Kubernetes cluster for Phoenix services
- [x] CI/CD pipeline for new services
- [ ] Monitoring and alerting setup
- [ ] Load testing framework

## Blockers
- Need access credentials for production AWS account
- Waiting on security review of Neo4j config

## Notes
- Daily standups at 10am PT
- Code reviews required before merge
- All new code needs 80%+ test coverage""",
        "date": format_date(BASE_DATE + timedelta(days=14)),
        "folder": "Engineering/Project Phoenix",
    })

    # Calendar events
    events.append({
        "id": "cal-phoenix-kickoff",
        "type": "Calendar",
        "title": "Project Phoenix Kickoff",
        "organizer": "marcus.williams@nexustech.io",
        "attendees": ["david.kim@nexustech.io", "alex.thompson@nexustech.io", "robert.garcia@nexustech.io", "rachel.foster@nexustech.io"],
        "date": format_date(BASE_DATE + timedelta(days=9)),
        "duration_minutes": 90,
        "location": "Conference Room A",
        "description": "Kickoff meeting for Project Phoenix platform rewrite. Agenda: architecture overview, team assignments, timeline discussion.",
    })

    return emails, slacks, docs, events


# ============================================================================
# STORY 3: Series A Fundraise
# ============================================================================

def generate_fundraise_story():
    """Generate the Series A fundraise story."""
    emails = []
    slacks = []
    docs = []
    events = []

    # VC outreach email
    emails.append({
        "id": "email-fund-001",
        "type": "Email",
        "from": "jadams@techventures.vc",
        "to": ["sarah.chen@nexustech.io"],
        "subject": "Nexus Technologies - Series A Interest",
        "body": """Sarah,

I've been following Nexus Technologies since your seed round. Your approach to enterprise knowledge management using knowledge graphs is differentiated.

I hear you recently closed Acme Corp as a customer - congratulations! That's exactly the kind of enterprise traction we look for.

Tech Ventures is actively investing in enterprise AI infrastructure. We'd love to learn more about your Series A plans.

Are you available for a call next week?

Best,
Jennifer Adams
Partner, Tech Ventures""",
        "date": format_date(BASE_DATE + timedelta(days=30)),
        "thread_id": "thread-series-a",
    })

    emails.append({
        "id": "email-fund-002",
        "type": "Email",
        "from": "sarah.chen@nexustech.io",
        "to": ["jadams@techventures.vc"],
        "subject": "Re: Nexus Technologies - Series A Interest",
        "body": """Jennifer,

Great to hear from you! Yes, the Acme deal was a big milestone - our first Fortune 500 logo.

We're planning to raise $15M Series A in Q1 2025. The round will fund:
- Engineering team expansion (5 new hires)
- Enterprise go-to-market push
- Platform infrastructure scaling

I'd be happy to share our deck and discuss. How about Tuesday at 2pm PT?

Best,
Sarah Chen
CEO, Nexus Technologies""",
        "date": format_date(BASE_DATE + timedelta(days=31)),
        "thread_id": "thread-series-a",
    })

    # Investor deck doc
    docs.append({
        "id": "doc-series-a-deck",
        "type": "Document",
        "title": "Nexus Technologies - Series A Deck (Confidential)",
        "author": "Sarah Chen",
        "content": """# Nexus Technologies
## Institutional Memory for the Enterprise

### The Problem
- Enterprise knowledge is scattered across 20+ tools
- 30% of employee time spent searching for information
- When employees leave, institutional knowledge leaves with them
- $47B market for enterprise knowledge management

### Our Solution
**Nexus creates a living knowledge graph from all your company's data**
- Automatically connects information across email, Slack, docs, meetings
- AI-powered retrieval that understands context and relationships
- Surfaces insights and connections humans would miss

### Why Now
- LLM capabilities make true understanding possible
- Remote work increased knowledge fragmentation
- Enterprises finally ready to invest in AI infrastructure

### Traction
- $500K ARR (4x YoY growth)
- 12 paying customers, 3 enterprise
- Acme Corp: $432K TCV (Fortune 500)
- 140% net revenue retention
- Pipeline: $2M+ in qualified opportunities

### Business Model
- Seat-based SaaS pricing
- Enterprise: $24-40/seat/month
- SMB: $15/seat/month
- Professional services for custom integrations

### Team
- Sarah Chen (CEO) - ex-Google PM, Stanford CS
- Marcus Williams (CTO) - ex-Netflix infrastructure
- Emily Rodriguez (VP Sales) - ex-Salesforce enterprise
- 12 FTEs total

### The Ask
- Raising $15M Series A at $60M pre-money
- Use of funds:
  - 60% - Engineering (5 new hires)
  - 25% - Sales & Marketing
  - 15% - Infrastructure & Operations

### Why Tech Ventures
- Deep enterprise SaaS expertise
- Portfolio synergies with [Company X] and [Company Y]
- Value-add beyond capital""",
        "date": format_date(BASE_DATE + timedelta(days=32)),
        "folder": "Fundraising/Series A",
    })

    # Internal slack about fundraise
    slacks.extend([
        {
            "id": "slack-fund-001",
            "type": "Slack",
            "channel": "#leadership",
            "author": "sarah.chen",
            "content": "Just got inbound from Tech Ventures - they want to discuss Series A! Jennifer Adams is one of the best enterprise SaaS investors. Meeting next Tuesday.",
            "date": format_date(BASE_DATE + timedelta(days=30, hours=2)),
            "thread_id": "slack-thread-fundraise",
        },
        {
            "id": "slack-fund-002",
            "type": "Slack",
            "channel": "#leadership",
            "author": "james.m",
            "content": "That's great news! Their portfolio has some huge exits. What's our target valuation?",
            "date": format_date(BASE_DATE + timedelta(days=30, hours=2, minutes=30)),
            "thread_id": "slack-thread-fundraise",
        },
        {
            "id": "slack-fund-003",
            "type": "Slack",
            "channel": "#leadership",
            "author": "sarah.chen",
            "content": "I'm thinking $60M pre. We're at $500K ARR growing 4x, with the Acme deal showing we can land enterprise. Comparable Series A rounds are getting 15-20x ARR multiples.",
            "date": format_date(BASE_DATE + timedelta(days=30, hours=3)),
            "thread_id": "slack-thread-fundraise",
        },
    ])

    # Calendar for VC meeting
    events.append({
        "id": "cal-vc-meeting",
        "type": "Calendar",
        "title": "Tech Ventures - Series A Discussion",
        "organizer": "sarah.chen@nexustech.io",
        "attendees": ["jadams@techventures.vc"],
        "date": format_date(BASE_DATE + timedelta(days=35)),
        "duration_minutes": 45,
        "location": "Zoom",
        "description": "Initial Series A discussion with Tech Ventures. Sharing deck and key metrics.",
    })

    return emails, slacks, docs, events


# ============================================================================
# STORY 4: Q4 Budget Planning
# ============================================================================

def generate_budget_story():
    """Generate Q4 budget planning story."""
    emails = []
    slacks = []
    docs = []
    events = []

    # Budget doc
    docs.append({
        "id": "doc-q4-budget",
        "type": "Document",
        "title": "Q4 2024 Budget and Hiring Plan",
        "author": "James Mitchell",
        "content": """# Q4 2024 Budget and Hiring Plan

## Revenue Forecast
| Month | MRR Forecast | Notes |
|-------|-------------|-------|
| October | $42K | Current run rate |
| November | $48K | SMB pipeline close |
| December | $60K | Acme first payment |

**Q4 Total Revenue:** $150K
**Exit ARR:** $720K

## Operating Expenses

### Headcount (Current: 12)
| Department | Current | Q4 Hires | End Q4 |
|------------|---------|----------|--------|
| Engineering | 5 | 1 | 6 |
| Sales | 2 | 1 | 3 |
| Product | 2 | 0 | 2 |
| Finance | 1 | 0 | 1 |
| Marketing | 1 | 0 | 1 |
| CS | 1 | 0 | 1 |

**Total Payroll (monthly):** $280K → $320K

### Infrastructure
- AWS: $25K/month → $35K/month (Phoenix scaling)
- Neo4j: $0 (Community Edition)
- Tools (Slack, GitHub, etc.): $5K/month

### Other
- Legal (contracts, SOC2): $40K one-time
- Office: $8K/month
- Travel: $15K

## Q4 Budget Summary
| Category | Q4 Total |
|----------|----------|
| Payroll | $920K |
| Infrastructure | $105K |
| Legal | $40K |
| Office | $24K |
| Travel | $15K |
| Other | $20K |
| **Total** | **$1.124M** |

## Cash Position
- Current: $1.8M
- Q4 Revenue: $150K
- Q4 Expenses: $1.124M
- **End Q4 Cash:** ~$826K (6.8 months runway)

## Decisions Needed
1. Approve senior engineer hire ($180K/year)
2. Approve SDR hire ($75K/year)
3. Confirm Phoenix infrastructure budget increase
4. SOC2 audit vendor selection

## Risk Factors
- If Acme delays past January, December revenue impacted
- Phoenix infrastructure costs could exceed estimate by 20%
- Runway depends on Series A timing""",
        "date": format_date(BASE_DATE + timedelta(days=3)),
        "folder": "Finance/Budget",
    })

    # Budget review email
    emails.append({
        "id": "email-budget-001",
        "type": "Email",
        "from": "james.mitchell@nexustech.io",
        "to": ["sarah.chen@nexustech.io"],
        "cc": ["marcus.williams@nexustech.io", "emily.rodriguez@nexustech.io"],
        "subject": "Q4 Budget Draft - Review Needed",
        "body": """Sarah,

Attached is the Q4 budget draft. Key points:

1. We'll end Q4 with ~$826K cash (6.8 months runway)
2. This assumes Acme closes as planned and pays in December
3. Phoenix infra costs add $30K/quarter to our burn

The 2 hires (senior eng + SDR) are essential:
- Senior eng for Phoenix timeline
- SDR to support Emily's enterprise push

Without Series A, we have until ~June before we need to cut costs.

Let me know your thoughts before Thursday's board meeting.

James""",
        "date": format_date(BASE_DATE + timedelta(days=3)),
        "thread_id": "thread-budget-q4",
    })

    # Board meeting doc
    docs.append({
        "id": "doc-board-oct",
        "type": "Document",
        "title": "Board Meeting Notes - October 2024",
        "author": "Sarah Chen",
        "content": """# Board Meeting Notes
**Date:** October 8, 2024
**Attendees:** Sarah Chen, James Mitchell, Board Members (via Zoom)

## Agenda

### 1. Q3 Review
- Exceeded revenue target: $135K vs $120K plan
- Closed 4 new customers including 2 enterprise prospects
- Launched v2.0 with improved search

### 2. Acme Corp Deal
- Signed: 3-year, $432K TCV, 500 seats
- First Fortune 500 logo
- Deployment starting January 2025

### 3. Project Phoenix
- Board approved Phase 1 ($80K budget)
- Critical for scaling past current architecture limits
- Team confident in November delivery

### 4. Q4 Budget
- Approved senior engineer hire
- Approved SDR hire
- Total Q4 burn: $1.124M

### 5. Series A Preparation
- Target: $15M at $60M pre-money
- Timing: Q1 2025
- Jennifer Adams (Tech Ventures) expressed strong interest

## Action Items
- [ ] Sarah: Finalize Series A deck by Oct 20
- [ ] James: Monthly board financial updates
- [ ] Marcus: Phoenix Phase 1 demo by Nov 30
- [ ] Emily: Global Retail update in 2 weeks

## Next Meeting
November 12, 2024""",
        "date": format_date(BASE_DATE + timedelta(days=8)),
        "folder": "Board/Meetings",
    })

    # Calendar for board meeting
    events.append({
        "id": "cal-board-oct",
        "type": "Calendar",
        "title": "October Board Meeting",
        "organizer": "sarah.chen@nexustech.io",
        "attendees": ["james.mitchell@nexustech.io", "marcus.williams@nexustech.io", "emily.rodriguez@nexustech.io"],
        "date": format_date(BASE_DATE + timedelta(days=8)),
        "duration_minutes": 120,
        "location": "Zoom",
        "description": "Monthly board meeting. Agenda: Q3 review, Acme update, Phoenix approval, Q4 budget.",
    })

    return emails, slacks, docs, events


# ============================================================================
# STORY 5: SOC2 Compliance
# ============================================================================

def generate_soc2_story():
    """Generate SOC2 compliance story."""
    emails = []
    slacks = []
    docs = []
    events = []

    # SOC2 plan doc
    docs.append({
        "id": "doc-soc2-plan",
        "type": "Document",
        "title": "SOC2 Type II Certification Plan",
        "author": "Robert Garcia",
        "content": """# SOC2 Type II Certification Plan

## Overview
SOC2 Type II certification is required for enterprise sales, particularly Acme Corp's compliance requirements. Target completion: February 2025.

## Scope
- Trust Service Criteria: Security, Availability, Confidentiality
- Systems in scope: Production infrastructure, customer data storage, employee systems

## Timeline
| Phase | Duration | Target Date |
|-------|----------|-------------|
| Gap Assessment | 2 weeks | Oct 31 |
| Policy Development | 3 weeks | Nov 21 |
| Control Implementation | 4 weeks | Dec 19 |
| Audit Prep | 2 weeks | Jan 2 |
| Type II Observation | 6 weeks | Feb 13 |
| Report Issuance | 2 weeks | Feb 27 |

## Key Controls Needed
1. **Access Management**
   - [ ] SSO for all employee tools
   - [ ] Quarterly access reviews
   - [ ] Privileged access management

2. **Change Management**
   - [ ] Code review requirements
   - [ ] Deployment approval process
   - [ ] Rollback procedures

3. **Incident Response**
   - [ ] IR plan documentation
   - [ ] On-call rotation
   - [ ] Customer notification process

4. **Vendor Management**
   - [ ] Vendor security assessments
   - [ ] Contract review process

5. **Data Protection**
   - [ ] Encryption at rest and in transit
   - [ ] Data retention policy
   - [ ] Backup testing

## Vendor Selection
Evaluating 3 audit firms:
1. ComplianceCo - $35K, 8-week timeline
2. SecureAudit - $42K, 6-week timeline
3. TrustPath - $38K, 7-week timeline

**Recommendation:** ComplianceCo (best value, acceptable timeline)

## Resource Requirements
- Robert Garcia: 50% allocation
- DevOps support: 20% allocation
- Legal review: 10 hours
- External consultant: $10K

## Risks
- Observation period must start by Dec 19 for Feb completion
- Any P1 incidents during observation could delay certification
- Employee training must be complete before observation""",
        "date": format_date(BASE_DATE + timedelta(days=12)),
        "folder": "Security/SOC2",
    })

    # SOC2 kickoff slack
    slacks.extend([
        {
            "id": "slack-soc2-001",
            "type": "Slack",
            "channel": "#engineering",
            "author": "robert.g",
            "content": "Starting SOC2 prep this week. Going to need help from everyone on policy documentation. I'll be reaching out individually about access reviews and change management processes.",
            "date": format_date(BASE_DATE + timedelta(days=12)),
            "thread_id": "slack-thread-soc2",
        },
        {
            "id": "slack-soc2-002",
            "type": "Slack",
            "channel": "#engineering",
            "author": "alex.t",
            "content": "What do you need from the dev team? I know our code review process is pretty informal right now",
            "date": format_date(BASE_DATE + timedelta(days=12, minutes=30)),
            "thread_id": "slack-thread-soc2",
        },
        {
            "id": "slack-soc2-003",
            "type": "Slack",
            "channel": "#engineering",
            "author": "robert.g",
            "content": "Main things: 1) All PRs need at least one approval before merge 2) No direct commits to main 3) Deployment approvals logged. I'll send a checklist later today.",
            "date": format_date(BASE_DATE + timedelta(days=12, hours=1)),
            "thread_id": "slack-thread-soc2",
        },
    ])

    # Vendor email
    emails.append({
        "id": "email-soc2-001",
        "type": "Email",
        "from": "robert.garcia@nexustech.io",
        "to": ["sarah.chen@nexustech.io", "james.mitchell@nexustech.io"],
        "subject": "SOC2 Auditor Recommendation",
        "body": """Sarah, James,

I've evaluated 3 SOC2 audit vendors. My recommendation is ComplianceCo:
- Cost: $35K (vs $42K and $38K alternatives)
- Timeline: 8 weeks (acceptable for our Feb target)
- References: Positive feedback from 3 startups in their portfolio

They can start the gap assessment next week if we sign by Friday.

One risk: Their timeline is slightly longer, so we have less buffer. But the cost savings ($7K+) and their startup experience make them the best fit.

Should I proceed with contracting?

Robert""",
        "date": format_date(BASE_DATE + timedelta(days=15)),
        "thread_id": "thread-soc2-vendor",
    })

    return emails, slacks, docs, events


# ============================================================================
# STORY 6: Customer Success / Support
# ============================================================================

def generate_customer_success_story():
    """Generate customer success and support stories."""
    emails = []
    slacks = []
    docs = []
    events = []

    # Customer issue email
    emails.append({
        "id": "email-support-001",
        "type": "Email",
        "from": "mstone@globalretail.com",
        "to": ["lisa.park@nexustech.io"],
        "subject": "Search performance issues",
        "body": """Lisa,

We've noticed search has been slow the past few days - queries that used to return in 1-2 seconds are now taking 5-10 seconds.

This is impacting our team's productivity. Can you look into it?

Michael Stone
CIO, Global Retail Inc""",
        "date": format_date(BASE_DATE + timedelta(days=20)),
        "thread_id": "thread-support-globalretail",
    })

    emails.append({
        "id": "email-support-002",
        "type": "Email",
        "from": "lisa.park@nexustech.io",
        "to": ["mstone@globalretail.com"],
        "cc": ["marcus.williams@nexustech.io"],
        "subject": "Re: Search performance issues",
        "body": """Michael,

Thank you for reporting this. I've escalated to our engineering team and they're investigating now.

Initial analysis shows increased load on our search cluster due to recent growth. We're adding capacity today and expect performance to return to normal within 24 hours.

I'll update you as soon as the fix is deployed.

Apologies for the inconvenience.

Lisa Park
Customer Success, Nexus Technologies""",
        "date": format_date(BASE_DATE + timedelta(days=20, hours=2)),
        "thread_id": "thread-support-globalretail",
    })

    # Internal escalation slack
    slacks.extend([
        {
            "id": "slack-support-001",
            "type": "Slack",
            "channel": "#engineering",
            "author": "lisa.park",
            "content": "Hey team, Global Retail is reporting slow search - 5-10 second query times. They're one of our largest customers. Can someone look into this urgently? @marcus.w @david.kim",
            "date": format_date(BASE_DATE + timedelta(days=20, hours=1)),
            "thread_id": "slack-thread-support-search",
        },
        {
            "id": "slack-support-002",
            "type": "Slack",
            "channel": "#engineering",
            "author": "david.kim",
            "content": "Looking now. Seeing high CPU on search-03 node. Looks like the index got corrupted during last night's update. Rebuilding now - should be fixed in ~2 hours.",
            "date": format_date(BASE_DATE + timedelta(days=20, hours=1, minutes=30)),
            "thread_id": "slack-thread-support-search",
        },
        {
            "id": "slack-support-003",
            "type": "Slack",
            "channel": "#engineering",
            "author": "marcus.w",
            "content": "Good catch. Let's add monitoring for index corruption to prevent this. @robert.g can you add an alert?",
            "date": format_date(BASE_DATE + timedelta(days=20, hours=2)),
            "thread_id": "slack-thread-support-search",
        },
    ])

    # QBR doc
    docs.append({
        "id": "doc-qbr-globalretail",
        "type": "Document",
        "title": "Global Retail Inc - Q3 Quarterly Business Review",
        "author": "Lisa Park",
        "content": """# Global Retail Inc - Q3 2024 QBR

## Account Overview
- **Customer since:** March 2024
- **Plan:** Enterprise (200 seats)
- **Monthly spend:** $6,000
- **Contract renewal:** March 2025

## Usage Metrics
| Metric | Q2 | Q3 | Change |
|--------|----|----|--------|
| Active users | 142 | 178 | +25% |
| Queries/day | 2,400 | 4,100 | +71% |
| Documents indexed | 45K | 82K | +82% |
| Avg session time | 12 min | 18 min | +50% |

## Key Wins
- Onboarded entire product team (40 users) in August
- Integrated with their Jira instance
- Search NPS: 72 (up from 61)

## Challenges
- Search performance incident on Oct 20 (resolved in 4 hours)
- Request for Salesforce integration (on roadmap for Q1)
- Some users still defaulting to old wiki

## Expansion Opportunities
- Engineering team interested (150 additional seats)
- Data science team exploring for ML documentation
- Potential $4,500/month upsell

## Action Items
- [ ] Lisa: Send Salesforce integration timeline
- [ ] Marcus: Technical call with their engineering lead
- [ ] Emily: Expansion proposal by Nov 15

## Renewal Risk Assessment
**Risk Level:** Low
- High engagement metrics
- Executive sponsor (Michael Stone) is champion
- Budget already approved for renewal""",
        "date": format_date(BASE_DATE + timedelta(days=25)),
        "folder": "Customer Success/Global Retail",
    })

    # QBR calendar event
    events.append({
        "id": "cal-qbr-globalretail",
        "type": "Calendar",
        "title": "Global Retail QBR",
        "organizer": "lisa.park@nexustech.io",
        "attendees": ["mstone@globalretail.com", "emily.rodriguez@nexustech.io"],
        "date": format_date(BASE_DATE + timedelta(days=25)),
        "duration_minutes": 60,
        "location": "Zoom",
        "description": "Quarterly business review with Global Retail. Review usage, discuss expansion, and roadmap alignment.",
    })

    return emails, slacks, docs, events


# ============================================================================
# ADDITIONAL RANDOM DATA
# ============================================================================

def generate_random_slack_chatter():
    """Generate random slack messages for realism."""
    messages = []

    random_messages = [
        ("general", "michelle.l", "New blog post is live! Check it out: 'How Knowledge Graphs Transform Enterprise Search'"),
        ("general", "sarah.chen", "Team lunch tomorrow at 12:30 - Thai Palace. My treat for closing Acme! 🎉"),
        ("random", "alex.t", "Anyone have a good standing desk recommendation? My back is killing me"),
        ("random", "rachel.f", "The coffee machine on 3rd floor is broken again..."),
        ("engineering", "robert.g", "Deploying database migration tonight at 10pm PT. Should be quick but heads up"),
        ("engineering", "david.kim", "New PR for the search optimization is up. Need reviewers - pretty big change"),
        ("product", "rachel.f", "User research sessions scheduled for next week. Looking for 3 volunteers to observe"),
        ("sales", "emily.r", "Pipeline review moved to 3pm today - conference room B"),
        ("general", "james.m", "Reminder: expense reports due by Friday"),
        ("engineering", "marcus.w", "Great work on the Phoenix sprint everyone. We're ahead of schedule!"),
        ("random", "lisa.park", "Found a great burrito place near the office. Happy to share coords"),
        ("general", "sarah.chen", "All hands meeting next Tuesday - have some exciting updates to share"),
        ("product", "rachel.f", "Customer feedback summary from last month is in the shared drive. Lots of good insights"),
        ("engineering", "alex.t", "Anyone else getting timeouts on the staging environment?"),
        ("sales", "emily.r", "Just got off the call with Global Retail - they're interested in expanding!"),
    ]

    for i, (channel, author, content) in enumerate(random_messages):
        messages.append({
            "id": f"slack-random-{i+1:03d}",
            "type": "Slack",
            "channel": f"#{channel}",
            "author": author,
            "content": content,
            "date": format_date(random_date()),
            "thread_id": f"slack-thread-random-{i+1}",
        })

    return messages


def generate_random_emails():
    """Generate random operational emails."""
    emails = []

    # Vendor emails
    emails.append({
        "id": "email-vendor-001",
        "type": "Email",
        "from": "tbrown@cloudservices.net",
        "to": ["robert.garcia@nexustech.io"],
        "subject": "AWS Reserved Instance Recommendations",
        "body": """Robert,

Based on your current usage patterns, I've identified some opportunities to optimize your AWS spend:

1. Convert 3x m5.xlarge to reserved instances - saves $400/month
2. Right-size your RDS instance - currently underutilized
3. Enable S3 Intelligent Tiering for your backup bucket

Happy to walk through the details on a call.

Thomas Brown
Account Manager, CloudServices""",
        "date": format_date(BASE_DATE + timedelta(days=16)),
        "thread_id": "thread-vendor-aws",
    })

    # Legal email
    emails.append({
        "id": "email-legal-001",
        "type": "Email",
        "from": "awhite@legalfirm.com",
        "to": ["sarah.chen@nexustech.io", "james.mitchell@nexustech.io"],
        "subject": "Acme Corp MSA - Final Review",
        "body": """Sarah, James,

Attached is the final Master Services Agreement with Acme Corporation. I've incorporated their requested changes:

1. Added mutual indemnification clause (Section 8.2)
2. Extended termination notice period to 90 days
3. Clarified data handling upon termination

Their legal team has signed off. Ready for your signature.

One note: The SOC2 certification timeline is referenced in the contract. Make sure you're on track for February.

Amanda White
Corporate Counsel, Legal Partners LLP""",
        "date": format_date(BASE_DATE + timedelta(days=23)),
        "thread_id": "thread-legal-acme",
    })

    # HR/recruiting email
    emails.append({
        "id": "email-hr-001",
        "type": "Email",
        "from": "marcus.williams@nexustech.io",
        "to": ["sarah.chen@nexustech.io"],
        "subject": "Senior Engineer Candidates - Final Round",
        "body": """Sarah,

We have 2 strong candidates for the senior engineer role. Both completed technical interviews:

1. **Jennifer Martinez** - 8 years exp, ex-Stripe
   - Pros: Strong distributed systems background, great culture fit
   - Cons: Asking for $195K (above budget)

2. **Kevin Liu** - 6 years exp, ex-Datadog
   - Pros: Graph database experience, within budget at $175K
   - Cons: Less leadership experience

My recommendation: Jennifer. The extra $15K is worth it for her experience. She'd accelerate Phoenix significantly.

Want to do final round interviews this week?

Marcus""",
        "date": format_date(BASE_DATE + timedelta(days=28)),
        "thread_id": "thread-hiring-senior-eng",
    })

    return emails


# ============================================================================
# SIMPLE ERP-LIKE DATA
# ============================================================================

def generate_erp_data():
    """Generate simple ERP-like data (invoices, purchase orders)."""
    invoices = []
    purchase_orders = []

    # Invoices to customers
    invoices.extend([
        {
            "id": "inv-2024-001",
            "type": "Invoice",
            "customer": "Global Retail Inc",
            "contact": "mstone@globalretail.com",
            "amount": 6000.00,
            "currency": "USD",
            "description": "Nexus Enterprise - 200 seats - October 2024",
            "status": "paid",
            "issued_date": format_date(BASE_DATE + timedelta(days=1)),
            "due_date": format_date(BASE_DATE + timedelta(days=31)),
            "paid_date": format_date(BASE_DATE + timedelta(days=25)),
        },
        {
            "id": "inv-2024-002",
            "type": "Invoice",
            "customer": "Acme Corporation",
            "contact": "pwong@acmecorp.com",
            "amount": 12000.00,
            "currency": "USD",
            "description": "Nexus Enterprise - 500 seats - December 2024 (first month)",
            "status": "pending",
            "issued_date": format_date(BASE_DATE + timedelta(days=60)),
            "due_date": format_date(BASE_DATE + timedelta(days=90)),
            "paid_date": None,
        },
        {
            "id": "inv-2024-003",
            "type": "Invoice",
            "customer": "TechStart Inc",
            "contact": "billing@techstart.io",
            "amount": 1500.00,
            "currency": "USD",
            "description": "Nexus Pro - 100 seats - October 2024",
            "status": "paid",
            "issued_date": format_date(BASE_DATE + timedelta(days=1)),
            "due_date": format_date(BASE_DATE + timedelta(days=31)),
            "paid_date": format_date(BASE_DATE + timedelta(days=18)),
        },
    ])

    # Purchase orders from vendors
    purchase_orders.extend([
        {
            "id": "po-2024-001",
            "type": "PurchaseOrder",
            "vendor": "CloudServices (AWS)",
            "contact": "tbrown@cloudservices.net",
            "amount": 35000.00,
            "currency": "USD",
            "description": "AWS infrastructure - Q4 2024",
            "status": "approved",
            "created_date": format_date(BASE_DATE + timedelta(days=5)),
            "approved_by": "James Mitchell",
        },
        {
            "id": "po-2024-002",
            "type": "PurchaseOrder",
            "vendor": "ComplianceCo",
            "contact": "audits@complianceco.com",
            "amount": 35000.00,
            "currency": "USD",
            "description": "SOC2 Type II Audit Services",
            "status": "approved",
            "created_date": format_date(BASE_DATE + timedelta(days=18)),
            "approved_by": "Sarah Chen",
        },
        {
            "id": "po-2024-003",
            "type": "PurchaseOrder",
            "vendor": "Legal Partners LLP",
            "contact": "awhite@legalfirm.com",
            "amount": 15000.00,
            "currency": "USD",
            "description": "Legal services - Contract review, corporate matters Q4",
            "status": "approved",
            "created_date": format_date(BASE_DATE + timedelta(days=10)),
            "approved_by": "James Mitchell",
        },
    ])

    return invoices, purchase_orders


# ============================================================================
# MAIN GENERATOR
# ============================================================================

def main():
    """Generate all mock data and save to files."""
    print("Generating mock company data for Nexus Technologies...")

    all_emails = []
    all_slacks = []
    all_docs = []
    all_events = []

    # Generate each story
    print("  - Acme deal story...")
    e, s, d, ev = generate_acme_deal_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - Project Phoenix story...")
    e, s, d, ev = generate_phoenix_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - Series A fundraise story...")
    e, s, d, ev = generate_fundraise_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - Q4 budget story...")
    e, s, d, ev = generate_budget_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - SOC2 compliance story...")
    e, s, d, ev = generate_soc2_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - Customer success story...")
    e, s, d, ev = generate_customer_success_story()
    all_emails.extend(e)
    all_slacks.extend(s)
    all_docs.extend(d)
    all_events.extend(ev)

    print("  - Random slack chatter...")
    all_slacks.extend(generate_random_slack_chatter())

    print("  - Random operational emails...")
    all_emails.extend(generate_random_emails())

    print("  - ERP data (invoices, POs)...")
    invoices, pos = generate_erp_data()

    # Create output directory
    OUTPUT_DIR.mkdir(exist_ok=True)

    # Save all data
    with open(OUTPUT_DIR / "emails.json", "w") as f:
        json.dump(all_emails, f, indent=2)

    with open(OUTPUT_DIR / "slack.json", "w") as f:
        json.dump(all_slacks, f, indent=2)

    with open(OUTPUT_DIR / "documents.json", "w") as f:
        json.dump(all_docs, f, indent=2)

    with open(OUTPUT_DIR / "calendar.json", "w") as f:
        json.dump(all_events, f, indent=2)

    with open(OUTPUT_DIR / "invoices.json", "w") as f:
        json.dump(invoices, f, indent=2)

    with open(OUTPUT_DIR / "purchase_orders.json", "w") as f:
        json.dump(pos, f, indent=2)

    # Also save combined file for easy ingestion
    all_data = all_emails + all_slacks + all_docs + all_events + invoices + pos
    with open(OUTPUT_DIR / "all_data.json", "w") as f:
        json.dump(all_data, f, indent=2)

    # Print summary
    print(f"\n{'='*50}")
    print("MOCK DATA GENERATED")
    print(f"{'='*50}")
    print(f"Emails:          {len(all_emails)}")
    print(f"Slack messages:  {len(all_slacks)}")
    print(f"Documents:       {len(all_docs)}")
    print(f"Calendar events: {len(all_events)}")
    print(f"Invoices:        {len(invoices)}")
    print(f"Purchase orders: {len(pos)}")
    print(f"Total records:   {len(all_data)}")
    print(f"\nSaved to: {OUTPUT_DIR}")

    # Print stories summary
    print(f"\nStories included:")
    print("  1. Acme Corp Enterprise Deal ($432K TCV)")
    print("  2. Project Phoenix Platform Rewrite")
    print("  3. Series A Fundraise ($15M target)")
    print("  4. Q4 Budget Planning")
    print("  5. SOC2 Compliance Initiative")
    print("  6. Customer Success (Global Retail)")


if __name__ == "__main__":
    main()
