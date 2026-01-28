"""
Lens-based anchor extraction for Memex Workspace.

Uses LLM to extract anchors from text based on a lens definition.
Anchors are stored in Memex with provenance links.
"""

import json
import hashlib
from typing import List, Dict, Any, Optional
from dataclasses import dataclass

from core.types import Anchor
from services.memex import memex
from services.llm import llm


def extract_anchors(
    text: str,
    lens_id: str = "lens:deal-flow",
    store_in_memex: bool = True
) -> List[Anchor]:
    """
    Extract anchors from text using a specified lens.

    Args:
        text: The input text to extract from
        lens_id: The lens to use for extraction (default: deal-flow)
        store_in_memex: Whether to store anchors in Memex graph

    Returns:
        List of extracted Anchor objects
    """
    # Get lens definition from Memex
    lens = memex.get_lens(lens_id)
    if not lens:
        print(f"Warning: Lens {lens_id} not found, using default extraction")
        lens = get_default_lens()

    # Build extraction prompt
    prompt = build_extraction_prompt(text, lens)

    # Extract using LLM
    try:
        response = llm.client.chat.completions.create(
            model=llm.model,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        raw_anchors = result.get("anchors", [])
    except Exception as e:
        print(f"Extraction error: {e}")
        return []

    # Convert to Anchor objects
    anchors = []
    source_id = None

    if store_in_memex:
        # Ingest source text to get content-addressed ID
        source_id = memex.ingest_content(text)

    for raw in raw_anchors:
        anchor = Anchor(
            id=raw.get("id", f"anchor:{hashlib.md5(raw.get('text', '').encode()).hexdigest()[:12]}"),
            type=raw.get("type", "unknown"),
            text=raw.get("text", ""),
            start=raw.get("start", 0),
            end=raw.get("end", 0),
            properties=raw.get("properties", {}),
            matched_patterns=raw.get("matched_patterns", []),
            confidence=raw.get("confidence", 0.8),
            source_id=source_id,
            lens_id=lens_id
        )
        anchors.append(anchor)

        # Store anchor in Memex if requested
        if store_in_memex and source_id:
            store_anchor_in_memex(anchor, source_id, lens_id)

    return anchors


def build_extraction_prompt(text: str, lens: Dict[str, Any]) -> str:
    """Build the extraction prompt using lens definition"""

    lens_meta = lens.get("Meta", lens)
    primitives = lens_meta.get("primitives", {})
    patterns = lens_meta.get("patterns", {})
    hints = lens_meta.get("extraction_hints", "")

    prompt = f"""Extract structured entities from this text using the "{lens_meta.get('name', 'Unknown')}" vocabulary.

## Primitives (types of entities to extract):
{json.dumps(primitives, indent=2)}

## Patterns (meaningful combinations):
{json.dumps(patterns, indent=2)}

## Extraction Hints:
{hints}

## Text to analyze:
"{text}"

## Instructions:
1. Find all entities matching the primitives
2. For each entity, note the exact text span and character positions
3. Check which patterns are satisfied (all required primitives present)
4. Return confidence based on how clearly the entity was expressed

## Output Format (JSON):
{{
    "anchors": [
        {{
            "id": "unique-slug-for-this-anchor",
            "type": "primitive_type",
            "text": "exact text span from input",
            "start": character_offset_start,
            "end": character_offset_end,
            "properties": {{"key": "extracted_value"}},
            "matched_patterns": ["pattern_names_if_applicable"],
            "confidence": 0.0_to_1.0
        }}
    ],
    "summary": "brief description of what was found"
}}

Extract all relevant entities from the text."""

    return prompt


def store_anchor_in_memex(anchor: Anchor, source_id: str, lens_id: str):
    """Store an anchor in Memex with proper links"""

    # Create anchor node
    anchor_node_id = memex.create_node(
        node_type=anchor.type.title(),  # e.g., "Company", "Amount"
        meta={
            "text": anchor.text,
            "start": anchor.start,
            "end": anchor.end,
            "properties": anchor.properties,
            "matched_patterns": anchor.matched_patterns,
            "confidence": anchor.confidence
        },
        node_id=anchor.id
    )

    if anchor_node_id:
        # Link to source (EXTRACTED_FROM)
        memex.create_link(
            source=anchor_node_id,
            target=source_id,
            link_type="EXTRACTED_FROM",
            meta={"extractor": "llm", "model": llm.model}
        )

        # Link to lens (INTERPRETED_THROUGH)
        memex.create_link(
            source=anchor_node_id,
            target=lens_id,
            link_type="INTERPRETED_THROUGH",
            meta={"confidence": anchor.confidence}
        )


def get_default_lens() -> Dict[str, Any]:
    """Return a default lens if none is specified"""
    return {
        "Meta": {
            "name": "General",
            "primitives": {
                "person": "a person's name",
                "organization": "a company or organization name",
                "amount": "a monetary value",
                "date": "a date or time reference",
                "location": "a place or location"
            },
            "patterns": {},
            "extraction_hints": "Extract any clearly mentioned entities"
        }
    }


def detect_handoff_intent(text: str, anchors: List[Anchor]) -> Optional[Dict[str, Any]]:
    """
    Detect if the text indicates a handoff to another person.

    Returns handoff info if detected:
        {"to_user": "username", "message": "handoff message"}
    """
    # Check for handoff keywords
    handoff_keywords = [
        "forward to", "hand off to", "assign to", "send to",
        "pass to", "transfer to", "give to"
    ]

    text_lower = text.lower()
    for keyword in handoff_keywords:
        if keyword in text_lower:
            # Try to find the target user
            # Look for user names after the keyword
            from services.users import DEMO_USERS

            for user_id, user in DEMO_USERS.items():
                if user.name.lower() in text_lower:
                    return {
                        "to_user": user_id,
                        "to_user_name": user.name,
                        "detected_keyword": keyword
                    }

    return None


def extract_with_handoff_detection(
    text: str,
    lens_id: str = "lens:deal-flow"
) -> Dict[str, Any]:
    """
    Extract anchors and detect handoff intent in one pass.

    Returns:
        {
            "anchors": [...],
            "handoff": {"to_user": "...", ...} or None,
            "summary": "..."
        }
    """
    anchors = extract_anchors(text, lens_id, store_in_memex=False)
    handoff = detect_handoff_intent(text, anchors)

    return {
        "anchors": anchors,
        "handoff": handoff,
        "anchor_count": len(anchors),
        "patterns_matched": list(set(
            p for a in anchors for p in a.matched_patterns
        ))
    }


# ============================================
# Email-Specific Extraction
# ============================================

def build_email_extraction_prompt(
    subject: str,
    body: str,
    lens: Dict[str, Any]
) -> str:
    """Build extraction prompt specifically for emails with subject/body zones"""

    lens_meta = lens.get("Meta", lens)
    primitives = lens_meta.get("primitives", {})
    patterns = lens_meta.get("patterns", {})
    hints = lens_meta.get("extraction_hints", "")

    prompt = f"""Extract structured entities from this email using the "{lens_meta.get('name', 'Email Communication')}" vocabulary.

## Primitives (types of entities to extract):
{json.dumps(primitives, indent=2)}

## Patterns (meaningful combinations):
{json.dumps(patterns, indent=2)}

## Extraction Hints:
{hints}

## Email to analyze:

### SUBJECT (zone: subject)
"{subject}"

### BODY (zone: body)
"{body}"

## Instructions:
1. Extract entities from BOTH the subject and body
2. For each entity, specify which zone it came from (subject or body)
3. Calculate character offsets WITHIN each zone (starting at 0 for each zone)
4. Look especially for:
   - People mentioned (names, email addresses)
   - Action items and commitments
   - Decisions and agreements
   - Dates and deadlines
   - Topics and projects discussed
5. Match patterns when all required primitives are present

## Output Format (JSON):
{{
    "anchors": [
        {{
            "id": "unique-slug-for-anchor",
            "type": "primitive_type",
            "text": "exact text span",
            "zone": "subject" or "body",
            "start": character_offset_within_zone,
            "end": character_offset_within_zone,
            "properties": {{"key": "value"}},
            "matched_patterns": ["pattern_names"],
            "confidence": 0.0_to_1.0
        }}
    ],
    "summary": "brief description of email content and extracted entities"
}}

Extract all relevant entities from the email."""

    return prompt


def extract_anchors_email(
    subject: str,
    body: str,
    lens_id: str = "lens:email",
    email_node_id: Optional[str] = None,
    store_in_memex: bool = True
) -> List[Anchor]:
    """
    Extract anchors from an email with subject/body zones.

    This provides zone-aware extraction that tracks whether anchors
    came from the subject or body, with correct offsets for inline
    highlighting.

    Args:
        subject: Email subject line
        body: Email body text
        lens_id: Lens to use (default: lens:email)
        email_node_id: Optional Email node ID for linking
        store_in_memex: Whether to store anchors in Memex

    Returns:
        List of extracted Anchor objects with zone info in properties
    """
    # Get lens definition
    lens = memex.get_lens(lens_id)
    if not lens:
        print(f"Warning: Lens {lens_id} not found, using default email lens")
        lens = get_email_default_lens()

    # Build email-specific prompt
    prompt = build_email_extraction_prompt(subject, body, lens)

    # Extract using LLM
    try:
        response = llm.client.chat.completions.create(
            model=llm.model,
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"}
        )
        result = json.loads(response.choices[0].message.content)
        raw_anchors = result.get("anchors", [])
    except Exception as e:
        print(f"Email extraction error: {e}")
        return []

    # Convert to Anchor objects
    anchors = []

    for raw in raw_anchors:
        zone = raw.get("zone", "body")
        anchor_text = raw.get("text", "")

        # Generate unique ID
        anchor_id = raw.get("id")
        if not anchor_id:
            hash_input = f"{email_node_id or ''}{zone}{anchor_text}"
            anchor_id = f"anchor:{hashlib.md5(hash_input.encode()).hexdigest()[:12]}"

        # Add zone info to properties
        properties = raw.get("properties", {})
        properties["zone"] = zone

        anchor = Anchor(
            id=anchor_id,
            type=raw.get("type", "unknown"),
            text=anchor_text,
            start=raw.get("start", 0),
            end=raw.get("end", 0),
            properties=properties,
            matched_patterns=raw.get("matched_patterns", []),
            confidence=raw.get("confidence", 0.8),
            source_id=email_node_id,
            lens_id=lens_id
        )
        anchors.append(anchor)

        # Store anchor in Memex if requested
        if store_in_memex and email_node_id:
            store_email_anchor_in_memex(anchor, email_node_id, lens_id)

    return anchors


def store_email_anchor_in_memex(anchor: Anchor, email_node_id: str, lens_id: str):
    """Store an email-extracted anchor in Memex with proper links"""

    # Create anchor node
    anchor_node_id = memex.create_node(
        node_type=anchor.type.title(),
        meta={
            "text": anchor.text,
            "zone": anchor.properties.get("zone", "body"),
            "start": anchor.start,
            "end": anchor.end,
            "properties": anchor.properties,
            "matched_patterns": anchor.matched_patterns,
            "confidence": anchor.confidence,
            "extracted_from_email": email_node_id
        },
        node_id=anchor.id
    )

    if anchor_node_id:
        # Link to email (EXTRACTED_FROM)
        memex.create_link(
            source=anchor_node_id,
            target=email_node_id,
            link_type="EXTRACTED_FROM",
            meta={
                "extractor": "llm",
                "model": llm.model,
                "zone": anchor.properties.get("zone", "body")
            }
        )

        # Link to lens (INTERPRETED_THROUGH)
        memex.create_link(
            source=anchor_node_id,
            target=lens_id,
            link_type="INTERPRETED_THROUGH",
            meta={"confidence": anchor.confidence}
        )


def get_email_default_lens() -> Dict[str, Any]:
    """Return a default email lens if none is configured"""
    return {
        "Meta": {
            "name": "Email Communication",
            "primitives": {
                "person": "A person mentioned by name or email",
                "organization": "A company or team",
                "action_item": "A task or commitment",
                "decision": "A decision or agreement",
                "date": "A deadline or time reference",
                "topic": "A subject being discussed"
            },
            "patterns": {
                "commitment": {"required": ["person", "action_item"], "optional": ["date"]},
                "request": {"required": ["action_item"], "optional": ["person", "date"]},
                "meeting": {"required": ["date"], "optional": ["person", "topic"]}
            },
            "extraction_hints": """
Extract people, tasks, decisions, dates, and topics from email content.
Look for action verbs, commitments, and deadlines.
"""
        }
    }
