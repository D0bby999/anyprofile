# Changelog

The notes of each release tag are shown in the app's update prompt; this file
keeps them together.

## 0.1.1 (2026-10-18)

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
- Settings → Support AnyProfile: donate through GitHub Sponsors or by card
  (Stripe).
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
  only Keychain item. Secrets saved before move in at launch, and their old
  Keychain items are removed, as is a secret's item when it is deleted. Before,
  each secret was its own item and macOS asked again for each one after an
  update, often when a profile was closed.
- Closing a profile's last window brings you back to AnyProfile instead of
  pulling you to another desktop: AnyProfile takes the focus, then the
  browser quits behind it (a front app that quits lets macOS pick any app,
  maybe a full-screen one elsewhere). Apostate only: Fury quits by itself
  with its last window.
- Quitting AnyProfile leaves you on the desktop you are on: it hands the
  focus to the app whose window is on top there before it goes.
- Open no longer brings the new browser to the front: it opens behind the
  app, so several profiles can be opened in a row, and the screen stays on
  the desktop you are on. Focus on the row shows it.
- Close snapshots are about 20 times smaller (a few MB instead of ~30 MB)
  and quicker: they leave out the sites' Cache API storage and the
  components Chromium downloads again by itself. Restoring one keeps the
  components already on disk.
- A closed profile drops Chromium's copies of its downloads
  (component_crx_cache), about 60 MB a profile.
- An engine version no profile uses is removed at launch once a newer one is
  installed (~330 MB each). The newest installed version always stays.
- One proxy for several profiles is a normal choice: the new-profile dialog
  no longer warns about it. The proxy picker still names the profiles that
  use each proxy, and Auto still picks a free one.
- New profile: with Auto as the default and no proxy free (or, for several
  profiles, not enough free), the dialog starts on No proxy instead of
  blocking Create until a proxy is chosen. The real-IP note says so.
- Backup and moving to another computer (Settings, and the first-run page):
  one file with every profile and its logins, the proxies, accounts,
  passwords, keys, extensions and recipes, encrypted with a password you
  choose. Restoring adds only what the computer does not have; nothing there
  is changed. Until now the keys stayed in this Mac's Keychain, so a new or
  broken computer meant every profile signed out.
- Updates: a newer version is downloaded and verified in the background, so
  updating is only a restart ("Restart to update"). An app left open for
  days still learns of a release (looked at every 6 hours, asked at most once
  a day). After an update, "What's new" shows its notes once. On macOS the
  AnyProfile menu has Check for Updates… and Settings… (⌘,).
- Arrange windows: with two profiles or more open, the status bar (and ⌘K)
  puts their windows side by side on the screen AnyProfile is on. Windows on
  another desktop are moved but not brought up, so the screen never jumps.
  Scripts and agents cannot resize windows.
- Agents get through browser dialogs: alert and confirm are answered OK,
  prompt is cancelled, and "Leave site?" keeps the page the first time (asked
  again, it leaves); each is reported in the tool result. Before, a dialog
  froze the tab and every tool after it.
- Agents reach iframes: snapshot shows their content with refs (e12f1),
  same-site and cross-site ones (sign-in boxes, captchas), and click and type
  work there. New tool select_option for native drop-downs.
- Closing a profile on macOS asks the browser over its DevTools pipe first,
  then with SIGHUP: a few browsers whose last window had just closed did not
  hear the signal and were killed, losing their newest cookies.
- Drag profiles into your own order, by the row or the grip beside its
  number (or focus the grip and press ↑ ↓): the row follows the pointer, the
  others slide aside, and it glides into place. The order is kept, pinned
  profiles stay on top, and new ones come last. Works when the list is
  sorted by #; dragging in another order says so, with a button to sort
  by #.
- Arrange windows lays them out in the list's order, not in the order the
  browsers happened to start. Opening several profiles at once arranges
  their windows side by side, behind AnyProfile, while 9 or fewer are open
  (more would be too small to use; Arrange still does it).
- The profile table no longer grows when a profile opens or its status
  changes: the row's buttons keep one width, a long status is cut (hover
  shows it whole), and the buttons stay at the right edge. Columns fold
  away by the room there is, measured with a profile running, so the table
  fits from the smallest window with the details open to a wide screen.
- Settings copies the MCP setup for the agent you pick: Claude Code, Codex
  CLI, Gemini CLI, OpenCode, Cursor, VS Code, Windsurf, or the plain JSON,
  and says where to paste it. The Claude Code command now adds AnyProfile for
  every folder (--scope user), not only the one it ran in. docs/mcp.md walks
  through each agent and what to do when one does not connect.
- Moving profiles to the trash works the same from the row menu, the
  selection bar and the details drawer: at once, with an Undo in the notice
  (the drawer asked first; the others said nothing).

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
