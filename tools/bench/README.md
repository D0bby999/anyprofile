# Bench tools (developer only)

Measurement scripts from the engine evaluation. They are **not** part of the
app and are never bundled into `AnyProfile.app`.

Warning: the `.mjs` scripts launch browsers with `--use-mock-keychain` and
`--remote-allow-origins=*`. Those flags are fine for throwaway test profiles
and are forbidden in the app (see Security in the top-level README).

| Tool | What it does |
|---|---|
| `mcp-serve/` | Runs the real launcher and API with `/mcp` on a throwaway data folder with one profile and a test shop page, for trying an AI agent without touching real profiles. `ANYPROFILE_TOKEN` and `-app <Chromium.app>`. |
| `launch-one/` | Opens one throwaway profile through AnyProfile's real launcher with the diagnostics port on and prints the port. Closing stdin stops it. |
| `detect-sites.mjs` | Visits ipinfo, CreepJS, BrowserScan, deviceandbrowserinfo and Pixelscan, then prints their verdicts. With `CDP_PORT` it attaches to a running browser, for example one from `launch-one`. |
| `chromium-bench.mjs` | Opens N profiles at once and reports memory per process type. |
| `macos-candidate-gates.py` | Disposable macOS headless smoke plus native Accessibility no-API-key-infobar check against a pre-0159 positive control; records exact core hashes and fails on missing windows/control. Mock Keychain only, not native Keychain acceptance. |
| `builtin-release/` | Emits the existing published platform pin as a test descriptor for Windows CI. Refuses placeholder URL/zero hashes; does not sign metadata or admit the unpublished macOS candidate. |
| `proxy-window-evidence.mjs` | Validates private per-site pre/post egress JSON across both lanes; rejects missing evidence, drift and mismatched IPhey-displayed IP. This only verifies sampled egress, not continuous stability or detector PASS. Run `node --test tools/bench/tests/proxy-window-evidence.test.mjs` for fixture checks. |
| `cdp-error-side-effects.mjs`, `cdp-error-contexts.mjs` | Run against an owned disposable browser CDP port; check passive Error formatter side effects, explicit stack/Debugger behavior, iframe and auto-attached worker. The tests do not prove all DevTools/UAF compatibility or external-detector acceptance. |
| `external-session/` | Runs an exact core-hash-verified candidate through the real launcher with an owned temporary DB/profile, mock Keychain and bounded lifetime. Diagnostics are explicit; `-diagnostics=false -start-url <url>` supports native-only observations. Optional `-proxy-file` requires a private 0600 input, imports into in-memory secrets and uses the app's GeoIP/locale/relay flow. Requires a known IPv4, country and timezone; revalidates the exit every three minutes and aborts the owned session on IP/IPv6/country/timezone drift or failed validation. Check egress before and after each site and acceptance phase; the periodic guard alone does not prove a stable window. Session duration is capped at three hours. Never loads user profiles or puts proxy credentials in argv/logs. |
| `pipe-spike/` | Checks the DevTools pipe the fingerprint check relies on: fds 5/6, `--remote-debugging-pipe`, a hidden probe tab, survival after the pipe closes. `SPIKE_SEED=<16 hex>`. |
| `proxy-relay.mjs` | A minimal authenticated-upstream relay. The app uses `internal/netproxy` instead. |
| `anybrowser-pipe-check.mjs` | Runs BrowserScan in AnyBrowser the way the app launches it (relay, DevTools pipe, GOOGLE_* variables). `UPSTREAM=<proxy url>`, args: binary, relay script, seeds, first seed. |
| `anybrowser-cookie-check.mjs` | Checks that a persistent cookie survives a restart with the real Keychain. Pass `mock` only to self-test the script. |

Local macOS candidate checks (new evidence directory; no production profiles):

```sh
python3 tools/bench/macos-candidate-gates.py \
  --candidate-app "$CANDIDATE_APP" --control-app "$PRE_0159_APP" \
  --mode all --output-dir "$NEW_RUN_DIR"
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s tools/bench/tests -p 'test_macos_candidate_gates.py'
```

This tool always uses `--use-mock-keychain` with fresh disposable profiles. A raw
headless run without it can block on `Chromium Safe Storage` Keychain access and
then hit the teardown watchdog. Diagnose the Keychain worker stack, not just the
waiting main thread; use the matching UUID `Chromium Framework.unstripped` to
symbolize `sample` addresses with `atos`. **Never** bypass the watchdog or enable
the public mock encryption key in production. Real-Keychain acceptance requires
an approved native identity/access flow and separate persistence validation.

Native AX queries resolve a list of processes by exact PID before reading the
owned window, wait for a canary title/non-empty stable tree and require the
pre-0159 warning as a positive control. Zero windows, denied Accessibility,
partial trees or a missing positive control are BLOCKED, never proof of absence.
These runs are diagnostic tests, not admissible ground-truth M2 captures.

Check the profile the app would really produce:

```sh
mkfifo /tmp/ctl; (sleep 600 > /tmp/ctl &)
go run ./tools/bench/launch-one -app /path/to/Fury.app -persona macos-m4-1710x1112 \
  -seed 00000000000003e8 < /tmp/ctl          # prints port=NNNNN
CDP_PORT=NNNNN SITES=browserscan node tools/bench/detect-sites.mjs x x -
pkill -f "sleep 600"                         # closes the profile
```

With a proxy, set `ANYPROFILE_PROXY=http://user:pass@host:port` in the
environment rather than passing it as an argument.
