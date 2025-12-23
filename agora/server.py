"""
SMTP server for receiving emails to the Agora pool.
"""

import asyncio
import email
import logging
import uuid
from datetime import datetime
from email.utils import parseaddr
from typing import Optional

from aiosmtpd.controller import Controller
from aiosmtpd.smtp import Envelope, Session, SMTP

from .config import SMTP_HOST, SMTP_PORT, POOL_ADDRESS, MEMEX_URL
from .models import Message
from .store import Store
from .memex_connector import ingest_message

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(levelname)s] %(message)s')
logger = logging.getLogger(__name__)


class AgoraHandler:
    """Handles incoming emails to the pool."""

    def __init__(self, store: Store):
        self.store = store

    async def handle_RCPT(
        self,
        server: SMTP,
        session: Session,
        envelope: Envelope,
        address: str,
        rcpt_options: list,
    ) -> str:
        """Accept emails to the pool address."""
        # Accept emails to the pool address
        local_part = address.split("@")[0].lower()
        pool_local = POOL_ADDRESS.split("@")[0].lower()

        if local_part == pool_local or local_part == "pool":
            envelope.rcpt_tos.append(address)
            return "250 OK"

        return "550 Recipient not accepted"

    async def handle_DATA(
        self,
        server: SMTP,
        session: Session,
        envelope: Envelope,
    ) -> str:
        """Process received email data."""
        try:
            # Parse email
            msg = email.message_from_bytes(envelope.content)

            # Extract sender
            from_header = msg.get("From", "")
            from_name, from_email = parseaddr(from_header)

            # Extract subject
            subject = msg.get("Subject", "")

            # Extract body
            body = self._extract_body(msg)

            # Threading
            message_id = msg.get("Message-ID", str(uuid.uuid4()))
            in_reply_to = msg.get("In-Reply-To")
            thread_id = msg.get("References", "").split()[0] if msg.get("References") else in_reply_to

            # Create message
            message = Message(
                id=f"agora:{uuid.uuid4()}",
                from_email=from_email or envelope.mail_from,
                from_name=from_name or None,
                subject=subject,
                body=body,
                thread_id=thread_id,
                in_reply_to=in_reply_to,
                created_at=datetime.utcnow(),
            )

            # Save to store
            self.store.save_message(message)
            logger.info(f"Received message from {message.from_email}: {subject[:50]}...")

            # Ingest to memex (async, don't wait)
            asyncio.create_task(self._ingest_to_memex(message))

            return "250 Message accepted"

        except Exception as e:
            logger.error(f"Error processing message: {e}")
            return "451 Error processing message"

    def _extract_body(self, msg: email.message.Message) -> str:
        """Extract the text body from an email."""
        if msg.is_multipart():
            for part in msg.walk():
                content_type = part.get_content_type()
                if content_type == "text/plain":
                    payload = part.get_payload(decode=True)
                    if payload:
                        charset = part.get_content_charset() or "utf-8"
                        return payload.decode(charset, errors="replace")
            # Fall back to HTML if no plain text
            for part in msg.walk():
                if part.get_content_type() == "text/html":
                    payload = part.get_payload(decode=True)
                    if payload:
                        charset = part.get_content_charset() or "utf-8"
                        return payload.decode(charset, errors="replace")
        else:
            payload = msg.get_payload(decode=True)
            if payload:
                charset = msg.get_content_charset() or "utf-8"
                return payload.decode(charset, errors="replace")

        return ""

    async def _ingest_to_memex(self, message: Message):
        """Ingest message to memex for knowledge extraction."""
        try:
            await ingest_message(message)
        except Exception as e:
            logger.warning(f"Failed to ingest to memex: {e}")


def run_server(host: str = SMTP_HOST, port: int = SMTP_PORT):
    """Run the SMTP server."""
    store = Store()
    handler = AgoraHandler(store)

    controller = Controller(
        handler,
        hostname=host,
        port=port,
    )

    logger.info(f"Starting Agora SMTP server on {host}:{port}")
    logger.info(f"Pool address: {POOL_ADDRESS}")

    controller.start()

    try:
        asyncio.get_event_loop().run_forever()
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    finally:
        controller.stop()


if __name__ == "__main__":
    run_server()
