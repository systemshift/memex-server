"""
Memex connector - Get user context and ingest messages.
"""

import logging
from typing import Optional

import httpx

from .config import MEMEX_URL
from .models import Message

logger = logging.getLogger(__name__)


async def get_user_context(email: str) -> str:
    """
    Get a user's context from memex.
    Returns a formatted string describing the user's work context.
    """
    async with httpx.AsyncClient(timeout=30) as client:
        context_parts = []

        # Try to get user node
        user_id = f"user:{email.split('@')[0]}"

        try:
            # Get user's connections via traversal
            resp = await client.get(
                f"{MEMEX_URL}/api/query/traverse",
                params={"start": user_id, "depth": 2},
            )

            if resp.status_code == 200:
                data = resp.json()
                nodes = data.get("nodes", [])

                if nodes:
                    context_parts.append(f"User {email} is connected to:")
                    for node in nodes[:20]:  # Limit to 20 nodes
                        node_type = node.get("Type", "Unknown")
                        node_id = node.get("ID", "")
                        meta = node.get("Meta", {})
                        name = meta.get("name", node_id)
                        context_parts.append(f"  - {node_type}: {name}")

        except Exception as e:
            logger.debug(f"Could not get user traversal: {e}")

        # Try to get recent activity
        try:
            resp = await client.get(
                f"{MEMEX_URL}/api/query/filter",
                params={"type": "Screenshot", "limit": 5},
            )

            if resp.status_code == 200:
                data = resp.json()
                screenshots = data.get("nodes", [])

                if screenshots:
                    context_parts.append("\nRecent activity:")
                    for sid in screenshots[:5]:
                        try:
                            node_resp = await client.get(f"{MEMEX_URL}/api/nodes/{sid}")
                            if node_resp.status_code == 200:
                                node = node_resp.json()
                                meta = node.get("Meta", {})
                                summary = meta.get("summary", "")
                                if summary:
                                    context_parts.append(f"  - {summary[:100]}")
                        except:
                            pass

        except Exception as e:
            logger.debug(f"Could not get recent activity: {e}")

        # Try to get entities associated with user
        try:
            resp = await client.get(
                f"{MEMEX_URL}/api/query/search",
                params={"q": email.split("@")[0], "limit": 10},
            )

            if resp.status_code == 200:
                data = resp.json()
                nodes = data.get("nodes", [])

                if nodes:
                    context_parts.append("\nRelated entities:")
                    for node in nodes[:10]:
                        node_type = node.get("Type", "Unknown")
                        meta = node.get("Meta", {})
                        name = meta.get("name", node.get("ID", ""))
                        context_parts.append(f"  - {node_type}: {name}")

        except Exception as e:
            logger.debug(f"Could not search for user: {e}")

        if context_parts:
            return "\n".join(context_parts)

        return f"No memex context available for {email}"


async def ingest_message(message: Message):
    """
    Ingest a message into memex for knowledge extraction.
    Creates a Message node and links it to sender.
    """
    async with httpx.AsyncClient(timeout=30) as client:
        # Create message node
        node_data = {
            "id": message.id,
            "type": "Message",
            "content": message.body,
            "meta": {
                "from_email": message.from_email,
                "from_name": message.from_name,
                "subject": message.subject,
                "thread_id": message.thread_id,
                "in_reply_to": message.in_reply_to,
                "source": "agora",
                "timestamp": message.created_at.isoformat(),
            },
        }

        try:
            resp = await client.post(
                f"{MEMEX_URL}/api/nodes",
                json=node_data,
                timeout=10,
            )

            if resp.status_code in (200, 201):
                logger.info(f"Ingested message {message.id} to memex")
            else:
                logger.warning(f"Failed to ingest message: {resp.status_code}")

        except Exception as e:
            logger.error(f"Error ingesting message: {e}")
            raise

        # Link to sender (if we have a user node for them)
        sender_id = f"user:{message.from_email.split('@')[0]}"
        try:
            await client.post(
                f"{MEMEX_URL}/api/links",
                json={
                    "source": message.id,
                    "target": sender_id,
                    "type": "SENT_BY",
                },
                timeout=10,
            )
        except:
            pass  # Sender node might not exist

        # Link to thread if it exists
        if message.in_reply_to:
            try:
                await client.post(
                    f"{MEMEX_URL}/api/links",
                    json={
                        "source": message.id,
                        "target": message.in_reply_to,
                        "type": "REPLIES_TO",
                    },
                    timeout=10,
                )
            except:
                pass
