"""
Digest generation - LLM filtering and rephrasing.
"""

import json
import logging
import re
import uuid
from datetime import datetime
from typing import List, Optional, Tuple

from openai import AsyncOpenAI

from .config import LLM_MODEL, OPENAI_API_KEY
from .models import Message, Digest, Rephrase
from .store import Store
from .memex_connector import get_user_context

logger = logging.getLogger(__name__)


FILTER_PROMPT = """Given this user's work context:
{context}

Here are messages from the pool. Determine which ones are relevant to this user.
Consider their projects, relationships, and work focus.

Messages:
{messages}

Return JSON only:
{{"relevant_ids": ["id1", "id2", ...], "reasoning": "brief explanation"}}

Be inclusive - when in doubt, include the message. Only exclude if clearly irrelevant."""


DIGEST_PROMPT = """Given this user's work context:
{context}

Create a personalized digest of these messages for them.

Messages:
{messages}

Write a digest that:
1. Summarizes key information relevant to their work
2. Groups related topics together
3. Highlights action items or decisions that affect them
4. Uses clear, concise language

Return only the digest text, no JSON. Write in a professional but friendly tone.
Start with a brief overview, then details. Keep it scannable."""


class DigestGenerator:
    """Generates personalized digests using LLM."""

    def __init__(self, store: Store):
        self.store = store
        self.llm = AsyncOpenAI(api_key=OPENAI_API_KEY) if OPENAI_API_KEY else None

    async def generate_digest(
        self,
        user_email: str,
        force: bool = False,
    ) -> Optional[Tuple[Digest, List[Rephrase]]]:
        """
        Generate a personalized digest for a user.
        Returns the Digest and list of Rephrase records for provenance.
        """
        if not self.llm:
            logger.error("No OpenAI API key configured")
            return None

        # Get messages since last digest
        messages = self.store.get_messages_for_user_digest(user_email)

        if not messages:
            logger.info(f"No new messages for {user_email}")
            return None

        logger.info(f"Processing {len(messages)} messages for {user_email}")

        # Get user context from memex
        context = await get_user_context(user_email)
        logger.debug(f"User context: {context[:200]}...")

        # Filter relevant messages
        relevant_messages = await self._filter_relevant(messages, context)

        if not relevant_messages:
            logger.info(f"No relevant messages for {user_email}")
            return None

        logger.info(f"Found {len(relevant_messages)} relevant messages")

        # Generate digest
        digest_content = await self._generate_digest_content(relevant_messages, context)

        if not digest_content:
            logger.warning(f"Failed to generate digest content for {user_email}")
            return None

        # Create digest record
        digest = Digest(
            id=f"digest:{uuid.uuid4()}",
            user_email=user_email,
            message_ids=[m.id for m in relevant_messages],
            summary=digest_content,
            sent_at=None,  # Will be set when sent
        )

        # Create rephrase records for provenance
        rephrases = []
        for msg in relevant_messages:
            rephrase = Rephrase(
                id=str(uuid.uuid4()),
                original_message_id=msg.id,
                for_user_email=user_email,
                rephrased_content=digest_content,  # Full digest as context
                created_at=datetime.utcnow(),
            )
            rephrases.append(rephrase)

        return digest, rephrases

    async def _filter_relevant(
        self,
        messages: List[Message],
        context: str,
    ) -> List[Message]:
        """Use LLM to filter relevant messages for the user."""
        if len(messages) <= 3:
            # For small batches, include all
            return messages

        # Format messages for prompt
        messages_text = self._format_messages_for_prompt(messages)

        prompt = FILTER_PROMPT.format(
            context=context,
            messages=messages_text,
        )

        try:
            response = await self.llm.chat.completions.create(
                model=LLM_MODEL,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1000,
            )

            text = response.choices[0].message.content
            if not text:
                return messages

            # Parse JSON response
            text = text.strip()
            if "```" in text:
                match = re.search(r'```(?:json)?\s*([\s\S]*?)```', text)
                if match:
                    text = match.group(1)

            data = json.loads(text)
            relevant_ids = set(data.get("relevant_ids", []))

            if not relevant_ids:
                return messages

            return [m for m in messages if m.id in relevant_ids]

        except Exception as e:
            logger.warning(f"Filter error (including all messages): {e}")
            return messages

    async def _generate_digest_content(
        self,
        messages: List[Message],
        context: str,
    ) -> Optional[str]:
        """Generate the digest content using LLM."""
        messages_text = self._format_messages_for_prompt(messages, include_full_body=True)

        prompt = DIGEST_PROMPT.format(
            context=context,
            messages=messages_text,
        )

        try:
            response = await self.llm.chat.completions.create(
                model=LLM_MODEL,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=2000,
            )

            return response.choices[0].message.content

        except Exception as e:
            logger.error(f"Digest generation error: {e}")
            return None

    def _format_messages_for_prompt(
        self,
        messages: List[Message],
        include_full_body: bool = False,
    ) -> str:
        """Format messages for LLM prompt."""
        parts = []
        for msg in messages:
            body = msg.body if include_full_body else msg.body[:500]
            parts.append(
                f"[{msg.id}] From: {msg.from_name or msg.from_email}\n"
                f"Subject: {msg.subject or '(no subject)'}\n"
                f"Date: {msg.created_at.strftime('%Y-%m-%d %H:%M')}\n"
                f"---\n{body}\n"
            )
        return "\n\n".join(parts)


async def generate_user_digest(user_email: str, store: Optional[Store] = None) -> Optional[str]:
    """Convenience function to generate a digest for a user."""
    store = store or Store()
    generator = DigestGenerator(store)

    result = await generator.generate_digest(user_email)
    if result:
        digest, rephrases = result

        # Save digest and rephrases
        store.save_digest(digest)
        for rephrase in rephrases:
            store.save_rephrase(rephrase)

        return digest.summary

    return None
