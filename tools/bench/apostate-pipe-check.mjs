// SPDX-License-Identifier: AGPL-3.0-or-later
// Runs BrowserScan in Apostate the way AnyProfile launches it: proxy through a
// local relay, the DevTools protocol on a pipe (fds 3 and 4, no TCP port), the
// three GOOGLE_* variables, timezone and locale from the exit IP.
//
// Usage: UPSTREAM=http://user:pass@host:port node apostate-pipe-check.mjs <chromium-binary> <relay.mjs> [seeds=5] [firstSeed=2000]
// The upstream port N becomes N+i for seed i (sticky ports). One JSON line per run on stdout.
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const [bin, relayScript, nSeeds = '5', firstSeed = '2000'] = process.argv.slice(2);
if (!bin || !relayScript || !process.env.UPSTREAM) {
  console.error('usage: UPSTREAM=... node apostate-pipe-check.mjs <chromium-binary> <relay.mjs> [seeds] [firstSeed]');
  process.exit(2);
}
const { UPSTREAM: _secret, ...baseEnv } = process.env; // the browser never sees the credential
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function parse(txt) {
  const lines = txt.split('\n');
  const i = lines.findIndex((l) => /authenticity: \d+%/.test(l));
  if (i < 0) return { score: null, deductions: [] };
  const score = Number(lines[i].match(/(\d+)%/)[1]);
  const deductions = [];
  for (let k = i + 1; k < lines.length && !/^What (is|websites)/.test(lines[k]); k++) {
    if (/^-\d+%$/.test(lines[k])) deductions.push(`${lines[k - 1]} ${lines[k]}`);
  }
  return { score, deductions };
}

// Minimal DevTools client over the pipe pair: NUL-terminated JSON.
function pipeClient(child) {
  const w = child.stdio[3], r = child.stdio[4];
  let next = 0, buf = Buffer.alloc(0);
  const wait = new Map();
  r.on('data', (chunk) => {
    buf = Buffer.concat([buf, chunk]);
    let z;
    while ((z = buf.indexOf(0)) >= 0) {
      const msg = JSON.parse(buf.subarray(0, z).toString('utf8'));
      buf = buf.subarray(z + 1);
      if (msg.id && wait.has(msg.id)) { wait.get(msg.id)(msg); wait.delete(msg.id); }
    }
  });
  return (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const id = ++next;
    const t = setTimeout(() => { wait.delete(id); reject(new Error(`${method}: timeout`)); }, 30000);
    wait.set(id, (m) => { clearTimeout(t); m.error ? reject(new Error(`${method}: ${JSON.stringify(m.error)}`)) : resolve(m.result); });
    w.write(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }) + '\0');
  });
}

for (let i = 0; i < Number(nSeeds); i++) {
  const seed = (Number(firstSeed) + i).toString(16).padStart(16, '0');
  const up = new URL(process.env.UPSTREAM);
  up.port = String(Number(up.port) + i);
  const port = 18300 + i;
  const relay = spawn('node', [relayScript, String(port)], { env: { ...baseEnv, UPSTREAM: up.href }, stdio: 'ignore' });
  const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-pipe-'));
  const t0 = Date.now();
  let row = { seed, ready: false };
  let child;
  try {
    await sleep(1500);
    const geo = JSON.parse(execFileSync('curl', ['-s', '--max-time', '20', '-x', `http://127.0.0.1:${port}`, 'https://ipinfo.io/json'], { encoding: 'utf8' }));
    row = { ...row, ip: geo.ip, country: geo.country, tz: geo.timezone };
    child = spawn(bin, [
      `--user-data-dir=${udd}`, '--no-first-run', '--no-default-browser-check', '--use-mock-keychain',
      `--proxy-server=http://127.0.0.1:${port}`, '--proxy-bypass-list=<-loopback>', '--disable-quic',
      '--force-webrtc-ip-handling-policy=disable_non_proxied_udp', '--fingerprint-webrtc-udp=block',
      `--fingerprint=${seed}`, '--fingerprint-platform=macos', `--fingerprint-timezone=${geo.timezone}`,
      '--fingerprint-locale=en-US,en', '--remote-debugging-pipe', 'about:blank',
    ], {
      env: { ...baseEnv, TZ: geo.timezone, GOOGLE_API_KEY: 'no', GOOGLE_DEFAULT_CLIENT_ID: 'no', GOOGLE_DEFAULT_CLIENT_SECRET: 'no' },
      stdio: ['ignore', 'ignore', 'ignore', 'pipe', 'pipe'],
    });
    const call = pipeClient(child);
    await call('Browser.getVersion');
    let page;
    for (let k = 0; k < 50 && !page; k++) {
      page = (await call('Target.getTargets')).targetInfos.find((t) => t.type === 'page');
      if (!page) await sleep(200);
    }
    const { sessionId } = await call('Target.attachToTarget', { targetId: page.targetId, flatten: true });
    await call('Page.navigate', { url: 'https://www.browserscan.net/' }, sessionId);
    let txt = '';
    for (let k = 0; k < 45; k++) { // up to 90 s for the score to appear
      await sleep(2000);
      txt = (await call('Runtime.evaluate', { expression: 'document.body ? document.body.innerText : ""', returnByValue: true }, sessionId)).result.value || '';
      if (/authenticity: \d+%/.test(txt) && /Browser:\n(Chrome|Firefox)/.test(txt)) break;
    }
    const chrome = (await call('Runtime.evaluate', { expression: 'outerHeight - innerHeight', returnByValue: true }, sessionId)).result.value;
    row = { ...row, ready: /Browser:\n(Chrome|Firefox)/.test(txt), ...parse(txt), chromeHeight: chrome };
  } catch (e) {
    row.error = String(e.message || e).slice(0, 200);
  } finally {
    if (child) child.kill('SIGTERM');
    relay.kill();
    await sleep(1500);
    fs.rmSync(udd, { recursive: true, force: true });
  }
  console.log(JSON.stringify({ ...row, secs: Math.round((Date.now() - t0) / 1000) }));
}
