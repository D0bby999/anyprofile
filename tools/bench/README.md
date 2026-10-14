# Bench tools (developer only)

Measurement scripts from the engine evaluation. They are **not** part of the
app and are never bundled into `AnyProfile.app`.

Warning: the `.mjs` scripts launch browsers with `--use-mock-keychain` and
`--remote-allow-origins=*`. Those flags are fine for throwaway test profiles
and are forbidden in the app (see Security in the top-level README).

| Tool | What it does |
|---|---|
| `launch-one/` | Opens one throwaway profile through AnyProfile's real launcher with the diagnostics port on and prints the port. Closing stdin stops it. |
| `detect-sites.mjs` | Visits ipinfo, CreepJS, BrowserScan, deviceandbrowserinfo and Pixelscan, then prints their verdicts. With `CDP_PORT` it attaches to a running browser, for example one from `launch-one`. |
| `chromium-bench.mjs` | Opens N profiles at once and reports memory per process type. |
| `pipe-spike/` | Checks the DevTools pipe the fingerprint check relies on: fds 5/6, `--remote-debugging-pipe`, a hidden probe tab, survival after the pipe closes. `SPIKE_SEED=<16 hex>`. |
| `proxy-relay.mjs` | A minimal authenticated-upstream relay. The app uses `internal/netproxy` instead. |
| `apostate-pipe-check.mjs` | Runs BrowserScan in Apostate the way the app launches it (relay, DevTools pipe, GOOGLE_* variables). `UPSTREAM=<proxy url>`, args: binary, relay script, seeds, first seed. |
| `apostate-cookie-check.mjs` | Checks that a persistent cookie survives a restart with the real Keychain. Pass `mock` only to self-test the script. |

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
