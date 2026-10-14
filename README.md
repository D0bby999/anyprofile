# AnyProfile

A multi-profile browser manager for creators on macOS. Each profile is a
separate browser identity with its own cookies, its own proxy and a consistent
device fingerprint, so several TikTok or YouTube channels can be worked on side
by side without their sessions mixing.

Phase 1 status: Apple Silicon Macs only, one engine (Fury), manual use.

## How it works

- **Engine:** the [Fury](https://github.com/furyteamtop/fury-antidetect-browser)
  core, a patched Chromium 155. AnyProfile downloads the pinned, notarized
  release on first use and verifies it. It runs the core as-is.
- **Identity:** each profile gets one of Fury's measured macOS personas and its
  own random seed at creation. Both are fixed for the profile's life.
- **Network:** a profile's proxy is reached through a relay on `127.0.0.1`
  inside AnyProfile, so the browser never sees the proxy password. The
  timezone, position and languages come from the proxy's exit IP.
- **Data:** `~/Library/Application Support/AnyProfile` holds the SQLite
  database, the engines and one folder per profile.

## Build

Requirements: macOS 13+ on Apple Silicon, Go (the toolchain in `go.mod` is
fetched automatically), Node 20+, Rust 1.87+, and the Wails CLI v2.14
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0`).

```sh
make build             # build/bin/AnyProfile.app
make dev               # live-reload development
make test              # Go unit tests, TypeScript check, Vitest
make lint              # SPDX headers, gofmt, vet, govulncheck, npm audit, clippy
ANYPROFILE_TEST_FURY_APP=/path/to/Fury.app make test-integration
```

## Security

AnyProfile is built to be handed to other people, so:

- **No secret on a command line, in the environment, in a log or in the
  database.** The fingerprint config and the cookie key reach the browser on
  inherited pipes. Proxy passwords and cookie keys live in the macOS Keychain
  (never synchronized to iCloud).
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
- **Extensions run only where you turn them on.** Each one is unpacked into
  the app's library and loaded with `--load-extension` in the profiles chosen.
- **Pinned supply chain.** The Fury core is pinned by SHA-256 of the archive
  and of the binary, plus Apple team ID `JKN8WHX7FS`, and downloaded only from
  a hard-coded GitHub Releases URL. `fury-shared` is pinned by commit, and Go,
  npm and Cargo use lockfiles. `make lint` runs `govulncheck` and `npm audit`.
- **Owner-only files.** The data directory is `0700` and the database `0600`.
  Browsing history and site storage rely on FileVault, and the app warns when
  FileVault is off.
- **Fails closed.** If AnyProfile quits while a proxied browser is still
  running, that browser loses its network instead of falling back to your real
  IP.

Report a vulnerability privately via GitHub Security Advisories on this
repository. Please do not open a public issue for it.

`tools/bench/` holds developer measurement scripts. They use flags the app
never uses and are not part of the app bundle.

## License

AGPL-3.0-or-later. See `LICENSE` and `NOTICE`.
