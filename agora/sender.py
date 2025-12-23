"""
Email sender for Agora digests.
"""

import logging
import smtplib
from datetime import datetime
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from typing import Optional

from .config import (
    OUTBOUND_SMTP_HOST,
    OUTBOUND_SMTP_PORT,
    OUTBOUND_SMTP_USER,
    OUTBOUND_SMTP_PASS,
    FROM_ADDRESS,
    POOL_ADDRESS,
)
from .models import Digest, User
from .store import Store

logger = logging.getLogger(__name__)


def send_digest(digest: Digest, user: User, store: Store) -> bool:
    """
    Send a digest email to a user.
    Returns True if successful.
    """
    try:
        # Create message
        msg = MIMEMultipart("alternative")
        msg["Subject"] = f"Your Agora Digest - {datetime.utcnow().strftime('%Y-%m-%d')}"
        msg["From"] = FROM_ADDRESS
        msg["To"] = user.email
        msg["Reply-To"] = POOL_ADDRESS

        # Plain text version
        text_content = digest.summary

        # HTML version (simple formatting)
        html_content = _format_digest_html(digest, user)

        msg.attach(MIMEText(text_content, "plain"))
        msg.attach(MIMEText(html_content, "html"))

        # Send
        if OUTBOUND_SMTP_USER and OUTBOUND_SMTP_PASS:
            # Authenticated SMTP
            with smtplib.SMTP(OUTBOUND_SMTP_HOST, OUTBOUND_SMTP_PORT) as server:
                server.starttls()
                server.login(OUTBOUND_SMTP_USER, OUTBOUND_SMTP_PASS)
                server.send_message(msg)
        else:
            # Local SMTP (no auth)
            with smtplib.SMTP(OUTBOUND_SMTP_HOST, OUTBOUND_SMTP_PORT) as server:
                server.send_message(msg)

        # Update digest record
        digest.sent_at = datetime.utcnow()
        store.save_digest(digest)

        # Update user's last digest time
        store.update_user_last_digest(user.email, datetime.utcnow())

        logger.info(f"Sent digest to {user.email}")
        return True

    except Exception as e:
        logger.error(f"Failed to send digest to {user.email}: {e}")
        return False


def _format_digest_html(digest: Digest, user: User) -> str:
    """Format digest as HTML email."""
    # Simple HTML formatting
    content = digest.summary.replace("\n\n", "</p><p>").replace("\n", "<br>")

    return f"""<!DOCTYPE html>
<html>
<head>
    <style>
        body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }}
        .header {{ border-bottom: 2px solid #007AFF; padding-bottom: 10px; margin-bottom: 20px; }}
        .header h1 {{ margin: 0; color: #007AFF; font-size: 24px; }}
        .content {{ background: #f9f9f9; padding: 20px; border-radius: 8px; }}
        .footer {{ margin-top: 20px; padding-top: 10px; border-top: 1px solid #eee; font-size: 12px; color: #666; }}
        p {{ margin: 1em 0; }}
    </style>
</head>
<body>
    <div class="header">
        <h1>Agora Digest</h1>
        <p>Hello {user.name or user.email.split('@')[0]},</p>
        <p>Here's your personalized summary of recent pool activity.</p>
    </div>
    <div class="content">
        <p>{content}</p>
    </div>
    <div class="footer">
        <p>This digest was generated from {len(digest.message_ids)} messages.</p>
        <p>Reply to this email to post to the pool.</p>
    </div>
</body>
</html>"""


def send_all_digests(store: Store, frequency: str = "daily") -> int:
    """
    Send digests to all active users with the specified frequency.
    Returns the number of digests sent.
    """
    from .digest import DigestGenerator
    import asyncio

    users = store.get_active_users(frequency)
    sent_count = 0

    generator = DigestGenerator(store)

    for user in users:
        try:
            # Generate digest
            result = asyncio.run(generator.generate_digest(user.email))

            if result:
                digest, rephrases = result

                # Save provenance
                for rephrase in rephrases:
                    store.save_rephrase(rephrase)

                # Send
                if send_digest(digest, user, store):
                    sent_count += 1

        except Exception as e:
            logger.error(f"Error processing digest for {user.email}: {e}")

    return sent_count
