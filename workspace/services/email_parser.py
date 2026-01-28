"""
Email MIME parsing service.

Parses raw email messages into structured EmailMessage objects.
Handles multipart messages, HTML stripping, and header extraction.
"""

import email
import re
import hashlib
from email.header import decode_header
from email.utils import parseaddr, parsedate_to_datetime
from dataclasses import dataclass, field
from typing import Optional, List, Dict, Any
from datetime import datetime
from html.parser import HTMLParser


class HTMLStripper(HTMLParser):
    """Strip HTML tags and extract text content"""

    def __init__(self):
        super().__init__()
        self.reset()
        self.fed = []

    def handle_data(self, data):
        self.fed.append(data)

    def get_data(self):
        return ''.join(self.fed)


def strip_html(html: str) -> str:
    """Convert HTML to plain text"""
    stripper = HTMLStripper()
    try:
        stripper.feed(html)
        return stripper.get_data()
    except Exception:
        # Fallback: crude tag removal
        return re.sub(r'<[^>]+>', '', html)


@dataclass
class EmailAddress:
    """Parsed email address with name and address parts"""
    name: str
    address: str

    def __str__(self):
        if self.name:
            return f"{self.name} <{self.address}>"
        return self.address

    def to_dict(self) -> Dict[str, str]:
        return {"name": self.name, "address": self.address}


@dataclass
class EmailMessage:
    """Parsed email message"""
    message_id: str
    subject: str
    from_addr: EmailAddress
    to_addrs: List[EmailAddress] = field(default_factory=list)
    cc_addrs: List[EmailAddress] = field(default_factory=list)
    date: Optional[datetime] = None
    body_plain: str = ""
    body_html: str = ""
    in_reply_to: Optional[str] = None
    references: List[str] = field(default_factory=list)
    headers: Dict[str, str] = field(default_factory=dict)

    # Computed properties
    thread_id: Optional[str] = None
    content_hash: Optional[str] = None

    def __post_init__(self):
        # Compute content hash for deduplication
        content = f"{self.message_id}{self.subject}{self.body_plain}"
        self.content_hash = hashlib.sha256(content.encode()).hexdigest()

        # Derive thread ID from references or message_id
        if self.references:
            self.thread_id = self.references[0]
        elif self.in_reply_to:
            self.thread_id = self.in_reply_to
        else:
            self.thread_id = self.message_id

    @property
    def body(self) -> str:
        """Get the best available body (prefer plain text)"""
        if self.body_plain:
            return self.body_plain
        if self.body_html:
            return strip_html(self.body_html)
        return ""

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization"""
        return {
            "message_id": self.message_id,
            "subject": self.subject,
            "from": self.from_addr.to_dict(),
            "to": [a.to_dict() for a in self.to_addrs],
            "cc": [a.to_dict() for a in self.cc_addrs],
            "date": self.date.isoformat() if self.date else None,
            "body": self.body,
            "body_plain": self.body_plain,
            "body_html": self.body_html,
            "in_reply_to": self.in_reply_to,
            "references": self.references,
            "thread_id": self.thread_id,
            "content_hash": self.content_hash
        }


def decode_header_value(value: str) -> str:
    """Decode RFC 2047 encoded header values"""
    if not value:
        return ""

    decoded_parts = []
    for part, encoding in decode_header(value):
        if isinstance(part, bytes):
            try:
                decoded_parts.append(part.decode(encoding or 'utf-8', errors='replace'))
            except Exception:
                decoded_parts.append(part.decode('utf-8', errors='replace'))
        else:
            decoded_parts.append(part)

    return ''.join(decoded_parts)


def parse_address(addr_string: str) -> EmailAddress:
    """Parse an email address string into EmailAddress"""
    if not addr_string:
        return EmailAddress(name="", address="")

    name, address = parseaddr(addr_string)
    return EmailAddress(
        name=decode_header_value(name),
        address=address.lower()
    )


def parse_address_list(addr_string: str) -> List[EmailAddress]:
    """Parse a comma-separated list of email addresses"""
    if not addr_string:
        return []

    addresses = []
    # Handle both comma and semicolon separators
    parts = re.split(r'[,;]', addr_string)
    for part in parts:
        part = part.strip()
        if part:
            addresses.append(parse_address(part))

    return addresses


def parse_references(refs_string: str) -> List[str]:
    """Parse References header into list of message IDs"""
    if not refs_string:
        return []

    # Extract all <message-id> patterns
    pattern = r'<[^>]+>'
    return re.findall(pattern, refs_string)


def get_message_body(msg: email.message.Message) -> tuple:
    """Extract plain and HTML body from email message"""
    body_plain = ""
    body_html = ""

    if msg.is_multipart():
        for part in msg.walk():
            content_type = part.get_content_type()
            content_disposition = str(part.get("Content-Disposition", ""))

            # Skip attachments
            if "attachment" in content_disposition:
                continue

            try:
                payload = part.get_payload(decode=True)
                if payload is None:
                    continue

                charset = part.get_content_charset() or 'utf-8'
                text = payload.decode(charset, errors='replace')

                if content_type == "text/plain" and not body_plain:
                    body_plain = text
                elif content_type == "text/html" and not body_html:
                    body_html = text
            except Exception:
                continue
    else:
        # Single part message
        try:
            payload = msg.get_payload(decode=True)
            if payload:
                charset = msg.get_content_charset() or 'utf-8'
                text = payload.decode(charset, errors='replace')

                if msg.get_content_type() == "text/html":
                    body_html = text
                else:
                    body_plain = text
        except Exception:
            pass

    return body_plain.strip(), body_html.strip()


def parse_email(raw_data: bytes) -> Optional[EmailMessage]:
    """
    Parse raw email bytes into EmailMessage.

    Args:
        raw_data: Raw email bytes from IMAP fetch

    Returns:
        EmailMessage object or None if parsing fails
    """
    try:
        msg = email.message_from_bytes(raw_data)

        # Extract message ID
        message_id = msg.get("Message-ID", "")
        if not message_id:
            # Generate one if missing
            content_hash = hashlib.md5(raw_data).hexdigest()[:16]
            message_id = f"<generated-{content_hash}@memex>"

        # Extract subject
        subject = decode_header_value(msg.get("Subject", ""))

        # Extract addresses
        from_addr = parse_address(msg.get("From", ""))
        to_addrs = parse_address_list(msg.get("To", ""))
        cc_addrs = parse_address_list(msg.get("Cc", ""))

        # Extract date
        date = None
        date_str = msg.get("Date")
        if date_str:
            try:
                date = parsedate_to_datetime(date_str)
            except Exception:
                pass

        # Extract thread info
        in_reply_to = msg.get("In-Reply-To", "")
        references = parse_references(msg.get("References", ""))

        # Extract body
        body_plain, body_html = get_message_body(msg)

        # Collect important headers
        headers = {}
        for header in ["X-Priority", "X-Spam-Status", "List-Unsubscribe"]:
            value = msg.get(header)
            if value:
                headers[header] = value

        return EmailMessage(
            message_id=message_id,
            subject=subject,
            from_addr=from_addr,
            to_addrs=to_addrs,
            cc_addrs=cc_addrs,
            date=date,
            body_plain=body_plain,
            body_html=body_html,
            in_reply_to=in_reply_to if in_reply_to else None,
            references=references,
            headers=headers
        )

    except Exception as e:
        print(f"Error parsing email: {e}")
        return None
