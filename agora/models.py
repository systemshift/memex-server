"""
Data models for Agora.
"""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional, List


@dataclass
class Message:
    """A message in the pool."""
    id: str
    from_email: str
    body: str
    from_name: Optional[str] = None
    subject: Optional[str] = None
    thread_id: Optional[str] = None
    in_reply_to: Optional[str] = None
    created_at: datetime = field(default_factory=datetime.utcnow)


@dataclass
class User:
    """A user who receives digests."""
    email: str
    name: Optional[str] = None
    active: bool = True
    digest_frequency: str = "daily"  # daily, weekly, realtime
    created_at: datetime = field(default_factory=datetime.utcnow)


@dataclass
class Digest:
    """A digest sent to a user."""
    id: str
    user_email: str
    message_ids: List[str]
    summary: str
    sent_at: Optional[datetime] = None


@dataclass
class Rephrase:
    """Tracks how a message was rephrased for a user (provenance)."""
    id: str
    original_message_id: str
    for_user_email: str
    rephrased_content: str
    created_at: datetime = field(default_factory=datetime.utcnow)
