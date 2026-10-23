# AnyProfile

A multi-profile browser manager for creators, on macOS and Windows. Each
profile is a separate browser identity with its own cookies, its own proxy and
a consistent device fingerprint, so several TikTok, YouTube or seller accounts
can be worked on side by side without their sessions mixing.

Status: beta. macOS 13+ on Apple Silicon, and Windows 10/11 x64.

Website: [anytools.world/anyprofile](https://anytools.world/en/anyprofile)
([Tiếng Việt](https://anytools.world/vi/anyprofile)).

## How it works

- **Engines:** [Apostate](https://github.com/heretic-tech/apostate) (default,
  macOS and Windows) and the [Fury](https://github.com/furyteamtop/fury-antidetect-browser)
  core (macOS only), both patched Chromium 155. AnyProfile downloads the pinned
  release on first use, verifies it and runs it as-is.
- **Identity:** a profile gets a persona of the computer's own operating system
  and its own random seed at creation, fixed for the profile's life. A profile
  opens only on the OS that created it: another OS's persona would render with
  this machine's fonts and GPU.
- **Network:** a profile's proxy is reached through a relay on `127.0.0.1`
  inside AnyProfile, so the browser never sees the proxy password. The
  timezone and languages come from the proxy's exit IP, and the app warns when
  the exit moves (country, timezone, an IPv6 exit elsewhere).
- **Buying proxies:** Proxies → Buy proxies lists providers that suit
  long-lived accounts and opens them in your default browser. Some are
  referral links; see `docs/privacy-and-terms.md`.
- **Mobile devices (AnyProfile Pro):** separate Android devices, each with its
  own apps, data and proxy: a LineageOS virtual machine on macOS, an LDPlayer 9
  instance on Windows. A device reaches the internet only through its proxy
  (UDP and IPv6 are blocked), and AI agents can drive it over MCP. Apps can
  still tell it is a virtual machine or emulator. See `docs/mobile.md`.
- **Data:** `~/Library/Application Support/AnyProfile` on macOS,
  `%APPDATA%\AnyProfile` on Windows, holds the SQLite database, the engines
  and one folder per profile.
- **Updates:** Settings → Version checks for a new release once a day and
  installs it after verifying its signature. See `docs/releasing.md`.

## Install

Download the newest release from [the website](https://anytools.world/en/anyprofile)
or [Releases](https://github.com/D0bby999/anyprofile/releases/latest): `AnyProfile-macos-arm64.dmg` (open it,
drag AnyProfile onto Applications) or `AnyProfile-amd64-installer.exe`.
Builds are not signed by Apple or Microsoft yet: on macOS, open it once from
System Settings → Privacy & Security → Open Anyway; on Windows, choose More
info → Run anyway. Windows details: `docs/windows.md`.

## Build

Requirements: Go (the toolchain in `go.mod` is fetched automatically), Node
20+, Rust 1.87+, and the Wails CLI v2.14
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0`). macOS 13+ on
Apple Silicon for the Mac app; Windows with NSIS for the Windows installer.

```sh
make build             # build/bin/AnyProfile.app
make build-windows     # on Windows: build/bin/AnyProfile-amd64-installer.exe
make dev               # live-reload development
make test              # Go unit tests, TypeScript check, Vitest
make lint              # SPDX headers, gofmt, vet (also for Windows), govulncheck, npm audit, clippy
ANYPROFILE_TEST_APOSTATE_APP=/path/to/Chromium.app make test-integration
```

## Help

Settings → Help → Copy diagnostics puts a report on the clipboard with the
app and OS versions and the end of the log, with e-mail and IP addresses,
proxy user names and secrets removed. Paste it into a GitHub issue.

What the app stores and sends, and the terms of use: `docs/privacy-and-terms.md`.

## Support the project

AnyProfile is free. If it saves you time, sponsor it on
[GitHub Sponsors](https://github.com/sponsors/D0bby999), donate by card through
[Stripe](https://donate.stripe.com/5kQaEW4es59c0Ey9reejK00), or use Settings → Support AnyProfile in the app.

## Security

AnyProfile is built to be handed to other people, so:

- **No secret on a command line, in the environment, in a log or in the
  database.** The fingerprint config and the cookie key reach the browser on
  inherited pipes. Proxy passwords, cookie keys and the other secrets are in
  one file encrypted with AES-GCM (`secrets.vault`), whose key lives in the
  macOS Keychain (never synchronized to iCloud) or Windows Credential Manager
  (never roamed). One item, read at launch: a new version asks for Keychain
  access once, not once per secret.
- **Cookies are encrypted with a per-profile key**, not the fixed key that
  `--use-mock-keychain` uses. A fingerprint of the key is kept beside the
  profile. If the key goes missing, the launch is refused rather than
  silently discarding every saved login.
- **No control port is open by default.** The DevTools port (full cookie
  access) opens only when you enable diagnostics for a profile's next launch.
  Even then it is loopback-only, never with `--remote-allow-origins`, and it
  turns itself off afterwards.
- **The local API is off by default.** When on, it listens on `127.0.0.1` only,
  needs a token kept in the Keychain, and refuses requests carrying an `Origin`
  header or a foreign `Host`, so web pages cannot reach it. A DevTools port
  opens only for a launch a script asks for. See `docs/local-api.md`.
- **AI agents over MCP.** With the local API on, Claude Code, Codex, Gemini
  CLI, OpenCode, Cursor, VS Code, Windsurf and other agents connect to
  `127.0.0.1:<port>/mcp` with the same token (Settings copies the setup for
  each), open profiles and work
  in their browsers. They go through the app's own DevTools pipe, so no port
  opens; pages are read in an isolated world and input events are real.
  Agents upload only files from the app's uploads folder, and follow recipes
  dropped into its recipes folder. See `docs/mcp.md`; starter scripts for
  Playwright, Puppeteer and Python are in `examples/`.
- **Extensions run only where you turn them on.** Each one is unpacked into
  the app's library and loaded with `--load-extension` in the profiles chosen.
- **Pinned supply chain.** The Fury core is pinned by SHA-256 of the archive
  and of the binary, plus Apple team ID `JKN8WHX7FS`, and downloaded only from
  a hard-coded GitHub Releases URL. `fury-shared` is pinned by commit, and Go,
  npm and Cargo use lockfiles. `make lint` runs `govulncheck` and `npm audit`.
- **Owner-only files.** The data directory is `0700` and the database `0600`.
  Browsing history and site storage rely on FileVault (BitLocker on Windows),
  and the app warns when it is off.
- **Fails closed.** If AnyProfile quits while a proxied browser is still
  running, that browser loses its network instead of falling back to your real
  IP.

Report a vulnerability privately via GitHub Security Advisories on this
repository. Please do not open a public issue for it.

`tools/bench/` holds developer measurement scripts. They use flags the app
never uses and are not part of the app bundle.

## License

AGPL-3.0-or-later. See `LICENSE` and `NOTICE`.
