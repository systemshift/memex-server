"""
Agora configuration - loaded from environment variables.
"""

import os
from dotenv import load_dotenv

load_dotenv()

# SMTP Server (receiving)
SMTP_HOST = os.getenv("AGORA_SMTP_HOST", "0.0.0.0")
SMTP_PORT = int(os.getenv("AGORA_SMTP_PORT", "2525"))
POOL_ADDRESS = os.getenv("AGORA_POOL_ADDRESS", "pool@localhost")

# Database
DB_PATH = os.getenv("AGORA_DB_PATH", "agora.db")

# Memex
MEMEX_URL = os.getenv("MEMEX_URL", "http://localhost:8080")

# Outbound SMTP (sending digests)
OUTBOUND_SMTP_HOST = os.getenv("AGORA_OUTBOUND_SMTP_HOST", "localhost")
OUTBOUND_SMTP_PORT = int(os.getenv("AGORA_OUTBOUND_SMTP_PORT", "587"))
OUTBOUND_SMTP_USER = os.getenv("AGORA_OUTBOUND_SMTP_USER", "")
OUTBOUND_SMTP_PASS = os.getenv("AGORA_OUTBOUND_SMTP_PASS", "")
FROM_ADDRESS = os.getenv("AGORA_FROM_ADDRESS", "agora@localhost")

# LLM
LLM_MODEL = os.getenv("AGORA_LLM_MODEL", "gpt-4o-mini")
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")

# Digest settings
DIGEST_HOUR = int(os.getenv("AGORA_DIGEST_HOUR", "8"))  # Hour to send daily digests
