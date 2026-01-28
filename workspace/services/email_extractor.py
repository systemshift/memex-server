"""
Email extraction webhook service.

Receives webhooks when Email nodes are created and triggers
LLM-based anchor extraction using the email lens.
"""

import json
from typing import Dict, Any, Optional, List
from datetime import datetime

from services.memex import memex
from core.extraction import extract_anchors_email
from core.types import Anchor
from config.email import email_config


class EmailExtractor:
    """
    Handles webhook events for email extraction.

    When an Email node is created, this service:
    1. Fetches the email content
    2. Extracts anchors using the email lens
    3. Stores anchors with provenance links
    4. Marks the email as processed
    """

    def __init__(self, lens_id: str = "lens:email"):
        self.lens_id = lens_id

    def handle_webhook(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        """
        Handle extraction webhook from Memex subscription.

        Expected payload:
        {
            "event_type": "node.created",
            "node_id": "email:xxx",
            "node_type": "Email",
            "timestamp": "..."
        }

        Returns:
            Result dict with extraction status and anchor count
        """
        event_type = payload.get("event_type")
        node_id = payload.get("node_id")
        node_type = payload.get("node_type")

        # Validate event
        if event_type != "node.created":
            return {"status": "ignored", "reason": "not a creation event"}

        if node_type != "Email":
            return {"status": "ignored", "reason": "not an Email node"}

        if not node_id:
            return {"status": "error", "reason": "no node_id provided"}

        # Process the email
        return self.extract_email(node_id)

    def extract_email(self, email_node_id: str) -> Dict[str, Any]:
        """
        Extract anchors from an email node.

        Args:
            email_node_id: The ID of the Email node

        Returns:
            Result dict with extraction status
        """
        try:
            # Fetch email node
            email_node = memex.get_node(email_node_id)
            if not email_node:
                return {"status": "error", "reason": "email node not found"}

            meta = email_node.meta

            # Check if already processed
            if meta.get("processed"):
                return {"status": "skipped", "reason": "already processed"}

            # Get email content
            subject = meta.get("subject", "")
            body_preview = meta.get("body_preview", "")

            # Try to get full body from source node
            source_id = meta.get("source_id")
            body = body_preview

            if source_id:
                source_node = memex.get_node(source_id)
                if source_node and source_node.meta.get("content"):
                    content = source_node.meta.get("content", "")
                    # Extract body from content (skip subject line)
                    if content.startswith("Subject:"):
                        body = content.split("\n\n", 1)[-1]
                    else:
                        body = content

            # Extract anchors
            anchors = extract_anchors_email(
                subject=subject,
                body=body,
                lens_id=self.lens_id,
                email_node_id=email_node_id,
                store_in_memex=True
            )

            # Mark email as processed
            self._mark_processed(email_node_id, len(anchors))

            return {
                "status": "success",
                "email_node_id": email_node_id,
                "anchors_extracted": len(anchors),
                "anchor_types": self._count_anchor_types(anchors)
            }

        except Exception as e:
            print(f"[EmailExtractor] Error extracting {email_node_id}: {e}")
            return {"status": "error", "reason": str(e)}

    def _mark_processed(self, email_node_id: str, anchor_count: int):
        """Mark an email node as processed"""
        try:
            memex._patch(f"/api/nodes/{email_node_id}", {
                "meta": {
                    "processed": True,
                    "processed_at": datetime.now().isoformat(),
                    "anchor_count": anchor_count
                }
            })
        except Exception as e:
            print(f"[EmailExtractor] Error marking processed: {e}")

    def _count_anchor_types(self, anchors: List[Anchor]) -> Dict[str, int]:
        """Count anchors by type"""
        counts = {}
        for anchor in anchors:
            counts[anchor.type] = counts.get(anchor.type, 0) + 1
        return counts

    def reprocess_email(self, email_node_id: str) -> Dict[str, Any]:
        """
        Force reprocessing of an email.

        Clears processed flag and runs extraction again.
        """
        try:
            # Clear processed flag
            memex._patch(f"/api/nodes/{email_node_id}", {
                "meta": {"processed": False}
            })

            # Extract again
            return self.extract_email(email_node_id)

        except Exception as e:
            return {"status": "error", "reason": str(e)}

    def extract_batch(self, email_node_ids: List[str]) -> Dict[str, Any]:
        """
        Extract anchors from multiple emails.

        Returns summary of results.
        """
        results = {
            "total": len(email_node_ids),
            "success": 0,
            "skipped": 0,
            "errors": 0,
            "total_anchors": 0
        }

        for node_id in email_node_ids:
            result = self.extract_email(node_id)

            if result.get("status") == "success":
                results["success"] += 1
                results["total_anchors"] += result.get("anchors_extracted", 0)
            elif result.get("status") == "skipped":
                results["skipped"] += 1
            else:
                results["errors"] += 1

        return results


# Global extractor instance
email_extractor = EmailExtractor()
