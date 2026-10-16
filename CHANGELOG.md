# Changelog

The notes of each release tag are shown in the app's update prompt; this file
keeps them together.

## Unreleased

- Buy proxies: every proxy provider in one list, opened in your browser.
  A Creator tools page, starting with PostClaw.

## 0.1.0 (2026-10-17)

First beta.

- Profiles with their own cookies, proxy, timezone, languages and fingerprint,
  on Apostate (default) or Fury.
- macOS on Apple Silicon and Windows 10/11 x64. Windows profiles carry a
  Windows fingerprint; a profile opens only on the OS that created it.
- Several platforms per profile, with start pages that follow the proxy's
  country (TikTok Shop, Shopee, Lazada, Amazon seller centers).
- Proxy checks with warnings for a changed country or timezone and for an IPv6
  exit elsewhere.
- Accounts with passwords and 2FA codes, extensions per profile, bookmark
  import, snapshots, a local API for scripts.
- In-app updates with signed releases, and Copy diagnostics for bug reports.
