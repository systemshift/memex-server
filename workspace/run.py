#!/usr/bin/env python3
"""
Memex Workspace - Entry Point

Run this file to start the workspace server.
"""

import os
import sys

# Add workspace to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from app import app

if __name__ == '__main__':
    port = int(os.getenv("PORT", 5002))
    # threaded=True allows concurrent requests (needed for SSE + API calls)
    app.run(host='0.0.0.0', port=port, debug=True, threaded=True)
