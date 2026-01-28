"""
Email IMAP ingestion service.

Polls IMAP server for new emails and creates Email nodes in Memex.
"""

import imaplib
import time
import hashlib
from typing import Optional, List, Callable
from datetime import datetime

from services.email_parser import parse_email, EmailMessage
from services.memex import memex
from config.email import email_config, EmailConfig


class IMAPPoller:
    """
    IMAP polling service for email ingestion.

    Connects to an IMAP server, fetches new emails,
    and creates Email nodes in Memex.
    """

    def __init__(self, config: Optional[EmailConfig] = None):
        self.config = config or email_config
        self.connection: Optional[imaplib.IMAP4_SSL] = None
        self.last_uid: int = 0
        self.running: bool = False

        # Callbacks
        self.on_email_ingested: Optional[Callable[[str, EmailMessage], None]] = None
        self.on_error: Optional[Callable[[Exception], None]] = None

    def connect(self) -> bool:
        """Establish connection to IMAP server"""
        try:
            if self.config.imap_ssl:
                self.connection = imaplib.IMAP4_SSL(
                    self.config.imap_host,
                    self.config.imap_port
                )
            else:
                self.connection = imaplib.IMAP4(
                    self.config.imap_host,
                    self.config.imap_port
                )

            self.connection.login(self.config.username, self.config.password)
            self.connection.select(self.config.mailbox)
            print(f"[IMAPPoller] Connected to {self.config.imap_host}")
            return True

        except Exception as e:
            print(f"[IMAPPoller] Connection error: {e}")
            if self.on_error:
                self.on_error(e)
            return False

    def disconnect(self):
        """Close IMAP connection"""
        if self.connection:
            try:
                self.connection.close()
                self.connection.logout()
            except Exception:
                pass
            self.connection = None
            print("[IMAPPoller] Disconnected")

    def fetch_new_emails(self, since_uid: int = 0) -> List[EmailMessage]:
        """
        Fetch emails newer than given UID.

        Args:
            since_uid: Fetch emails with UID greater than this

        Returns:
            List of parsed EmailMessage objects
        """
        if not self.connection:
            if not self.connect():
                return []

        try:
            # Search for messages with UID greater than since_uid
            if since_uid > 0:
                search_criteria = f'(UID {since_uid + 1}:*)'
            else:
                # First run: get recent messages
                search_criteria = 'ALL'

            status, data = self.connection.uid('search', None, search_criteria)
            if status != 'OK':
                print(f"[IMAPPoller] Search failed: {status}")
                return []

            uid_list = data[0].split()
            if not uid_list:
                return []

            # Limit batch size
            uid_list = uid_list[-self.config.batch_size:]

            emails = []
            for uid in uid_list:
                uid_int = int(uid)
                if uid_int <= since_uid:
                    continue

                status, msg_data = self.connection.uid('fetch', uid, '(RFC822)')
                if status != 'OK':
                    continue

                raw_email = msg_data[0][1]
                email_msg = parse_email(raw_email)

                if email_msg:
                    emails.append(email_msg)
                    self.last_uid = max(self.last_uid, uid_int)

            return emails

        except Exception as e:
            print(f"[IMAPPoller] Fetch error: {e}")
            if self.on_error:
                self.on_error(e)
            # Try to reconnect on error
            self.disconnect()
            return []

    def ingest_email(self, email_msg: EmailMessage) -> Optional[str]:
        """
        Ingest a single email into Memex.

        Creates:
        - Source node with content hash
        - Email node with metadata
        - Links between them

        Returns:
            Email node ID or None if failed
        """
        try:
            # Create source node first (content-addressed)
            source_content = f"Subject: {email_msg.subject}\n\n{email_msg.body}"
            source_id = memex.ingest_content(source_content)

            # Create Email node
            email_node_id = f"email:{email_msg.content_hash[:24]}"

            # Prepare metadata
            meta = {
                "message_id": email_msg.message_id,
                "subject": email_msg.subject,
                "from_name": email_msg.from_addr.name,
                "from_email": email_msg.from_addr.address,
                "to": [a.to_dict() for a in email_msg.to_addrs],
                "cc": [a.to_dict() for a in email_msg.cc_addrs],
                "date": email_msg.date.isoformat() if email_msg.date else None,
                "thread_id": email_msg.thread_id,
                "in_reply_to": email_msg.in_reply_to,
                "body_preview": email_msg.body[:500] if email_msg.body else "",
                "content_hash": email_msg.content_hash,
                "source_id": source_id,
                "processed": False,  # Will be set true after extraction
                "ingested_at": datetime.now().isoformat()
            }

            # Create the email node
            result = memex.create_node(
                node_type="Email",
                meta=meta,
                node_id=email_node_id
            )

            if result:
                # Link email to source
                memex.create_link(
                    source=email_node_id,
                    target=source_id,
                    link_type="HAS_CONTENT",
                    meta={"created": datetime.now().isoformat()}
                )

                # Link to thread if this is a reply
                if email_msg.in_reply_to:
                    # Try to find parent email
                    parent_hash = hashlib.sha256(
                        f"{email_msg.in_reply_to}".encode()
                    ).hexdigest()[:24]
                    parent_id = f"email:{parent_hash}"

                    # Create REPLY_TO link (may fail if parent doesn't exist)
                    memex.create_link(
                        source=email_node_id,
                        target=parent_id,
                        link_type="REPLY_TO",
                        meta={"thread_id": email_msg.thread_id}
                    )

                print(f"[IMAPPoller] Ingested email: {email_msg.subject[:50]}")

                if self.on_email_ingested:
                    self.on_email_ingested(email_node_id, email_msg)

                return email_node_id

        except Exception as e:
            print(f"[IMAPPoller] Ingest error: {e}")
            if self.on_error:
                self.on_error(e)

        return None

    def poll_once(self) -> int:
        """
        Poll for new emails and ingest them.

        Returns:
            Number of emails ingested
        """
        emails = self.fetch_new_emails(self.last_uid)
        count = 0

        for email_msg in emails:
            node_id = self.ingest_email(email_msg)
            if node_id:
                count += 1

        if count > 0:
            print(f"[IMAPPoller] Ingested {count} emails")

        return count

    def run(self, stop_event=None):
        """
        Run the polling loop.

        Args:
            stop_event: Optional threading.Event to stop the loop
        """
        self.running = True
        print(f"[IMAPPoller] Starting poll loop (interval: {self.config.poll_interval}s)")

        while self.running:
            if stop_event and stop_event.is_set():
                break

            try:
                self.poll_once()
            except Exception as e:
                print(f"[IMAPPoller] Poll error: {e}")
                if self.on_error:
                    self.on_error(e)

            # Wait for next poll
            for _ in range(self.config.poll_interval):
                if stop_event and stop_event.is_set():
                    break
                if not self.running:
                    break
                time.sleep(1)

        self.disconnect()
        print("[IMAPPoller] Stopped")

    def stop(self):
        """Stop the polling loop"""
        self.running = False


# Global poller instance (lazy initialization)
_poller: Optional[IMAPPoller] = None


def get_poller() -> IMAPPoller:
    """Get or create the global IMAP poller"""
    global _poller
    if _poller is None:
        _poller = IMAPPoller()
    return _poller
