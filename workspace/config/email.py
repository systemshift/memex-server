"""
Email configuration for Memex Workspace.

Supports IMAP connection to any email provider.
"""

import os
from dataclasses import dataclass, field
from typing import Optional, List


@dataclass
class EmailConfig:
    """Configuration for IMAP email connection"""

    # IMAP server settings
    imap_host: str = field(default_factory=lambda: os.getenv("IMAP_HOST", "imap.gmail.com"))
    imap_port: int = field(default_factory=lambda: int(os.getenv("IMAP_PORT", "993")))
    imap_ssl: bool = field(default_factory=lambda: os.getenv("IMAP_SSL", "true").lower() == "true")

    # Authentication
    username: str = field(default_factory=lambda: os.getenv("EMAIL_USERNAME", ""))
    password: str = field(default_factory=lambda: os.getenv("EMAIL_PASSWORD", ""))

    # Mailbox settings
    mailbox: str = field(default_factory=lambda: os.getenv("EMAIL_MAILBOX", "INBOX"))

    # Polling settings
    poll_interval: int = field(default_factory=lambda: int(os.getenv("EMAIL_POLL_INTERVAL", "60")))
    batch_size: int = field(default_factory=lambda: int(os.getenv("EMAIL_BATCH_SIZE", "50")))

    # Processing settings
    auto_extract: bool = field(default_factory=lambda: os.getenv("EMAIL_AUTO_EXTRACT", "true").lower() == "true")
    lens_id: str = field(default_factory=lambda: os.getenv("EMAIL_LENS_ID", "lens:email"))

    # Webhook for extraction
    extraction_webhook: str = field(default_factory=lambda: os.getenv(
        "EMAIL_EXTRACTION_WEBHOOK",
        "http://localhost:5002/api/webhooks/extract-email"
    ))

    # Memex API
    memex_url: str = field(default_factory=lambda: os.getenv("MEMEX_URL", "http://localhost:8080"))

    def is_configured(self) -> bool:
        """Check if email is properly configured"""
        return bool(self.username and self.password and self.imap_host)

    def validate(self) -> List[str]:
        """Validate configuration, return list of errors"""
        errors = []
        if not self.imap_host:
            errors.append("IMAP_HOST is required")
        if not self.username:
            errors.append("EMAIL_USERNAME is required")
        if not self.password:
            errors.append("EMAIL_PASSWORD is required (use app-specific password for Gmail)")
        if self.poll_interval < 10:
            errors.append("EMAIL_POLL_INTERVAL should be at least 10 seconds")
        return errors


# Global config instance
email_config = EmailConfig()
