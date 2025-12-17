#!/bin/bash
# Start the Screen Workflow Monitor

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
source .venv/bin/activate

# Get username from system
USER=$(whoami)

echo "Starting Screen Workflow Monitor..."
echo "User: $USER"
echo "Press Ctrl+C to stop"
echo ""

python screen_monitor.py --user "$USER" --interval 30
