"""
Agora CLI - Command-line interface for managing the pool.
"""

import asyncio
import logging

import click

from .config import SMTP_HOST, SMTP_PORT, DB_PATH
from .models import User
from .store import Store

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s'
)


@click.group()
@click.option('--db', default=DB_PATH, help='Database path')
@click.pass_context
def cli(ctx, db):
    """Agora - LLM-powered communication pool."""
    ctx.ensure_object(dict)
    ctx.obj['db'] = db
    ctx.obj['store'] = Store(db)


# Server commands
@cli.command()
@click.option('--host', default=SMTP_HOST, help='SMTP host')
@click.option('--port', default=SMTP_PORT, type=int, help='SMTP port')
def serve(host, port):
    """Start the SMTP server."""
    from .server import run_server
    run_server(host, port)


# User commands
@cli.group()
def user():
    """Manage users."""
    pass


@user.command('add')
@click.argument('email')
@click.option('--name', '-n', help='User name')
@click.option('--frequency', '-f', default='daily',
              type=click.Choice(['daily', 'weekly', 'realtime']),
              help='Digest frequency')
@click.pass_context
def user_add(ctx, email, name, frequency):
    """Add a user to the pool."""
    store = ctx.obj['store']

    user = User(
        email=email,
        name=name,
        digest_frequency=frequency,
    )
    store.add_user(user)
    click.echo(f"Added user: {email}")


@user.command('list')
@click.pass_context
def user_list(ctx):
    """List all users."""
    store = ctx.obj['store']
    users = store.list_users()

    if not users:
        click.echo("No users")
        return

    click.echo(f"{'Email':<30} {'Name':<20} {'Frequency':<10} {'Active'}")
    click.echo("-" * 70)
    for u in users:
        click.echo(f"{u.email:<30} {(u.name or '-'):<20} {u.digest_frequency:<10} {'Yes' if u.active else 'No'}")


@user.command('remove')
@click.argument('email')
@click.pass_context
def user_remove(ctx, email):
    """Deactivate a user."""
    store = ctx.obj['store']
    store.deactivate_user(email)
    click.echo(f"Deactivated user: {email}")


# Digest commands
@cli.group()
def digest():
    """Manage digests."""
    pass


@digest.command('generate')
@click.argument('email')
@click.option('--send/--no-send', default=False, help='Send after generating')
@click.pass_context
def digest_generate(ctx, email, send):
    """Generate a digest for a user."""
    from .digest import DigestGenerator
    from .sender import send_digest

    store = ctx.obj['store']
    user = store.get_user(email)

    if not user:
        click.echo(f"User not found: {email}")
        return

    generator = DigestGenerator(store)
    result = asyncio.run(generator.generate_digest(email))

    if not result:
        click.echo("No messages to digest")
        return

    digest_obj, rephrases = result

    click.echo(f"\n{'='*60}")
    click.echo(f"Digest for {email}")
    click.echo(f"Messages: {len(digest_obj.message_ids)}")
    click.echo(f"{'='*60}\n")
    click.echo(digest_obj.summary)
    click.echo(f"\n{'='*60}")

    # Save
    store.save_digest(digest_obj)
    for r in rephrases:
        store.save_rephrase(r)

    if send:
        if send_digest(digest_obj, user, store):
            click.echo(f"\nSent to {email}")
        else:
            click.echo("\nFailed to send")


@digest.command('send-all')
@click.option('--frequency', '-f', default='daily',
              type=click.Choice(['daily', 'weekly', 'realtime']),
              help='Send to users with this frequency')
@click.pass_context
def digest_send_all(ctx, frequency):
    """Generate and send digests to all users."""
    from .sender import send_all_digests

    store = ctx.obj['store']
    count = send_all_digests(store, frequency)
    click.echo(f"Sent {count} digests")


# Message commands
@cli.group()
def message():
    """View messages."""
    pass


@message.command('list')
@click.option('--limit', '-l', default=20, help='Number of messages')
@click.pass_context
def message_list(ctx, limit):
    """List recent messages."""
    store = ctx.obj['store']
    messages = store.get_messages_since()[-limit:]

    if not messages:
        click.echo("No messages")
        return

    for msg in messages:
        click.echo(f"\n[{msg.id}]")
        click.echo(f"From: {msg.from_name or msg.from_email}")
        click.echo(f"Subject: {msg.subject or '(no subject)'}")
        click.echo(f"Date: {msg.created_at}")
        click.echo(f"---")
        click.echo(msg.body[:200] + ("..." if len(msg.body) > 200 else ""))


# Stats
@cli.command()
@click.pass_context
def stats(ctx):
    """Show pool statistics."""
    store = ctx.obj['store']
    s = store.get_stats()

    click.echo(f"\nAgora Pool Statistics")
    click.echo(f"{'='*30}")
    click.echo(f"Messages: {s['messages']}")
    click.echo(f"Active Users: {s['active_users']}")
    click.echo(f"Digests Sent: {s['digests_sent']}")


# Test command
@cli.command()
@click.argument('email')
@click.option('--subject', '-s', default='Test Message', help='Subject')
@click.option('--body', '-b', default='This is a test message.', help='Body')
@click.pass_context
def test_message(ctx, email, subject, body):
    """Add a test message to the pool."""
    import uuid
    from datetime import datetime, timezone
    from .models import Message

    store = ctx.obj['store']

    msg = Message(
        id=f"test:{uuid.uuid4()}",
        from_email=email,
        subject=subject,
        body=body,
        created_at=datetime.now(timezone.utc),
    )
    store.save_message(msg)
    click.echo(f"Added test message: {msg.id}")


def main():
    cli(obj={})


if __name__ == "__main__":
    main()
