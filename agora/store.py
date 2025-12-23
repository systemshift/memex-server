"""
SQLite storage for Agora messages, users, and digests.
"""

import json
import sqlite3
import uuid
from datetime import datetime
from typing import List, Optional

from .config import DB_PATH
from .models import Message, User, Digest, Rephrase


SCHEMA = """
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    from_email TEXT NOT NULL,
    from_name TEXT,
    subject TEXT,
    body TEXT NOT NULL,
    thread_id TEXT,
    in_reply_to TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    email TEXT PRIMARY KEY,
    name TEXT,
    active BOOLEAN DEFAULT TRUE,
    digest_frequency TEXT DEFAULT 'daily',
    last_digest_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS digests (
    id TEXT PRIMARY KEY,
    user_email TEXT,
    message_ids TEXT,
    summary TEXT,
    sent_at TIMESTAMP,
    FOREIGN KEY (user_email) REFERENCES users(email)
);

CREATE TABLE IF NOT EXISTS rephrases (
    id TEXT PRIMARY KEY,
    original_message_id TEXT,
    for_user_email TEXT,
    rephrased_content TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (original_message_id) REFERENCES messages(id),
    FOREIGN KEY (for_user_email) REFERENCES users(email)
);

CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_digests_user ON digests(user_email);
"""


class Store:
    """SQLite storage for Agora."""

    def __init__(self, db_path: str = DB_PATH):
        self.db_path = db_path
        self._init_db()

    def _init_db(self):
        """Initialize database schema."""
        with sqlite3.connect(self.db_path) as conn:
            conn.executescript(SCHEMA)
            conn.commit()

    def _get_conn(self) -> sqlite3.Connection:
        """Get a database connection."""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        return conn

    # Messages
    def save_message(self, msg: Message) -> str:
        """Save a message to the pool."""
        if not msg.id:
            msg.id = str(uuid.uuid4())

        with self._get_conn() as conn:
            conn.execute(
                """INSERT INTO messages
                   (id, from_email, from_name, subject, body, thread_id, in_reply_to, created_at)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
                (msg.id, msg.from_email, msg.from_name, msg.subject,
                 msg.body, msg.thread_id, msg.in_reply_to, msg.created_at)
            )
            conn.commit()
        return msg.id

    def get_message(self, msg_id: str) -> Optional[Message]:
        """Get a message by ID."""
        with self._get_conn() as conn:
            row = conn.execute(
                "SELECT * FROM messages WHERE id = ?", (msg_id,)
            ).fetchone()
            if row:
                return self._row_to_message(row)
        return None

    def get_messages_since(self, since: Optional[datetime] = None) -> List[Message]:
        """Get messages since a timestamp."""
        with self._get_conn() as conn:
            if since:
                rows = conn.execute(
                    "SELECT * FROM messages WHERE created_at > ? ORDER BY created_at",
                    (since,)
                ).fetchall()
            else:
                rows = conn.execute(
                    "SELECT * FROM messages ORDER BY created_at"
                ).fetchall()
            return [self._row_to_message(r) for r in rows]

    def get_messages_for_user_digest(self, user_email: str) -> List[Message]:
        """Get messages for a user's digest (since their last digest)."""
        with self._get_conn() as conn:
            # Get user's last digest time
            user_row = conn.execute(
                "SELECT last_digest_at FROM users WHERE email = ?", (user_email,)
            ).fetchone()

            if user_row and user_row["last_digest_at"]:
                rows = conn.execute(
                    """SELECT * FROM messages
                       WHERE created_at > ?
                       ORDER BY created_at""",
                    (user_row["last_digest_at"],)
                ).fetchall()
            else:
                rows = conn.execute(
                    "SELECT * FROM messages ORDER BY created_at"
                ).fetchall()

            return [self._row_to_message(r) for r in rows]

    def _row_to_message(self, row: sqlite3.Row) -> Message:
        """Convert a database row to a Message."""
        return Message(
            id=row["id"],
            from_email=row["from_email"],
            from_name=row["from_name"],
            subject=row["subject"],
            body=row["body"],
            thread_id=row["thread_id"],
            in_reply_to=row["in_reply_to"],
            created_at=datetime.fromisoformat(row["created_at"]) if row["created_at"] else datetime.utcnow()
        )

    # Users
    def add_user(self, user: User) -> str:
        """Add a user."""
        with self._get_conn() as conn:
            conn.execute(
                """INSERT OR REPLACE INTO users
                   (email, name, active, digest_frequency, created_at)
                   VALUES (?, ?, ?, ?, ?)""",
                (user.email, user.name, user.active, user.digest_frequency, user.created_at)
            )
            conn.commit()
        return user.email

    def get_user(self, email: str) -> Optional[User]:
        """Get a user by email."""
        with self._get_conn() as conn:
            row = conn.execute(
                "SELECT * FROM users WHERE email = ?", (email,)
            ).fetchone()
            if row:
                return self._row_to_user(row)
        return None

    def get_active_users(self, frequency: str = "daily") -> List[User]:
        """Get active users with a specific digest frequency."""
        with self._get_conn() as conn:
            rows = conn.execute(
                """SELECT * FROM users
                   WHERE active = TRUE AND digest_frequency = ?""",
                (frequency,)
            ).fetchall()
            return [self._row_to_user(r) for r in rows]

    def list_users(self) -> List[User]:
        """List all users."""
        with self._get_conn() as conn:
            rows = conn.execute("SELECT * FROM users").fetchall()
            return [self._row_to_user(r) for r in rows]

    def update_user_last_digest(self, email: str, timestamp: datetime):
        """Update when user last received a digest."""
        with self._get_conn() as conn:
            conn.execute(
                "UPDATE users SET last_digest_at = ? WHERE email = ?",
                (timestamp, email)
            )
            conn.commit()

    def deactivate_user(self, email: str):
        """Deactivate a user."""
        with self._get_conn() as conn:
            conn.execute(
                "UPDATE users SET active = FALSE WHERE email = ?", (email,)
            )
            conn.commit()

    def _row_to_user(self, row: sqlite3.Row) -> User:
        """Convert a database row to a User."""
        return User(
            email=row["email"],
            name=row["name"],
            active=bool(row["active"]),
            digest_frequency=row["digest_frequency"],
            created_at=datetime.fromisoformat(row["created_at"]) if row["created_at"] else datetime.utcnow()
        )

    # Digests
    def save_digest(self, digest: Digest) -> str:
        """Save a digest record."""
        if not digest.id:
            digest.id = str(uuid.uuid4())

        with self._get_conn() as conn:
            conn.execute(
                """INSERT INTO digests
                   (id, user_email, message_ids, summary, sent_at)
                   VALUES (?, ?, ?, ?, ?)""",
                (digest.id, digest.user_email, json.dumps(digest.message_ids),
                 digest.summary, digest.sent_at)
            )
            conn.commit()
        return digest.id

    # Rephrases (provenance)
    def save_rephrase(self, rephrase: Rephrase) -> str:
        """Save a rephrase record for provenance."""
        if not rephrase.id:
            rephrase.id = str(uuid.uuid4())

        with self._get_conn() as conn:
            conn.execute(
                """INSERT INTO rephrases
                   (id, original_message_id, for_user_email, rephrased_content, created_at)
                   VALUES (?, ?, ?, ?, ?)""",
                (rephrase.id, rephrase.original_message_id, rephrase.for_user_email,
                 rephrase.rephrased_content, rephrase.created_at)
            )
            conn.commit()
        return rephrase.id

    def get_rephrases_for_message(self, message_id: str) -> List[Rephrase]:
        """Get all rephrases of a message (for provenance tracking)."""
        with self._get_conn() as conn:
            rows = conn.execute(
                "SELECT * FROM rephrases WHERE original_message_id = ?",
                (message_id,)
            ).fetchall()
            return [
                Rephrase(
                    id=r["id"],
                    original_message_id=r["original_message_id"],
                    for_user_email=r["for_user_email"],
                    rephrased_content=r["rephrased_content"],
                    created_at=datetime.fromisoformat(r["created_at"]) if r["created_at"] else datetime.utcnow()
                )
                for r in rows
            ]

    # Stats
    def get_stats(self) -> dict:
        """Get pool statistics."""
        with self._get_conn() as conn:
            msg_count = conn.execute("SELECT COUNT(*) FROM messages").fetchone()[0]
            user_count = conn.execute("SELECT COUNT(*) FROM users WHERE active = TRUE").fetchone()[0]
            digest_count = conn.execute("SELECT COUNT(*) FROM digests").fetchone()[0]

            return {
                "messages": msg_count,
                "active_users": user_count,
                "digests_sent": digest_count,
            }
