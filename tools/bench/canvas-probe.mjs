// SPDX-License-Identifier: AGPL-3.0-or-later
// Headless feasibility check for the profile-seeded canvas readback noise:
// two seeds must collide without the switch and differ with it, and the same
// profile must keep its canvas when reopened. Also reports how much of the
// bitmap the noise moves, so a large or alpha-changing perturbation is visible
// before any detector run.
//
//   ANYBROWSER_NO_SANDBOX=1 node tools/bench/canvas-probe.mjs \
//     --binary <Chromium> --seed-a <16hex> --seed-b <16hex> [--out result.json]
//
// The browser sandbox stays on unless ANYBROWSER_NO_SANDBOX=1 (agent sandboxes
// that block seatbelt). Headless uses SwiftShader, so this proves the mechanism,
// not the native GPU rendering; detector deltas still need a headed run.
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const SCENE = `(() => {
  const c = document.createElement('canvas'); c.width = 220; c.height = 60;
  const x = c.getContext('2d', { willReadFrequently: true });
  x.textBaseline = 'top'; x.font = '16px Arial'; x.fillStyle = '#f60'; x.fillRect(10, 10, 80, 20);
  x.fillStyle = '#069'; x.fillText('AnyBrowser probe', 10, 35);
  const d = x.getImageData(0, 0, 220, 60).data;
  let s = ''; const chunk = 0x8000;
  for (let i = 0; i < d.length; i += chunk) s += String.fromCharCode.apply(null, d.subarray(i, i + chunk));
  const u = c.toDataURL(); let h = 2166136261;
  for (let i = 0; i < u.length; i++) { h ^= u.charCodeAt(i); h = Math.imul(h, 16777619) >>> 0; }
  return { canvas: h.toString(16), length: u.length, pixels: btoa(s) };
})()`;

export function canvasVerdict({ plainA, plainB, plainReopen, noiseA, noiseB, noiseReopen }) {
  const plainCollision = plainA === plainB;
  const plainStable = plainA === plainReopen;
  const noiseDistinct = noiseA !== noiseB;
  const noiseStable = noiseA === noiseReopen;
  return {
    plainCollision, plainStable, noiseDistinct, noiseStable,
    pass: plainCollision && plainStable && noiseDistinct && noiseStable,
  };
}

// pixelStats keeps the magnitude honest: a change that touches alpha, or moves
// a channel by more than one, is not the subtle ±1 readback the patch claims.
export function pixelStats(plain, noisy) {
  if (plain.length !== noisy.length || plain.length % 4) throw new Error('pixel buffers must be RGBA and the same size');
  let changed = 0, max = 0, sum = 0, alpha = 0;
  for (let p = 0; p < plain.length; p += 4) {
    let pixelChanged = false;
    for (let c = 0; c < 4; c++) {
      const delta = Math.abs(plain[p + c] - noisy[p + c]);
      if (!delta) continue;
      pixelChanged = true;
      sum += delta;
      if (delta > max) max = delta;
      if (c === 3) alpha++;
    }
    if (pixelChanged) changed++;
  }
  const pixels = plain.length / 4;
  return {
    pixels, changedPixels: changed, changedPct: +(100 * changed / pixels).toFixed(2),
    maxChannelDelta: max, meanDeltaPerChangedPixel: +(sum / Math.max(1, changed)).toFixed(3),
    alphaChanges: alpha,
  };
}

async function launch({ binary, seed, noise, dir }) {
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  fs.rmSync(path.join(dir, 'DevToolsActivePort'), { force: true });
  const unsandboxed = process.env.ANYBROWSER_NO_SANDBOX === '1' ? ['--no-sandbox', '--disable-gpu-sandbox'] : [];
  const argv = [`--user-data-dir=${dir}`, '--headless=new', ...unsandboxed,
    '--disable-crash-reporter', '--disable-breakpad', '--hide-crash-restore-bubble',
    '--remote-debugging-port=0', '--no-first-run', '--no-default-browser-check', '--use-mock-keychain',
    `--fingerprint=${seed}`, '--fingerprint-platform=macos', '--fingerprint-locale=en-US,en', 'about:blank'];
  if (noise) argv.splice(1, 0, '--fingerprint-noise');
  const child = spawn(binary, argv, { stdio: ['ignore', 'ignore', 'ignore'] });
  try {
    for (let i = 0; i < 200 && !fs.existsSync(path.join(dir, 'DevToolsActivePort')); i++) await new Promise(r => setTimeout(r, 100));
    const port = fs.readFileSync(path.join(dir, 'DevToolsActivePort'), 'utf8').split('\n')[0];
    const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find(target => target.type === 'page');
    const ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = () => reject(new Error('DevTools socket failed')); });
    let id = 0; const pending = new Map();
    ws.onmessage = event => { const message = JSON.parse(event.data); if (message.id && pending.has(message.id)) { pending.get(message.id)(message); pending.delete(message.id); } };
    const send = (method, params = {}, ms = 25000) => new Promise(resolve => { const n = ++id; pending.set(n, resolve); ws.send(JSON.stringify({ id: n, method, params })); setTimeout(() => resolve({ timeout: true }), ms); });
    const reply = await send('Runtime.evaluate', { expression: SCENE, returnByValue: true });
    if (!reply.result?.result?.value) throw new Error('canvas probe returned nothing');
    ws.close();
    return reply.result.result.value;
  } finally {
    try { child.kill('SIGTERM'); } catch { /* gone */ }
    await new Promise(r => setTimeout(r, 800));
    try { child.kill('SIGKILL'); } catch { /* gone */ }
  }
}

function parseArgs(args) {
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    if (!args[i].startsWith('--') || args[i + 1] === undefined) throw new Error('Expected --key value');
    options[args[i].slice(2)] = args[i + 1];
  }
  return options;
}

async function run(args) {
  const opts = parseArgs(args);
  const binary = opts.binary || process.env.ANYBROWSER_BINARY;
  const seedA = opts['seed-a'] || process.env.SEED_A;
  const seedB = opts['seed-b'] || process.env.SEED_B;
  if (!binary || !/^[0-9a-f]{16}$/.test(seedA ?? '') || !/^[0-9a-f]{16}$/.test(seedB ?? '')) {
    throw new Error('Pass --binary, --seed-a and --seed-b (16 hex), or ANYBROWSER_BINARY/SEED_A/SEED_B');
  }
  const base = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'anyprofile-canvas-'));
  try {
    const at = name => path.join(base, name);
    const run = (seed, noise, dir) => launch({ binary, seed, noise, dir });
    const plainA = await run(seedA, false, at('plain-a'));
    const plainB = await run(seedB, false, at('plain-b'));
    const plainReopen = await run(seedA, false, at('plain-a'));
    const noiseA = await run(seedA, true, at('noise-a'));
    const noiseB = await run(seedB, true, at('noise-b'));
    const noiseReopen = await run(seedA, true, at('noise-a'));
    const verdict = canvasVerdict({
      plainA: plainA.canvas, plainB: plainB.canvas, plainReopen: plainReopen.canvas,
      noiseA: noiseA.canvas, noiseB: noiseB.canvas, noiseReopen: noiseReopen.canvas,
    });
    const stats = pixelStats(
      Buffer.from(plainA.pixels, 'base64'), Buffer.from(noiseA.pixels, 'base64'));
    const result = {
      plain: { a: plainA.canvas, b: plainB.canvas, aReopen: plainReopen.canvas },
      noise: { a: noiseA.canvas, b: noiseB.canvas, aReopen: noiseReopen.canvas },
      verdict, pixelStats: stats,
      note: 'Headless SwiftShader: mechanism only. Detector deltas need a native headed run.',
    };
    if (opts.out) fs.writeFileSync(path.resolve(opts.out), JSON.stringify(result, null, 2), { mode: 0o600 });
    console.log(JSON.stringify(result, null, 2));
    if (!verdict.pass) process.exitCode = 1;
  } finally {
    await fs.promises.rm(base, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  run(process.argv.slice(2)).catch(error => { console.error(error.message); process.exitCode = 1; });
}
