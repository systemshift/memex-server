#!/usr/bin/env python3
"""
Screen Workflow Monitor - Tacit Knowledge Capture System

Captures screenshots, uses GPT-4o vision to extract workflow knowledge,
and ingests it into memex for other employees to query.

Usage:
    python screen_monitor.py --user "john" --interval 30
"""

import argparse
import base64
import hashlib
import io
import json
import os
import re
import signal
import subprocess
import sys
import time
from datetime import datetime
from typing import Optional

import mss
import requests
from openai import OpenAI
from PIL import Image
from dotenv import load_dotenv

load_dotenv()

# ============== Configuration ==============

MEMEX_API = os.getenv("MEMEX_API", "http://localhost:8080")
DEFAULT_INTERVAL = 30  # seconds between captures
MAX_IMAGE_SIZE = (1920, 1080)  # Resize large screenshots to save API costs


# ============== Screen Capture Module ==============

def get_session_type() -> str:
    """Detect if running X11 or Wayland."""
    return os.environ.get("XDG_SESSION_TYPE", "x11").lower()


class ScreenCapture:
    """Captures screenshots and gathers window metadata."""

    def __init__(self, interval: int = DEFAULT_INTERVAL):
        self.interval = interval
        self.session_type = get_session_type()
        self.last_capture_time = 0
        self.last_window_title = None
        self.temp_screenshot = "/tmp/memex_screenshot.png"

        # Only init mss for X11
        if self.session_type == "x11":
            self.sct = mss.mss()
        else:
            self.sct = None
            print(f"Detected Wayland session - using gnome-screenshot")

    def get_active_window_info(self) -> dict:
        """Get info about the currently focused window."""
        # Try Wayland/GNOME method first (gdbus)
        if self.session_type == "wayland":
            try:
                result = subprocess.run(
                    ["gdbus", "call", "--session",
                     "--dest", "org.gnome.Shell",
                     "--object-path", "/org/gnome/Shell",
                     "--method", "org.gnome.Shell.Eval",
                     "global.display.focus_window ? global.display.focus_window.title : 'Desktop'"],
                    capture_output=True, text=True, timeout=2
                )
                if result.returncode == 0 and "true" in result.stdout:
                    # Parse: (true, "'Window Title'")
                    match = re.search(r"'([^']*)'", result.stdout)
                    if match:
                        window_title = match.group(1)

                        # Get app name
                        result2 = subprocess.run(
                            ["gdbus", "call", "--session",
                             "--dest", "org.gnome.Shell",
                             "--object-path", "/org/gnome/Shell",
                             "--method", "org.gnome.Shell.Eval",
                             "global.display.focus_window ? global.display.focus_window.wm_class : 'Unknown'"],
                            capture_output=True, text=True, timeout=2
                        )
                        app_name = "Unknown"
                        if result2.returncode == 0:
                            match2 = re.search(r"'([^']*)'", result2.stdout)
                            if match2:
                                app_name = match2.group(1)

                        return {"window_title": window_title, "app_name": app_name}
            except:
                pass

        # Try xdotool (X11 or XWayland)
        try:
            result = subprocess.run(
                ["xdotool", "getactivewindow", "getwindowname"],
                capture_output=True, text=True, timeout=2
            )
            window_title = result.stdout.strip() if result.returncode == 0 else None

            result = subprocess.run(
                ["xdotool", "getactivewindow", "getwindowclassname"],
                capture_output=True, text=True, timeout=2
            )
            app_name = result.stdout.strip() if result.returncode == 0 else None

            if window_title and app_name:
                return {"window_title": window_title, "app_name": app_name}
        except:
            pass

        # Fallback: try xprop
        try:
            result = subprocess.run(
                ["xprop", "-root", "_NET_ACTIVE_WINDOW"],
                capture_output=True, text=True, timeout=2
            )
            if "window id" in result.stdout.lower():
                window_id = result.stdout.split()[-1]
                result = subprocess.run(
                    ["xprop", "-id", window_id, "WM_NAME", "WM_CLASS"],
                    capture_output=True, text=True, timeout=2
                )
                lines = result.stdout.strip().split('\n')
                window_title = "Unknown"
                app_name = "Unknown"
                for line in lines:
                    if "WM_NAME" in line:
                        window_title = line.split('=', 1)[-1].strip().strip('"')
                    if "WM_CLASS" in line:
                        parts = line.split('=', 1)[-1].strip().split(',')
                        if parts:
                            app_name = parts[-1].strip().strip('"')
                return {"window_title": window_title, "app_name": app_name}
        except:
            pass

        # Final fallback
        return {"window_title": "Desktop", "app_name": "Unknown"}

    def capture_wayland(self) -> bytes:
        """Capture screenshot on Wayland using gnome-screenshot or grim."""
        # Try gnome-screenshot first (GNOME/Ubuntu)
        try:
            subprocess.run(
                ["gnome-screenshot", "-f", self.temp_screenshot],
                capture_output=True, timeout=5, check=True
            )
            with open(self.temp_screenshot, "rb") as f:
                return f.read()
        except:
            pass

        # Try grim (wlroots/Sway)
        try:
            subprocess.run(
                ["grim", self.temp_screenshot],
                capture_output=True, timeout=5, check=True
            )
            with open(self.temp_screenshot, "rb") as f:
                return f.read()
        except:
            pass

        raise RuntimeError("No Wayland screenshot tool found. Install gnome-screenshot or grim.")

    def capture_x11(self) -> bytes:
        """Capture screenshot on X11 using mss."""
        monitor = self.sct.monitors[1]  # Primary monitor
        screenshot = self.sct.grab(monitor)
        img = Image.frombytes("RGB", screenshot.size, screenshot.bgra, "raw", "BGRX")

        buffer = io.BytesIO()
        img.save(buffer, format="PNG", optimize=True)
        return buffer.getvalue()

    def capture(self) -> dict:
        """Capture screenshot and return with metadata."""
        # Capture based on session type
        if self.session_type == "wayland":
            image_bytes = self.capture_wayland()
            img = Image.open(io.BytesIO(image_bytes))
        else:
            image_bytes = self.capture_x11()
            img = Image.open(io.BytesIO(image_bytes))

        # Resize if too large (saves API costs)
        if img.size[0] > MAX_IMAGE_SIZE[0] or img.size[1] > MAX_IMAGE_SIZE[1]:
            img.thumbnail(MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
            buffer = io.BytesIO()
            img.save(buffer, format="PNG", optimize=True)
            image_bytes = buffer.getvalue()

        # Get window info
        window_info = self.get_active_window_info()

        timestamp = datetime.now()

        return {
            "image_bytes": image_bytes,
            "image_base64": base64.b64encode(image_bytes).decode("utf-8"),
            "window_title": window_info.get("window_title", "Unknown"),
            "app_name": window_info.get("app_name", "Unknown"),
            "timestamp": timestamp,
            "timestamp_str": timestamp.isoformat(),
        }

    def should_capture(self) -> tuple[bool, str]:
        """
        Determine if we should capture now.
        Returns (should_capture, reason).
        """
        now = time.time()

        # Check if app changed
        current_window = self.get_active_window_info()
        current_title = current_window.get("window_title", "")

        if self.last_window_title and current_title != self.last_window_title:
            self.last_window_title = current_title
            self.last_capture_time = now
            return True, "app_switch"

        self.last_window_title = current_title

        # Check interval
        if now - self.last_capture_time >= self.interval:
            self.last_capture_time = now
            return True, "interval"

        return False, ""


# ============== Vision Analysis Module ==============

EXTRACTION_PROMPT = """Analyze this screenshot of a user's computer workflow.

Your task is to extract tacit knowledge - the implicit "how things work" information that would help a new team member understand workflows.

Extract:
1. **Current Task**: What is the user doing right now? (1 concise sentence)
2. **Application**: What application/tool is being used? (just the name)
3. **Workflow Step**: What phase of work is this? (e.g., "debugging", "code review", "writing documentation", "testing", "deploying", "researching")
4. **Tacit Knowledge**: What implicit knowledge or insights would help someone understand this? Think about:
   - Configuration tips ("This setting controls X")
   - Debugging patterns ("When you see this error, check Y")
   - Workflow shortcuts ("To do X quickly, use Y")
   - Tool relationships ("After doing X, you need to Y")
   - Context clues ("This file is important because...")
   List 1-3 specific, actionable insights. Be specific to what's visible.
5. **Context**: Any project names, file paths, URLs, or identifiers visible?

Return ONLY valid JSON (no markdown):
{
  "task": "string describing current task",
  "application": "app name",
  "workflow_step": "category",
  "knowledge_insights": ["insight 1", "insight 2"],
  "context": {
    "project": "project name or null",
    "files": ["file paths visible"],
    "urls": ["urls visible"],
    "other": "any other relevant context"
  },
  "confidence": 0.8
}

If the screen shows nothing meaningful (lock screen, screensaver, blank, login), return:
{"skip": true, "reason": "description"}"""


class VisionAnalyzer:
    """Uses GPT-4o to extract workflow knowledge from screenshots."""

    def __init__(self):
        self.client = OpenAI()
        self.model = "gpt-4o"

    def analyze(self, image_base64: str, metadata: dict) -> dict:
        """Send screenshot to GPT-4o and extract structured workflow data."""
        try:
            response = self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {
                        "role": "user",
                        "content": [
                            {"type": "text", "text": EXTRACTION_PROMPT},
                            {
                                "type": "image_url",
                                "image_url": {
                                    "url": f"data:image/png;base64,{image_base64}",
                                    "detail": "low"  # Use low detail to reduce costs
                                }
                            }
                        ]
                    }
                ],
                max_tokens=500,
                temperature=0.3,
            )

            content = response.choices[0].message.content

            # Parse JSON from response
            # Try to extract JSON if wrapped in markdown
            json_match = re.search(r'\{[\s\S]*\}', content)
            if json_match:
                return json.loads(json_match.group())
            else:
                return {"skip": True, "reason": "Could not parse response"}

        except json.JSONDecodeError as e:
            print(f"JSON parse error: {e}")
            return {"skip": True, "reason": f"JSON parse error: {e}"}
        except Exception as e:
            print(f"Vision API error: {e}")
            return {"skip": True, "reason": f"API error: {e}"}


# ============== Memex Ingestion Module ==============

def normalize_id(text: str) -> str:
    """Normalize text for use as node ID."""
    return re.sub(r'[^a-z0-9]+', '-', text.lower().strip())[:50]


class MemexIngester:
    """Stores extracted workflow knowledge in memex."""

    def __init__(self, user: str):
        self.user = user
        self.session_id = datetime.now().strftime("%Y%m%d_%H%M%S")
        self.api = MEMEX_API

    def create_node(self, node_id: str, node_type: str, meta: dict) -> bool:
        """Create or update a node in memex."""
        try:
            resp = requests.post(
                f"{self.api}/api/nodes",
                json={
                    "id": node_id,
                    "type": node_type,
                    "meta": meta,
                },
                timeout=10
            )
            return resp.status_code in (200, 201, 409)  # 409 = already exists
        except Exception as e:
            print(f"Failed to create node {node_id}: {e}")
            return False

    def create_link(self, source: str, target: str, link_type: str, meta: dict = None) -> bool:
        """Create a relationship between nodes."""
        try:
            resp = requests.post(
                f"{self.api}/api/links",
                json={
                    "source": source,
                    "target": target,
                    "type": link_type,
                    "meta": meta or {},
                },
                timeout=10
            )
            return resp.status_code in (200, 201, 409)
        except Exception as e:
            print(f"Failed to create link {source}->{target}: {e}")
            return False

    def ingest(self, screenshot_data: dict, analysis: dict) -> bool:
        """Store screenshot and extracted knowledge in memex."""

        timestamp = screenshot_data["timestamp_str"]
        image_hash = hashlib.sha256(screenshot_data["image_bytes"]).hexdigest()[:16]

        # 1. Create Screenshot source node
        screenshot_id = f"screenshot:{image_hash}"
        self.create_node(screenshot_id, "Screenshot", {
            "timestamp": timestamp,
            "app": screenshot_data["app_name"],
            "window_title": screenshot_data["window_title"],
            "user": self.user,
            "session": self.session_id,
            # Don't store the actual image in the graph - too large
            # Could store path to file or S3 URL if needed
        })

        # 2. Create/merge User node
        user_id = f"user:{normalize_id(self.user)}"
        self.create_node(user_id, "User", {
            "name": self.user,
        })
        self.create_link(screenshot_id, user_id, "CAPTURED_BY")

        # 3. Create/merge Application node
        app_name = analysis.get("application", screenshot_data["app_name"])
        app_id = f"app:{normalize_id(app_name)}"
        self.create_node(app_id, "Application", {
            "name": app_name,
        })
        self.create_link(screenshot_id, app_id, "CAPTURED_IN")

        # 4. Create Task node
        task_desc = analysis.get("task", "Unknown task")
        task_id = f"task:{normalize_id(task_desc)}"
        self.create_node(task_id, "Task", {
            "description": task_desc,
            "workflow_step": analysis.get("workflow_step", ""),
            "context": analysis.get("context", {}),
        })
        self.create_link(screenshot_id, task_id, "SHOWS_TASK")
        self.create_link(user_id, task_id, "PERFORMED")

        # 5. Create Knowledge nodes for each insight
        for insight in analysis.get("knowledge_insights", []):
            if not insight or len(insight) < 10:
                continue

            knowledge_hash = hashlib.sha256(insight.encode()).hexdigest()[:12]
            knowledge_id = f"knowledge:{knowledge_hash}"
            self.create_node(knowledge_id, "Knowledge", {
                "insight": insight,
                "confidence": analysis.get("confidence", 0.5),
                "source_app": app_name,
                "discovered_by": self.user,
                "discovered_at": timestamp,
            })

            # Link knowledge to its sources
            self.create_link(knowledge_id, screenshot_id, "EXTRACTED_FROM")
            self.create_link(knowledge_id, task_id, "RELATES_TO")
            self.create_link(knowledge_id, app_id, "ABOUT_APP")

        # 6. Create Workflow session node (aggregate)
        workflow_id = f"workflow:{self.user}:{self.session_id}"
        self.create_node(workflow_id, "Workflow", {
            "user": self.user,
            "session_id": self.session_id,
            "started_at": timestamp,
        })
        self.create_link(screenshot_id, workflow_id, "PART_OF_SESSION")

        return True


# ============== Main Daemon ==============

class WorkflowMonitor:
    """Main daemon that orchestrates capture -> analyze -> ingest."""

    def __init__(self, user: str, interval: int = DEFAULT_INTERVAL):
        self.user = user
        self.capture = ScreenCapture(interval)
        self.analyzer = VisionAnalyzer()
        self.ingester = MemexIngester(user)
        self.running = False
        self.capture_count = 0
        self.skip_count = 0

    def handle_signal(self, signum, frame):
        """Handle shutdown signals gracefully."""
        print(f"\nReceived signal {signum}, shutting down...")
        self.running = False

    def run(self):
        """Main loop: capture -> analyze -> ingest."""
        self.running = True

        # Setup signal handlers
        signal.signal(signal.SIGINT, self.handle_signal)
        signal.signal(signal.SIGTERM, self.handle_signal)

        print("=" * 60)
        print("Screen Workflow Monitor")
        print("=" * 60)
        print(f"User: {self.user}")
        print(f"Capture interval: {self.capture.interval}s")
        print(f"Session ID: {self.ingester.session_id}")
        print(f"Memex API: {MEMEX_API}")
        print("=" * 60)
        print("Monitoring... (Ctrl+C to stop)\n")

        while self.running:
            try:
                should_capture, reason = self.capture.should_capture()

                if should_capture:
                    # Capture screenshot
                    data = self.capture.capture()
                    print(f"[{data['timestamp'].strftime('%H:%M:%S')}] "
                          f"Captured ({reason}): {data['app_name']} - {data['window_title'][:40]}...")

                    # Analyze with GPT-4o
                    print("  Analyzing with GPT-4o...")
                    analysis = self.analyzer.analyze(data["image_base64"], data)

                    if analysis.get("skip"):
                        self.skip_count += 1
                        print(f"  Skipped: {analysis.get('reason', 'No meaningful content')}")
                    else:
                        # Ingest into memex
                        print(f"  Task: {analysis.get('task', 'Unknown')}")
                        print(f"  Insights: {len(analysis.get('knowledge_insights', []))}")

                        self.ingester.ingest(data, analysis)
                        self.capture_count += 1

                        for insight in analysis.get("knowledge_insights", [])[:2]:
                            print(f"    -> {insight[:60]}...")

                    print()

                time.sleep(1)

            except Exception as e:
                print(f"Error in main loop: {e}")
                time.sleep(5)

        print("\n" + "=" * 60)
        print(f"Session complete!")
        print(f"Captures processed: {self.capture_count}")
        print(f"Captures skipped: {self.skip_count}")
        print("=" * 60)


# ============== Entry Point ==============

def main():
    parser = argparse.ArgumentParser(
        description="Screen Workflow Monitor - Capture tacit knowledge from screen activity"
    )
    parser.add_argument(
        "--user", "-u",
        required=True,
        help="Username for this monitoring session"
    )
    parser.add_argument(
        "--interval", "-i",
        type=int,
        default=DEFAULT_INTERVAL,
        help=f"Seconds between captures (default: {DEFAULT_INTERVAL})"
    )

    args = parser.parse_args()

    # Check for OpenAI API key
    if not os.getenv("OPENAI_API_KEY"):
        print("Error: OPENAI_API_KEY environment variable not set.")
        sys.exit(1)

    monitor = WorkflowMonitor(args.user, args.interval)
    monitor.run()


if __name__ == "__main__":
    main()
