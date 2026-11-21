# AnyMobile Metal: open latency and scrolling on Apple Silicon

## What changed

Host stack sampling during Android boot showed guest vCPU threads in `CompositeDisk::flush` → `sync_data` → `fcntl` on macOS. A single guest flush synchronized **every writable partition**, and ordinary disk requests were served on the vCPU that rang the virtio doorbell. The GPU readback thread in one sampled interval averaged about 0.6 ms per frame; that is not a reason to disable hardware acceleration.

The engine candidate keeps the first barrier for pre-existing writes, marks each partition dirty before writing, honors guest flushes and clears the mark **only after successful sync**. A bounded kick channel feeds one disk worker in request order; a kick already pending drains newly available requests. Completed chains may share a queue interrupt. The worker participates in the VM pause barrier and is stopped/joined on reset before an old ring can be reused. The Go launcher also keeps a responsive ADB connection while Android finishes booting. Device formats, Google Play, CPU, memory, fidelity and refresh rate remain unchanged.

Regression checks cover sync failure and retry, writes held by a full disk, reset during pause, and slow ordinary writes crossing a checkpoint. The live single-device test writes and syncs a marker, stops the VM and reads it after reboot.

## Reproduce

Use the commands in [mobile-maintenance.md](mobile-maintenance.md#đo-metal-và-mumuplayer-trên-cùng-mac). Build the engine from the **separate `pro` checkout**, sign the test VMM with `anyvmm/hvf/hypervisor.entitlements`, and use a private APFS clone of the staged runtime with `ANYPROFILE_AVO_DIR`. Never replace the installed VMM just to benchmark it. For MuMu, use a separate ADB server port. Neither harness needs the user's Android profiles or accounts.

Mac M2, 16 GB. MuMuPlayer Pro 1.10.3 versus builds of the VMM before and after the disk changes (`pro` baseline `541b55d`), identical Android image for the two AnyMobile builds. Three vCPUs, 4 GB per device, Phone/portrait. Interactive latency ends when Settings has opened, HOME input has been sent and the launcher is resumed; it excludes UI, proxy resolution, initial Android download and Google Play preparation. Run serially on the same host, leave guest 1 alive when creating guest 2. For the matched scrolling test: 780×1688, 320 dpi, alternate five 350 ms swipes in Settings, collect `dumpsys gfxinfo`. These are **Android app frame metrics**, not physical input-to-photon delay. Different Android distributions may count missed deadlines differently; compare latency percentiles as well as jank.

## Results (local candidate, not released)

Median of five matched trials per product, 16 GB Apple M2. Each trial created two new guests with Google Play enabled on Metal; guest 1 remained running during the second creation. Default MuMu guest contents are vendor-specific. Both displays reported ~60 Hz where queried. These are per-run medians, not pooled frame counts.

| Scenario | Metal candidate | MuMuPlayer Pro |
|---|---:|---:|
| Guest 1: create to interactive | 12.12 s | 15.42 s |
| Guest 1: Settings jank; p95 frame time | 0%; 7 ms | 0%; 17 ms |
| Guest 2: create to interactive | 17.35 s | 13.53 s |
| Guest 2: Settings jank; p95 frame time | 5.63%; 30 ms | 0%; 18 ms |

Guest 2 ranged from 15.09–21.13 s and 0.58–11.43% jank for Metal, versus 11.83–16.80 s and 0% reported jank for MuMu. A matched reference VMM run before the disk changes took 26.87 s for guest 2, with 26.36% jank. In a separate ABBA run with other host activity and one-direction swipes, guest 2 took 28.31/30.05 s on reference builds vs 20.26/20.50 s on candidates; the candidate still had isolated long frames. A single fast run is not parity. Guest marker verification also passed after a full stop/reopen. MuMu's test-only density override is applied before the swipes, not included in its measured launch path.

A separate longer run (`MOBILE_BENCH_SUSTAINED=1` alongside the two-instance setting) issued 100 alternating swipes with guest 1 still running. On guest 2, Metal reported **91/2,881 missed frames (3.16%), p95 20 ms**; MuMu reported **0/3,507, p95 19 ms**. This is one short sustained workload, not a gaming or soak test.

## What did not work

- Forcing the Vulkan HWUI renderer at boot repeatedly aborted Android system startup on this image. Kept the working renderer and its checks.
- Uncached userdata I/O gave 46% jank on one second-device scroll; not enabled.
- Constraining guest background CPU sets varied from 2.7% to 37.4% jank on the second guest; not enabled. No changes to other apps' priority and no background guest freezing to improve scores.

## Release threshold

Do not generalize from one host, Settings, or a handful of launches. Before a performance claim, repeat on a dedicated host (record memory pressure and resident devices), at least five paired cold/warm launches, sustained two-device workloads and four devices on a machine with enough RAM. Include a WebView feed, media playback, app switching, disk-full recovery and long-run reliability. Compare the packaged app and signed runtime that customers would actually install, not only a local driver test.
