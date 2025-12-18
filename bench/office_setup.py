#!/usr/bin/env python3
"""
OAuth Setup Helper for Office Suite Connectors.

This script helps obtain OAuth tokens for Microsoft 365 or Google Workspace
by running a local web server to handle the OAuth callback.

Usage:
    python office_setup.py --provider microsoft --client-id YOUR_ID --client-secret YOUR_SECRET
    python office_setup.py --provider google --client-id YOUR_ID --client-secret YOUR_SECRET

Prerequisites:
    Microsoft:
        1. Go to https://portal.azure.com
        2. Azure Active Directory > App registrations > New registration
        3. Set redirect URI to http://localhost:8765/callback
        4. API permissions: Microsoft Graph > Delegated:
           - Files.Read.All (for OneDrive/SharePoint)
           - Mail.Read (for Outlook)
           - User.Read
        5. Create a client secret under "Certificates & secrets"

    Google:
        1. Go to https://console.cloud.google.com
        2. Create a new project or select existing
        3. Enable APIs: Drive API, Gmail API
        4. OAuth consent screen: Configure for internal or external users
        5. Credentials > Create OAuth 2.0 Client ID (Web application)
        6. Set redirect URI to http://localhost:8765/callback
        7. Download client JSON
"""

import argparse
import json
import os
import secrets
import sys
import webbrowser
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from urllib.parse import urlencode, parse_qs, urlparse

import httpx

DEFAULT_OUTPUT_DIR = Path.home() / ".memex"


class OAuthCallbackHandler(BaseHTTPRequestHandler):
    """Handle OAuth callback."""

    def log_message(self, format, *args):
        pass  # Suppress logs

    def do_GET(self):
        # Parse the callback URL
        parsed = urlparse(self.path)

        if parsed.path == "/callback":
            params = parse_qs(parsed.query)

            if "code" in params:
                self.server.auth_code = params["code"][0]
                self.send_response(200)
                self.send_header("Content-type", "text/html")
                self.end_headers()
                self.wfile.write(b"""
                <html>
                <body style="font-family: sans-serif; text-align: center; padding: 50px;">
                <h1>Authorization Successful!</h1>
                <p>You can close this window and return to the terminal.</p>
                </body>
                </html>
                """)
            elif "error" in params:
                self.server.auth_error = params.get("error_description", params["error"])[0]
                self.send_response(400)
                self.send_header("Content-type", "text/html")
                self.end_headers()
                self.wfile.write(f"""
                <html>
                <body style="font-family: sans-serif; text-align: center; padding: 50px;">
                <h1>Authorization Failed</h1>
                <p>Error: {self.server.auth_error}</p>
                </body>
                </html>
                """.encode())
        else:
            self.send_response(404)
            self.end_headers()


def get_microsoft_token(client_id: str, client_secret: str, tenant_id: str = "common") -> dict:
    """Get Microsoft OAuth token."""
    redirect_uri = "http://localhost:8765/callback"
    state = secrets.token_urlsafe(16)

    # Build authorization URL
    auth_params = {
        "client_id": client_id,
        "response_type": "code",
        "redirect_uri": redirect_uri,
        "scope": "openid profile User.Read Files.Read.All Mail.Read offline_access",
        "state": state,
        "response_mode": "query",
    }

    auth_url = f"https://login.microsoftonline.com/{tenant_id}/oauth2/v2.0/authorize?" + urlencode(auth_params)

    print(f"\nOpening browser for Microsoft authorization...")
    print(f"If browser doesn't open, visit:\n{auth_url}\n")
    webbrowser.open(auth_url)

    # Start local server
    server = HTTPServer(("localhost", 8765), OAuthCallbackHandler)
    server.auth_code = None
    server.auth_error = None

    print("Waiting for authorization callback...")
    while server.auth_code is None and server.auth_error is None:
        server.handle_request()

    if server.auth_error:
        raise Exception(f"Authorization failed: {server.auth_error}")

    # Exchange code for token
    print("Exchanging authorization code for tokens...")
    token_url = f"https://login.microsoftonline.com/{tenant_id}/oauth2/v2.0/token"

    with httpx.Client() as client:
        resp = client.post(
            token_url,
            data={
                "client_id": client_id,
                "client_secret": client_secret,
                "code": server.auth_code,
                "redirect_uri": redirect_uri,
                "grant_type": "authorization_code",
            },
        )
        resp.raise_for_status()
        tokens = resp.json()

    return {
        "client_id": client_id,
        "client_secret": client_secret,
        "tenant_id": tenant_id,
        "access_token": tokens["access_token"],
        "refresh_token": tokens.get("refresh_token"),
        "expires_in": tokens.get("expires_in"),
    }


def get_google_token(client_id: str, client_secret: str) -> dict:
    """Get Google OAuth token."""
    redirect_uri = "http://localhost:8765/callback"
    state = secrets.token_urlsafe(16)

    # Build authorization URL
    auth_params = {
        "client_id": client_id,
        "response_type": "code",
        "redirect_uri": redirect_uri,
        "scope": "https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/gmail.readonly",
        "state": state,
        "access_type": "offline",
        "prompt": "consent",
    }

    auth_url = "https://accounts.google.com/o/oauth2/v2/auth?" + urlencode(auth_params)

    print(f"\nOpening browser for Google authorization...")
    print(f"If browser doesn't open, visit:\n{auth_url}\n")
    webbrowser.open(auth_url)

    # Start local server
    server = HTTPServer(("localhost", 8765), OAuthCallbackHandler)
    server.auth_code = None
    server.auth_error = None

    print("Waiting for authorization callback...")
    while server.auth_code is None and server.auth_error is None:
        server.handle_request()

    if server.auth_error:
        raise Exception(f"Authorization failed: {server.auth_error}")

    # Exchange code for token
    print("Exchanging authorization code for tokens...")

    with httpx.Client() as client:
        resp = client.post(
            "https://oauth2.googleapis.com/token",
            data={
                "client_id": client_id,
                "client_secret": client_secret,
                "code": server.auth_code,
                "redirect_uri": redirect_uri,
                "grant_type": "authorization_code",
            },
        )
        resp.raise_for_status()
        tokens = resp.json()

    return {
        "client_id": client_id,
        "client_secret": client_secret,
        "access_token": tokens["access_token"],
        "refresh_token": tokens.get("refresh_token"),
        "expires_in": tokens.get("expires_in"),
    }


def main():
    parser = argparse.ArgumentParser(description="OAuth Setup for Office Connectors")
    parser.add_argument(
        "--provider",
        choices=["microsoft", "google"],
        required=True,
        help="OAuth provider",
    )
    parser.add_argument(
        "--client-id",
        required=True,
        help="OAuth client ID",
    )
    parser.add_argument(
        "--client-secret",
        required=True,
        help="OAuth client secret",
    )
    parser.add_argument(
        "--tenant-id",
        default="common",
        help="Azure AD tenant ID (Microsoft only, default: common)",
    )
    parser.add_argument(
        "--output",
        help="Output file path (default: ~/.memex/<provider>_credentials.json)",
    )

    args = parser.parse_args()

    # Determine output path
    output_dir = DEFAULT_OUTPUT_DIR
    output_dir.mkdir(exist_ok=True)

    if args.output:
        output_path = Path(args.output)
    else:
        output_path = output_dir / f"{args.provider}_credentials.json"

    print("=" * 60)
    print(f"Office Suite OAuth Setup - {args.provider.title()}")
    print("=" * 60)

    try:
        if args.provider == "microsoft":
            credentials = get_microsoft_token(
                args.client_id,
                args.client_secret,
                args.tenant_id,
            )
        else:
            credentials = get_google_token(
                args.client_id,
                args.client_secret,
            )

        # Save credentials
        with open(output_path, "w") as f:
            json.dump(credentials, f, indent=2)

        # Set restrictive permissions
        os.chmod(output_path, 0o600)

        print("\n" + "=" * 60)
        print("SUCCESS!")
        print("=" * 60)
        print(f"Credentials saved to: {output_path}")
        print(f"\nTo use with office_connector:")
        print(f"  python office_connector.py --provider {args.provider} --auth-file {output_path}")

    except Exception as e:
        print(f"\nERROR: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
