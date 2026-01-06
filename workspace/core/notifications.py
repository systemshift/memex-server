"""
Real-time notification system for Memex Workspace.

Uses Server-Sent Events (SSE) for push notifications to users.
In-memory storage for demo (use Redis in production).
"""

import json
import time
from datetime import datetime
from typing import Dict, List, Optional, Generator
from threading import Lock
from dataclasses import dataclass, field

from core.types import Notification, NotificationType


class NotificationManager:
    """
    Manages notifications for all users.
    Thread-safe for concurrent access.
    """

    def __init__(self):
        # In-memory storage: user_id -> list of notifications
        self._notifications: Dict[str, List[Notification]] = {}
        self._lock = Lock()

        # Activity log for dashboard
        self._activity_log: List[Dict] = []
        self._max_activity = 100  # Keep last 100 activities

    def notify(
        self,
        to_user_id: str,
        notification_type: NotificationType,
        title: str,
        message: str = "",
        from_user_id: Optional[str] = None,
        work_item_id: Optional[str] = None
    ) -> Notification:
        """
        Send a notification to a user.

        Returns the created notification.
        """
        notification = Notification(
            type=notification_type,
            title=title,
            message=message,
            from_user_id=from_user_id,
            to_user_id=to_user_id,
            work_item_id=work_item_id
        )

        with self._lock:
            if to_user_id not in self._notifications:
                self._notifications[to_user_id] = []
            self._notifications[to_user_id].append(notification)

            # Also log to activity
            self._log_activity({
                "type": "notification",
                "notification_type": notification_type.value,
                "from_user": from_user_id,
                "to_user": to_user_id,
                "title": title,
                "work_item_id": work_item_id,
                "timestamp": datetime.now().isoformat()
            })

        return notification

    def notify_handoff(
        self,
        from_user_id: str,
        to_user_id: str,
        work_item_id: str,
        title: str,
        message: str = ""
    ) -> Notification:
        """Convenience method for handoff notifications"""
        return self.notify(
            to_user_id=to_user_id,
            notification_type=NotificationType.HANDOFF,
            title=title,
            message=message,
            from_user_id=from_user_id,
            work_item_id=work_item_id
        )

    def get_pending(self, user_id: str) -> List[Notification]:
        """Get all unread notifications for a user"""
        with self._lock:
            notifications = self._notifications.get(user_id, [])
            return [n for n in notifications if not n.read]

    def get_all(self, user_id: str) -> List[Notification]:
        """Get all notifications for a user (read and unread)"""
        with self._lock:
            return self._notifications.get(user_id, []).copy()

    def get_count(self, user_id: str) -> int:
        """Get count of unread notifications"""
        return len(self.get_pending(user_id))

    def mark_read(self, notification_id: str, user_id: str) -> bool:
        """Mark a notification as read"""
        with self._lock:
            notifications = self._notifications.get(user_id, [])
            for n in notifications:
                if n.id == notification_id:
                    n.read = True
                    n.read_at = datetime.now()
                    return True
        return False

    def mark_all_read(self, user_id: str) -> int:
        """Mark all notifications as read for a user. Returns count marked."""
        count = 0
        with self._lock:
            notifications = self._notifications.get(user_id, [])
            for n in notifications:
                if not n.read:
                    n.read = True
                    n.read_at = datetime.now()
                    count += 1
        return count

    def clear(self, user_id: str):
        """Clear all notifications for a user"""
        with self._lock:
            self._notifications[user_id] = []

    def stream(self, user_id: str, interval: float = 1.0) -> Generator[str, None, None]:
        """
        Generator for SSE stream of notifications.

        Yields SSE-formatted events when new notifications arrive.
        """
        last_check = datetime.now()

        while True:
            # Get new notifications since last check
            with self._lock:
                notifications = self._notifications.get(user_id, [])
                new_notifications = [
                    n for n in notifications
                    if not n.read and n.created > last_check
                ]

            if new_notifications:
                # Send notification event
                data = {
                    "type": "notifications",
                    "count": len(self.get_pending(user_id)),
                    "new": [n.to_dict() for n in new_notifications]
                }
                yield f"data: {json.dumps(data)}\n\n"
                last_check = datetime.now()
            else:
                # Send heartbeat to keep connection alive
                yield f"data: {json.dumps({'type': 'heartbeat', 'count': self.get_count(user_id)})}\n\n"

            time.sleep(interval)

    def _log_activity(self, activity: Dict):
        """Log an activity for the dashboard"""
        with self._lock:
            self._activity_log.insert(0, activity)
            # Trim to max size
            if len(self._activity_log) > self._max_activity:
                self._activity_log = self._activity_log[:self._max_activity]

    def log_activity(
        self,
        activity_type: str,
        user_id: str,
        title: str,
        details: Optional[Dict] = None
    ):
        """Log a general activity (not notification)"""
        self._log_activity({
            "type": activity_type,
            "user_id": user_id,
            "title": title,
            "details": details or {},
            "timestamp": datetime.now().isoformat()
        })

    def get_activity_log(self, limit: int = 50) -> List[Dict]:
        """Get recent activity for dashboard"""
        with self._lock:
            return self._activity_log[:limit]

    def stream_activity(self, interval: float = 2.0) -> Generator[str, None, None]:
        """
        Generator for SSE stream of activity updates.
        Used by dashboard for real-time activity feed.
        """
        last_count = 0

        while True:
            with self._lock:
                current_count = len(self._activity_log)

            if current_count != last_count:
                # New activity, send update
                data = {
                    "type": "activity",
                    "activities": self.get_activity_log(10)
                }
                yield f"data: {json.dumps(data)}\n\n"
                last_count = current_count
            else:
                # Heartbeat
                yield f"data: {json.dumps({'type': 'heartbeat'})}\n\n"

            time.sleep(interval)


# Global notification manager instance
notifications = NotificationManager()
