# Examples

Starting points for working with AnyProfile from code or from an AI agent. Licensed **0BSD**: copy them into
your own scripts, with or without credit. The app itself stays AGPL.

| Folder | What |
|---|---|
| `playwright-node/` | Node + `playwright-core` |
| `puppeteer-node/` | Node + `puppeteer-core` |
| `python-playwright/` | Python + `playwright` |
| `recipes/` | Recipes for AI agents over MCP (`check-login.md`) |

## Scripts

1. Settings → Local API → On, then Copy token. `export ANYPROFILE_TOKEN=<token>` (PowerShell:
   `$env:ANYPROFILE_TOKEN="<token>"`). Another port than 51525: `ANYPROFILE_PORT`.
2. Install and run:

   ```bash
   cd examples/playwright-node && npm install && node run.mjs "Shop VN 01" https://www.youtube.com/
   cd examples/puppeteer-node && npm install && node run.mjs "#12"
   cd examples/python-playwright && python -m venv .venv && . .venv/bin/activate \
     && pip install -r requirements.txt && python run.py "Shop VN 01"
   ```

   No browser download is needed: the scripts drive the profile's own browser.
3. Replace `task()` with your steps.

Each script does the parts that are easy to get wrong:

- finds the profile by id, exact name or `#serial`, as the app does;
- opens it with DevTools for this launch only, and closes it at the end, even after an error, unless it was
  already open;
- uses the profile's own context and pages (`contexts[0]`), which carry its cookies; a new context would not;
- never sets a viewport or emulates a device, which would no longer match the profile's fingerprint;
- lets AnyProfile close the browser, so the profile's session and tabs are saved.

Wait for elements or load states, not for a fixed time. Keep a person's pace: one profile at a time, no bursts.
Doing the same thing in many accounts at once is what platforms look for.

## Recipes

A recipe is a Markdown file with a front matter (name, description, arguments) and steps written for an AI agent.
Put it in AnyProfile's recipes folder (Settings → Local API → Open recipes folder). Claude Code then shows it as
`/mcp__anyprofile__<name>`, and an agent can also find it with the `recipe` tool. See `docs/mcp.md`.
