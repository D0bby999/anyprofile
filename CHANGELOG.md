# Changelog

The notes of each release tag are shown in the app's update prompt; this file
keeps them together.

## 0.2.1 (unreleased)

- Creator Tools now includes the Craft App Suite, and hides the Pro invitation when Pro is active.
- Removed Scheduled Posts; the old queue data remains in the database.
- Updated the proxy recommendations and added Oxylabs.

Engine 0.5.1.157 (261107):

- A window no longer paints at the pixel ratio its persona claims. The profile's
  device pixel ratio was written into the screen info the renderer lays out and
  rasterises with — a rendering scale, not a fact a page reads. A seed that drew a
  1x panel (an external 1080p or QHD display on a Mac mini) therefore painted the
  whole browser at half the density of a 2x display and let macOS upscale it, so
  every page arrived soft with faint text, the same look the colour-gamut fix
  removed from a different surface. The ratio is now the display's own, while
  screen.width and height, colour depth, gamut and HDR still come from the
  profile: what a page reads about its display is unchanged. Existing profiles
  move to this build on their next open, because the catalogue digest is the same.

Agent profile management (261104):

- An agent can now manage profiles, not only drive them: 10 new MCP tools
  (`app_status`, `proxy_list`, `profile_create`, `profile_update`,
  `profile_trash`, `profile_restore`, `profile_purge`, `proxy_assign`,
  `proxy_check`, `extension_set`) create profiles, file them in folders and
  tags, assign proxies, turn extensions on and read the app's state. They are
  off by default and appear only when Settings → Local API → "Let agents create
  and change profiles" is on, so an agent cannot call what you did not allow.
- The destructive ones say so twice: `profile_trash` needs `confirm: true`, and
  `profile_purge` needs the profile's exact name; a proxy change is refused
  while the profile is open, because its browser keeps the network it started
  with. Tool answers carry the profile's fields, never its identity seed, its
  saved accounts or a proxy username.
- Errors can now carry a disposition: `retry: "safe"` (call again after the
  step named in the message, like taking a new snapshot) or `retry: "never"`,
  both in the REST error body and at the end of an MCP tool error. Scripts and
  agents stop guessing whether a repeat can help.

## 0.2.0 (2026-11-04)

Pro membership (261104):

- Pro no longer repaints the app. Membership used to swap the working accent to
  the Tailwind violet ramp and wash the sidebar with a radial gradient, so a
  paid user's primary button changed colour — and the only thing a colour then
  said was "you are on the paid tier", while the same hue everywhere else still
  meant "this is the action". Pro is now a material: a seal on the account row,
  a firmer rule on the rail, a deeper surface under the Pro blocks. The accent
  stays blue for everyone.
- One membership marker instead of two. The "PRO" chip in the brand row (9px,
  letter-spaced, uppercase) is gone; the account row at the bottom of the rail
  carries the seal, the name and the state, and it is the only place that does.

Profile status (261104):

- A profile whose last open failed now says so in its row ("Did not open",
  hover for the launcher's message) and counts under "need attention". The
  message used to exist only until the page error cleared, so a browser that
  died before its window appeared left the row looking merely closed, and the
  retry looked like the first attempt.
- A profile with no proxy reads "Real IP" instead of a green "OK": the direct
  connection is not a healthy state of a profile built to travel through a
  proxy, it is the one thing such a profile should not silently be.
- A narrow window still shows which proxy a profile uses. The proxy column is
  the first thing dropped when space runs out, which took away the one fact
  that decides whether a profile is safe to open; the row now carries the
  proxy beside its name instead.

Engine colour (261104):

- AnyBrowser no longer paints wide-gamut pages for a display the machine does
  not have. The profile's colour-gamut and HDR claim used to be written into the
  display colour spaces, which also told the compositor to paint every
  wide-gamut layer into that claimed display: Meta Business Suite, written in
  `color(display-p3 …)`, arrived washed out, with a cyan "Create Post" where
  Chrome paints #3576b9, faint text and no card borders. The two media features
  are now answered from the profile in Blink (0165) and the display spaces stay
  the real ones (0166), so what a page reads is unchanged while what the
  compositor paints is the page as written. The engine was rebuilt
  (0.5.1.156, Chromium 156.0.8078.74) and the pin's hashes updated.

Code signing (261104):

- The app and the engine are signed with the same Developer ID (Cassau, LLC,
  team 25C8SNXS6Q), the hardened runtime and a secure timestamp, and the engine
  archive is published on its own pre-release with its URL, hashes and team
  pinned in the build: a profile will not start on an engine whose signature
  does not name that team.
- Every Mach-O in the engine archive is signed on its own, and each helper gets
  its own entitlements — `allow-jit` for the renderer and the GPU process,
  library validation off for the plain helper. Signing them all with the app's
  entitlements produced a bundle that passed every local check and then could
  not start a renderer under the hardened runtime: the page died with "V8
  process OOM (Failed to reserve virtual memory for CodeRange)" and the profile
  reported "browser failed".
- The archive no longer carries AppleDouble `._` entries. `ditto` stores
  extended attributes as those entries, and `unzip` writes them out as real
  files, so a bundle sealed a moment earlier read as modified for anyone who
  unpacked the zip by hand; the packager now keeps them out and stops if one
  reappears.

Cookie safety (261103):

- A profile's cookie jar is copied aside before every launch (a few tens of kB,
  ten copies kept). When a launch finds that the jar lost most of its cookies —
  what happens when the folder is opened by a build or a tool whose key differs,
  because a browser deletes every cookie it cannot decrypt — the last copy is
  put back before the browser reads it, so the accounts are still signed in
  instead of showing a login page. The emptied file is kept beside the copies
  and the event is written to the log.

Window focus (261103):

- A profile opened from the app now raises its own window once the browser has
  actually created it. It used to ask too early: the app became active while
  the window did not exist yet, and the window that appeared a moment later
  stayed behind, so it looked open but nothing on it answered until "Bring
  window to front" was clicked. The raise is verified and retried a few times,
  and never fought for once the user has moved to another app.

Engine settings (261103):

- The engine list says it at the top now: a profile keeps the engine it was
  created with, and the default engine applies to new profiles only. After
  switching, a toast names the profiles that stay on their own engine, so the
  next Open is not a surprise.

Proxy table (261102):

- The proxy list has fixed columns now: a long label, host, timezone or note is
  cut with an ellipsis (with the whole value in its tooltip) instead of spilling
  over the column beside it, and the table no longer grows past the window.
- "Used by" names one profile and counts the rest ("+4"), every name in the
  tooltip, instead of wrapping five chips into the rows below.
- On a narrow window the expiry column steps out and its room goes to the
  status chip and the names; the date stays whole at every width.

Colour and contrast (261102):

- Hover, selection and state tints were nearly invisible on the white table
  (1.04-1.12:1); they are now steps you can see and tell apart: hover 1.18:1,
  selected 1.23:1, selected under the pointer 1.34:1, in both themes.
- Hovering a selected, running or agent row now darkens it. Those states used
  to override the hover rule, so the row gave no feedback at all.
- The row whose drawer is open, and the row being dragged, carry a 3px accent
  bar at the left edge, so "selected" and "the one I am looking at" are no
  longer the same picture.
- Text selection (drag over a field or a name) is a real blue tint with 12:1
  text instead of a near-white wash; the keyboard focus ring is stronger; the
  faint text used for machine names and placeholders went from 2.6:1 to 4.0:1.
  The Pro theme gets its own violet selection tint.

Row menu (261102):

- The "⋯" menu on a profile row no longer hides behind the action buttons of
  the rows below, and no longer gets cut off at the bottom of the table: it is
  rendered in a layer of its own, follows its button when the table scrolls,
  and flips above the button when there is no room below it.

Window focus (261101):

- Mobile devices opened in a row now stand side by side instead of every first
  window landing on the same spot at the right edge: a phone is tall and
  narrow, so four fit across a 13-inch screen, and the fifth steps slightly off
  the first column. Each device still remembers where the user left it.
- Opening one profile brings that browser to the front. It used to open behind
  the app and then be pushed back several times while starting, which looked
  like the window flickering.
- Opening several profiles at once lays them out in a grid behind the app and
  then raises the whole set once, the first window keeping the focus, so they
  neither land on the same spot nor take the focus from each other. A toast
  says how many opened. The API and agents still open browsers without
  touching the focus.
- Past six windows the grid is skipped, since every pane would be too small to
  work in; the toast offers to arrange them by hand instead of pretending it
  did. ⌘1…⌘9 brings the nth open browser to the front, numbered in the same
  order the windows are arranged in.
- Arrange windows now leaves the window in the first cell in front, instead of
  whichever window happened to be activated last.
- A profile's first window opens at a size the web is laid out for (1280 wide,
  at most 900 tall, fitted to the work area with a margin, stepped for each
  window already open). Chromium keeps whatever size and position the user
  leaves it at afterwards.

Fingerprint reference (261101):

- A profile measured on an older Chromium build no longer reports
  "Fingerprint changed" on every launch. The reference is renewed when the
  browser itself moves to a new Chromium build, and a drift that only the
  build decides (user agent, canvas, WebGL) reads as a warning instead of a
  red alert. Profiles left with a reference from the Apostate build get a
  fresh one on their next launch.

Agent automation (261101):

- MCP agents can find an element by text, role and name, label or test id
  (`find`, `click_text`, `device_find`, `device_tap_text`) and pass a `target`
  object to `click`, `type`, `device_tap` and `device_type`. A step no longer
  needs a full snapshot first; `find` returns refs only, and ambiguity returns
  the candidates instead of guessing. Targets wait up to five seconds for the
  element to appear, so a script does not need a sleep. Mobile adds
  `device_long_press`, `device_drag`, `device_pull_file`, clipboard read/write
  (typed unsupported where Android blocks adb), notifications, and
  camera/microphone/location/notifications/photos permission grant/revoke.
- Web agents can hover, double-click, right-click and drag; `snapshot
  mode=interactive` drops static text; `captcha_check` detects a challenge and
  tells the agent to hand it to the user instead of looping.
- MCP now keeps a per-session lease on `open_profile`/`open_device` (stateful
  streamable sessions), so two agents cannot drive one profile at once — every
  tool on the profile, not only `open_profile`, checks the lease.
- `anyprofile record` records intent steps into a flow JSON (`secret: true`
  keeps typed text out); replay with `anyprofile run`. Automation examples and
  a pass/fail/wall-time harness live in `examples/automation/` and
  `scripts/automation-eval.sh`. Target clicks
  wait for a stable, enabled element, and every MCP call leaves a JSONL trace
  under `logs/traces/` (tool, profile/device, ms, ok, error — no page content).
- Scripts can drive the same actions over HTTP: `/v1/act`, `/v1/run`,
  `/v1/find`, `/v1/trace`, with a TypeScript client, a Python client and a
  dependency-free `anyprofile run flow.json` CLI. HTTP sessions lease a
  profile/device for 60 s with a heartbeat, so two scripts cannot silently
  share a tab.

Pro licence v2 (261031):

- Pro now uses a signed Ed25519 JWS certificate bound to a per-computer
  device key, not just a bearer key. The app verifies it offline, with
  per-kind grace: subscription 7 days, gift 30 days, lifetime five years.
  Lifetime means permanent use plus 12 months of updates/support.
- The owner can grant a lifetime/test licence or create a promo/paid gift
  code without Stripe (`pro/tools/admin.sh`). Gift codes are one-time,
  hashed at rest, and extend the licence already on the computer. The app's
  Pro page has a gift-code field; the UI still calls the feature Mobile.
- A customer can buy a one-year gift from Settings → Pro → Buy a gift. This
  uses Stripe `mode=payment` and the `pro_gift` lookup key; create that
  one-time price before enabling the button in production.
- Customers can manage Pro devices without a password: `/manage` sends a
  20-minute email link to `/manage/devices` to remove one device or reset all.
- Stripe subscriptions keep working and can activate the new certificate.
  The old key path remains for one release.

AnyMobile (261031):

- Mobile now has automation presets (Automation 2 GB/2 CPU/540×960,
  Balanced, Interactive), headless starts for agents, a persistent ADB shell,
  soft/hard reset with a clean-disk snapshot, per-device metrics and the
  `device_reset`/`device_metrics`/`device_show` MCP tools. The app UI still
  calls the feature Mobile and opens a window when the user presses Open.
- Mobile devices now go quiet when nobody uses them: a device without a window
  turns its screen off after 15 s and, after two minutes without a tool call,
  has its vCPUs frozen — CPU near 0, RAM and apps intact, about 60 ms to thaw.
  Screenshots and input wake the screen automatically. scrcpy is capped at
  30 fps / 4 Mbps so an idle window does not heat the Mac.
- Mobile devices can be watched inside the app: **View in app** opens a JPEG
  preview (about 2-3 frames a second) with tap, Back and Home. It is a quick
  look, not a replacement for the smooth scrcpy window.
- The scrcpy window now remembers its position per device, can stay always on
  top, and the row menu can take a screenshot (`~/Pictures/AnyProfile`) or
  record up to three minutes (`~/Movies/AnyProfile`).
- Mobile devices start fast again: closing one saves its RAM state (about
  2.5 GB per device) and reopening it takes 2.6-4.4 s instead of a ~28 s cold
  boot. The Mobile page has a **Fast start** switch that also deletes the
  saved states when turned off. A saved state is valid only for the same
  runtime, image, shape and disks; a restore, reset or crash makes the next
  open cold.

- The Mac mobile runtime is now **AnyMobile**, its internal name: the Pro
  folder is `anymobile/`, its engines are `anymobile` and `anymobile-image`,
  and packages, records and scripts use those names. The app keeps calling the
  feature **Mobile** everywhere. `android-vm` 1.0.0 was never published; the
  old engine id is only kept in historical notes. The runtime now ships its
  GPL/Apache/BSD texts and an `anymobile/verify.sh` gate before publishing.
- Legacy Apostate databases migrate again: the pre-option 0.5.1 profiles that
  migration 21 was written for are verified and rewritten to AnyBrowser after
  a backup, templates follow, the saved default engine is fixed by schema 23,
  and schema 24 moves those profiles to the AnyBrowser release that replaced
  Apostate 0.5.1 (`0.5.1.156`). Unknown or post-option Apostate rows still
  fail closed. Launch history no longer blocks the migration.

Canvas noise on for new profiles (261102):

- Profiles created from now on opt into the profile-seeded readback noise, so
  two profiles on one machine no longer share a canvas hash. It is the same
  switch as before; what changed is the rule behind it: a pixel moves only
  where the window around it holds an edge, and a canvas that only filled
  pixel-aligned rectangles is left exact. Measured with the packaged build:
  Pixelscan "consistent, no masking" with the noise on, IPhey 100, one hash
  across all seven readback routes, two seeds rendering different canvases.
- Profiles created before today keep their stored value, so their pixels do not
  move; their canvas hash stays what it was.

Geolocation from the profile (261102):

- A profile that has a position — copied from the proxy exit — now answers
  geolocation with that position instead of blocking it, and the permission is
  allowed so a page that asks gets an answer. A profile without a position is
  still denied, so no site ever gets this computer's real location. The engine
  answers once a second, so `watchPosition` keeps working.
- The engine needed three pieces, not one: the provider, a system-layer source
  that answers kAllowed (macOS never granted this binary Location Services, so
  the page stayed at `prompt` with a provider installed), and installing that
  manager where the platform one is, because the permission layer and the
  device service both read the process singleton.
- Measured on the rebuilt engine: no flag → `prompt`, deny preference →
  `denied`, flag with the preference allowed → `granted` and 52.5200/13.4050.

Canvas readback policy (261102):

- The engine is rebuilt from `156.0.8078.74` with the same 119 patches plus one
  more (0163). Two surfaces every machine renders identically no longer move
  when the opt-in readback noise is on: a canvas holding only pixel-aligned
  solid rectangles, and a smooth WebGL ramp. Measured with the new
  `tools/bench/canvas-routes.mjs`, which decodes every route in Node: 15.13 % of
  the solid-rect canvas and 91.11 % of the WebGL ramp moved before, both are 0 %
  now, two seeds still render different text canvases, and all seven readback
  routes still return one answer for one canvas.
- New `tools/bench/context-gate.mjs` asks one browser the same questions in
  eight execution contexts (main frame, Worker, SharedWorker, ServiceWorker and
  four iframe kinds) and passes on this build, with a spoofed identity and
  across two launches of one profile. It also passes real Chrome on this host,
  so the checks are calibrated on a browser that is not ours.
- The noise switch is still off for new profiles: the detector run that would
  decide it (pixelscan, fv.pro) needs a proxy.
- Rebuilt on Xcode 27.2. The pinned macOS SDK 26.5 is gone, so this build used
  Command Line Tools 26.2, and `scripts/configure.sh` now names the missing SDK
  instead of dying silently; Xcode 27 also splits out the Metal toolchain, which
  the build downloads once (`xcodebuild -downloadComponent MetalToolchain`).

Security rebase (261031):

- The macOS engine is now Chromium 156.0.8078.74 (the 28–29 October stable
  update: 52 fixes, four critical) instead of .48. All 119 patches applied
  cleanly and the persona catalogue now declares build .74. This is a local
  ad-hoc build: the built-in download URL is still a placeholder and no
  Developer ID/notarization is attached, so installing from the catalog still
  fails closed.
- Two profiles are now compared by Visitor ID and the rest of the composite
  identity; a shared canvas hash on one machine is no longer treated as a
  failure, as real Chrome on the same Mac behaves the same. The manual test
  guide was updated; canvas readback noise stays off.

AnyBrowser identity (261030):

- New profiles now draw a seed the installed AnyBrowser can serve in full. On a
  machine with fewer cores than the chip a seed claims, the engine refuses to
  raise the core count, which used to produce a machine that cannot exist (an
  Apple M4 with eight cores). Profile creation asks the engine for the
  composition, keeps drawing until the chip fits this machine, and pins the
  cores and memory it composed. It costs about 0.1 s per profile. Profiles
  created before this change keep their identity.
- A profile moved to a machine with fewer cores than it was composed with is
  still clamped so it stays usable, but the launch now records a warning naming
  the pinned and the available values instead of changing it silently.
- If AnyBrowser is installed but cannot report what a seed composes, profile
  creation now fails with an error instead of silently pinning this computer's
  machine. Creating a profile before the engine is installed is refused with an
  "install it first" message, and the welcome screen keeps Create disabled until
  the engine is installed.

Proxy affiliate links (261027):

- "Buy proxies" now opens our affiliate referral links for IPRoyal, NodeMaven,
  Decodo, Proxy-Seller, MarsProxies, Evomi, ProxyEmpire, Infatica, 9Proxy,
  Coronium and LTESocks. SOAX, Massive and Froxy still open their home pages
  until their referral links arrive. The dialog already discloses that some
  links are referral links and that the price you pay does not change.

Corrective review (261027):

- Restore folders and user templates with encrypted backups, remapping same-name
  folders before restoring profiles; failed imports still roll back atomically.
- Launch queue is FIFO, cancellable and deduplicated; adopted browsers count
  toward the ceiling, and StopAll cancels pending starts before freeing slots.
  Queued profiles now show a cancel action in the table and drawer.
- Folder bulk moves are atomic; partial folder ordering and concurrent name
  uniqueness are enforced. Template defaults are validated and applied before
  the profile-created notification.
- Profile paging has a stable tie-breaker and order index; backend search folds
  Vietnamese accents. An in-repo benchmark harness measures a 50-row
  page at ~0.44 ms on the development M2 (not an end-to-end UI measurement).

Upgrade round 261027 (backend and API first; the matching screens land
separately, so some of this is not visible in the app yet):

- Launch ceiling: the app can cap how many browsers start or run at once
  (setting `max_concurrent_launches`, 0 = unlimited); extra starts queue
  instead of overwhelming the machine. Set it from Settings once the
  settings section ships; the API binding is `SetMaxConcurrentLaunches`.
- Faster profile lists: listing now loads accounts and extensions only
  for the profiles it returns, and the list API can page
  (`ListProfilesPage`). On the benchmark machine a 50-row page costs the
  same ~1.4 ms and ~0.22 MB whether the database holds 100 or 500
  profiles; a full 500-profile list still costs ~2.1 MB per call.
- Folders (backend and API): file each profile in at most one folder,
  filter the list by folder or by unfiled, move profiles in bulk, and
  delete a folder without ever losing a profile — its profiles move to
  another folder or to unfiled, and a profile restored from the Trash
  keeps its folder.
- Profile templates (backend and API): built-in presets for TikTok,
  YouTube, Facebook and a blank one, plus user templates that can be
  created, edited, duplicated and deleted. A template carries a site,
  start page, language, notes, tags, engine and folder, plus a proxy
  recommendation note — never a proxy, a secret or a cookie.
- Engine renamed: the default engine is now AnyBrowser, the project's own
  Chromium fork build (pro/chrome), replacing the upstream Apostate
  download. Existing profiles and templates are rebound automatically
  (migration 0021); seeds and personas do not change, so profiles keep
  their machines. The placeholder Google API key variables are no longer
  exported at launch — AnyBrowser builds are keyless. The first
  AnyBrowser build is not hosted yet, so engine installs stay unavailable
  until it is published.

## 0.1.0 (2026-10-24)

The first public release. Earlier builds numbered 0.1.0 and 0.1.1 were never
published; what they had is listed under them below and is all in this one.


- AnyProfile Pro: buy it by the month or the year in Settings (card, Apple
  Pay, Google Pay); the key activates by itself. Pro brings tested recipes and
  scripts (who is still signed in, posting to YouTube Studio and TikTok) and
  keeps them up to date, for AI agents or one command.
- Proxy status tells the truth: a proxy whose check fails shows "Not
  answering" instead of "Working", Check all says how many did not answer, and
  only proxies that answered are handed out as the next free one, so a dead
  proxy no longer blocks first-run setup.
- Mobile devices (beta): the Mac's Android runtime comes with Pro, from the
  license server, signed with the Pro key, its source a click away on the
  Mobile page (QEMU is GPL-2.0). Until it is out, the Mobile page says
  "Coming soon"; it arrives with no app update. While a device runs, its proxy is tried every two minutes and
  the Mobile page says when the device has no network.
- Smaller fixes: dates follow the app's language, proxy paste errors are
  translated, an error leaves with the screen it was about, a closed profile
  no longer warns that its DevTools port is open, trashing a mobile device
  offers Undo, and downloads say when they are checking and unpacking.
- Mobile devices, part of AnyProfile Pro: separate Android devices, each with
  its own apps, data and proxy. On a Mac they run as LineageOS (Android 16)
  virtual machines that the Mobile page downloads; on Windows, as LDPlayer 9
  instances. A device reaches the internet only through its proxy, DNS
  included; UDP, ping and IPv6 are blocked, and a dead proxy means no network,
  not your own IP. Timezone and language follow the proxy's country. Devices
  can be backed up, restored and trashed like profiles. Apps can still tell a
  device is a virtual machine or an emulator (docs/mobile.md).
- AI agents drive mobile devices over MCP: list, open and close them, read
  the screen, tap, swipe, type in any language, take screenshots, open apps
  and links, and push files or APKs from the uploads folder (docs/mcp.md).
- macOS downloads as a disk image: open it and drag AnyProfile onto
  Applications. The in-app updater keeps using the .zip. The release
  workflow can sign with a Developer ID and notarize the image, which lets
  macOS open it without "Apple could not verify"; it does so once the Apple
  secrets exist (docs/releasing.md).
- Installers are English only, the disk image included. The Windows
  installer names Cassau, LLC as publisher, offers to run AnyProfile when it
  finishes, and asks to close a running AnyProfile instead of failing to
  replace it.
- Easier to read and to start with: wherever a profile would use your real IP
  (first-run setup, New profile, the profile panel), Buy proxies is one click
  away. Settings has a list of sections, starts with everyday options and
  folds away the older Fury engine until a profile needs it. A row's Open
  button turns blue only on the row you point at, Close no longer looks like
  deleting, and "Real IP" shows in amber. Plainer words for sticky proxies
  and for a profile's device before its first open.
- Setup says how big the browser download is before it starts. "Add
  extensions in Settings" is a link to that section. The Platform column
  keeps its header in a narrow window, and the profile list loses its refresh
  button: it updates by itself.

### From the unpublished 0.1.1 (2026-10-18)

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

### From the unpublished 0.1.0 (2026-10-17)

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
