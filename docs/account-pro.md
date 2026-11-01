# Account-based Pro (implementation notes)

The account layer is implemented behind `AUTH_ENABLED`. When the flag is off,
everything behaves exactly as before: keys, gifts, Stripe checkout, and
certificates. The sign-in UI is present but returns a clear failure until the
flag and Google credentials are configured.

## Server configuration

Add a Google OAuth **Web application** client with the callback:

```text
https://pro.anytools.world/auth/google/callback
```

Set these Worker secrets:

```bash
cd pro/server
npx wrangler secret put GOOGLE_CLIENT_ID
npx wrangler secret put GOOGLE_CLIENT_SECRET
```

Set the rollout flag (already enabled in production):

```text
AUTH_ENABLED=1
```

The callback returns to a temporary `127.0.0.1` loopback listener owned by the
desktop app. The server never redirects to an arbitrary host.

## API surface

```text
GET  /auth/google/start
GET  /auth/google/callback
POST /v1/auth/device
POST /v1/auth/refresh
POST /v1/auth/logout
GET  /v1/account
POST /v1/account/activate
POST /v1/account/link-license
GET  /v1/account/devices
POST /v1/account/devices/remove
```

`/v1/activate` remains the key-based fallback.

## Security invariants

- Google client secret is a Worker secret; it never enters the app bundle.
- The desktop app generates a PKCE verifier; the server stores only its SHA-256
  challenge in `auth_requests`.
- The callback ticket is random, hashed at rest, valid for 120 seconds, and
  consumed once.
- Session access tokens are short-lived; refresh tokens rotate on every use.
- Tokens live in the encrypted vault/Keychain, not SQLite.
- Account activation still issues the existing Ed25519 device certificate.
- A certificate remains valid offline; account sign-in is not required on every
  launch.

## Migration

Existing keys are untouched. A signed-in customer can use **Link this key** once
to attach the licence to the account. The server refuses a key owned by another
account.

## Current rollout status

- Migration `0005_accounts.sql` applied to production D1.
- Worker deployed with `AUTH_ENABLED=1`; Google client ID and secret are set as Worker secrets.
- Server tests cover ticket exchange, activation, refresh rotation, and key link.
- Frontend tests cover the account section and existing Pro flows.
- Real Google sign-in is enabled at the server level; the final browser
  account picker still needs one human click.


## Sign-in callback troubleshooting (2026-11-01)

The first real sign-in created the Google user and session but had no licence
attached, so device activation returned `account_no_entitlement` and the browser
stayed on the loopback callback. The owner licences were linked to the Google
account, and this computer was activated from that account.

The loopback page now strips `?ticket=...&state=...` from the address bar and
tries to close itself, so a one-time ticket is not left visible.

If an existing session was created before the licence was linked, use
**Settings → Pro → Use this account on this computer**.


## Web account portal

The Worker now serves:

```text
GET  /account            account page (signed out or signed in)
GET  /account/login      Google sign-in
GET  /account/callback   optional compatibility callback
GET  /account/logout     revokes the web session
```

Web sign-in reuses the desktop callback:

```text
https://pro.anytools.world/auth/google/callback
```

`/account/callback` remains as a compatibility route, but no extra Google
redirect URI is required when the desktop callback is already configured.

The web session is a separate opaque cookie (`__Host-anyprofile_account`),
hashed in D1 (`web_sessions`), valid for 30 days. Desktop bearer sessions and
certificates are unchanged.

A signed-in web page shows the account email, licence kind/status, features,
updates-through date, and device count/list.
