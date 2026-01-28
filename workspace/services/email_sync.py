"""
Email sync state management.

Persists sync state (last UID, timestamps) in Memex for
resumable email polling.
"""

from typing import Optional, Dict, Any
from datetime import datetime

from services.memex import memex


SYNC_STATE_NODE_ID = "config:email-sync-state"


class EmailSyncState:
    """
    Manages email sync state persistence.

    State is stored as a Config node in Memex, allowing the
    poller to resume from where it left off across restarts.
    """

    def __init__(self, account_id: str = "default"):
        self.account_id = account_id
        self._node_id = f"{SYNC_STATE_NODE_ID}:{account_id}"
        self._cache: Optional[Dict[str, Any]] = None

    def get_state(self) -> Dict[str, Any]:
        """Get current sync state from Memex"""
        if self._cache is not None:
            return self._cache

        try:
            node = memex.get_node(self._node_id)
            if node:
                self._cache = node.meta
                return self._cache
        except Exception as e:
            print(f"[EmailSyncState] Error getting state: {e}")

        # Return default state
        return {
            "last_uid": 0,
            "last_sync": None,
            "emails_ingested": 0,
            "errors": 0
        }

    def save_state(self, state: Dict[str, Any]) -> bool:
        """Save sync state to Memex"""
        try:
            # Update timestamp
            state["updated_at"] = datetime.now().isoformat()

            # Try to update existing node
            result = memex._patch(f"/api/nodes/{self._node_id}", {
                "meta": state
            })

            if result is None:
                # Node doesn't exist, create it
                result = memex.create_node(
                    node_type="Config",
                    meta=state,
                    node_id=self._node_id
                )

            self._cache = state
            return result is not None

        except Exception as e:
            print(f"[EmailSyncState] Error saving state: {e}")
            return False

    def get_last_uid(self) -> int:
        """Get the last processed UID"""
        state = self.get_state()
        return state.get("last_uid", 0)

    def set_last_uid(self, uid: int) -> bool:
        """Update the last processed UID"""
        state = self.get_state()
        state["last_uid"] = uid
        return self.save_state(state)

    def record_sync(self, emails_count: int, errors: int = 0) -> bool:
        """Record a sync operation"""
        state = self.get_state()
        state["last_sync"] = datetime.now().isoformat()
        state["emails_ingested"] = state.get("emails_ingested", 0) + emails_count
        state["errors"] = state.get("errors", 0) + errors
        state["last_batch_size"] = emails_count
        return self.save_state(state)

    def get_stats(self) -> Dict[str, Any]:
        """Get sync statistics"""
        state = self.get_state()
        return {
            "account_id": self.account_id,
            "last_uid": state.get("last_uid", 0),
            "last_sync": state.get("last_sync"),
            "total_emails_ingested": state.get("emails_ingested", 0),
            "total_errors": state.get("errors", 0),
            "last_batch_size": state.get("last_batch_size", 0)
        }

    def reset(self) -> bool:
        """Reset sync state (start fresh)"""
        state = {
            "last_uid": 0,
            "last_sync": None,
            "emails_ingested": 0,
            "errors": 0,
            "reset_at": datetime.now().isoformat()
        }
        return self.save_state(state)


# Global sync state instance
_sync_states: Dict[str, EmailSyncState] = {}


def get_sync_state(account_id: str = "default") -> EmailSyncState:
    """Get or create sync state for an account"""
    if account_id not in _sync_states:
        _sync_states[account_id] = EmailSyncState(account_id)
    return _sync_states[account_id]
