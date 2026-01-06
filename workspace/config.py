"""
Configuration for Memex Workspace
"""

import os
from dotenv import load_dotenv

load_dotenv()

# Memex API
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")

# OpenAI
OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

# Server
PORT = int(os.getenv("PORT", 5002))
DEBUG = os.getenv("DEBUG", "true").lower() == "true"
