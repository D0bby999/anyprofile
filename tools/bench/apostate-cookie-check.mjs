// SPDX-License-Identifier: AGPL-3.0-or-later
// Checks that Apostate keeps a persistent cookie across a restart when it uses
// the real macOS Keychain ("Chromium Safe Storage"), and how long each start
// takes. A Keychain dialog blocks the browser, which shows up as a timeout.
//
// Usage: node apostate-cookie-check.mjs <chromium-binary> [mock]
//   mock: add --use-mock-keychain (self-test of this script, never the real check)
// Prints one JSON line per launch, then a verdict line.
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const [bin, mode] = process.argv.slice(2);
if (!bin) { console.error('usage: node apostate-cookie-check.mjs <chromium-binary> [mock]'); process.exit(2); }
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-cookie-'));

function launch() {
  const args = [`--user-data-dir=${udd}`, '--no-first-run', '--no-default-browser-check',
    '--fingerprint=00000000000a11ce', '--fingerprint-platform=macos', '--remote-debugging-pipe', 'about:blank'];
  if (mode === 'mock') args.unshift('--use-mock-keychain');
  const child = spawn(bin, args, {
    env: { ...process.env, GOOGLE_API_KEY: 'no', GOOGLE_DEFAULT_CLIENT_ID: 'no', GOOGLE_DEFAULT_CLIENT_SECRET: 'no' },
    stdio: ['ignore', 'ignore', 'ignore', 'pipe', 'pipe'],
  });
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
  const call = (method, params = {}, ms = 60000) => new Promise((resolve, reject) => {
    const id = ++next;
    const t = setTimeout(() => { wait.delete(id); reject(new Error(`${method}: no reply in ${ms / 1000} s (Keychain dialog?)`)); }, ms);
    wait.set(id, (m) => { clearTimeout(t); m.error ? reject(new Error(`${method}: ${JSON.stringify(m.error)}`)) : resolve(m.result); });
    w.write(JSON.stringify({ id, method, params }) + '\0');
  });
  const exited = new Promise((r2) => child.on('exit', r2));
  return { child, call, exited };
}

const expires = Math.floor(Date.now() / 1000) + 86400;
let ok = false;
try {
  // Launch 1: write a persistent cookie, then close gracefully so it is flushed.
  let t0 = Date.now();
  let b = launch();
  await b.call('Browser.getVersion');
  await b.call('Storage.setCookies', { cookies: [{ name: 'ap_check', value: 'kept', domain: 'example.com', path: '/', expires, secure: true }] });
  const set = (await b.call('Storage.getCookies')).cookies.some((c) => c.name === 'ap_check');
  await sleep(1500);
  await b.call('Browser.close').catch(() => {});
  await Promise.race([b.exited, sleep(10000)]);
  console.log(JSON.stringify({ launch: 1, cookieSet: set, secs: Math.round((Date.now() - t0) / 1000) }));

  // Launch 2: the cookie must still be there, so it was encrypted and decrypted with the same key.
  t0 = Date.now();
  b = launch();
  await b.call('Browser.getVersion');
  const kept = (await b.call('Storage.getCookies')).cookies.find((c) => c.name === 'ap_check');
  ok = !!kept && kept.value === 'kept';
  await b.call('Browser.close').catch(() => {});
  await Promise.race([b.exited, sleep(10000)]);
  console.log(JSON.stringify({ launch: 2, cookieKept: ok, secs: Math.round((Date.now() - t0) / 1000) }));
} catch (e) {
  console.log(JSON.stringify({ error: String(e.message || e).slice(0, 200) }));
} finally {
  fs.rmSync(udd, { recursive: true, force: true });
}
console.log(ok ? 'VERDICT pass' : 'VERDICT fail');
