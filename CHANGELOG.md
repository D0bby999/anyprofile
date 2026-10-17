# Changelog

The notes of each release tag are shown in the app's update prompt; this file
keeps them together.

## Unreleased

- Engine versions: Settings → Engines lists every version of Apostate and
  Fury, to download or remove. New versions arrive in the signed latest.json,
  without an app update; profiles move to a newer version on their own when it
  keeps their machine.
- Fury 0.4.5: its macOS core is rebuilt (0.3.8) so a profile closes when its
  window does, from the second launch on too. Same Chrome 155.0.8059.39; the
  machines of macOS profiles are unchanged.
- Buy proxies: 14 providers suited to long-lived accounts, with their logos and
  proxy types, filtered by type; the button is green so it stands apart.
  Opened in your browser.
- Creator tools: PostClaw, plus free open-source apps to edit video, record the
  screen, caption, clean up audio and upscale images.
- AI agents: Claude Code and other MCP clients open profiles and work in
  their browsers (read, click, type, screenshots, network requests) through
  the local API, without opening a DevTools port. See docs/mcp.md.
- Agents can upload files, only from AnyProfile's uploads folder, including
  through upload buttons that open a hidden file input (YouTube Studio).
- Recipes: step-by-step instructions for agents, dropped into the recipes
  folder, offered as MCP prompts. Starter scripts for Playwright, Puppeteer
  and Python in examples/.
- Profiles opened by the local API or an agent open behind the app you are
  in, and the agent works there without bringing the browser to the front;
  Focus on the profile's row shows it.
- The row of a profile an AI agent is driving pulses violet, like a
  recording light, instead of an Agent chip in the Status column.
- An agent's open_tab opens a tab in the profile's window instead of a
  window of its own, which looked like a second browser.
- Google sign-ins last across restarts. The browser joined each Google
  sign-in made on the web (Chrome sign-in, Dice), failed to get a token of
  its own, and on the next launch signed the web out to match. "Allow Chrome
  sign-in" is now off in every profile, so the browser leaves Google's
  cookies alone.
- Keychain asks at most once per new version, when the app starts. Every
  secret now lives in one encrypted file (secrets.vault) whose key is the
  only Keychain item; secrets saved before move in on first use. Before,
  each secret was its own item and macOS asked again for each one after an
  update, often when a profile was closed.

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
