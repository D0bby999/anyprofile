// SPDX-License-Identifier: AGPL-3.0-or-later
// Open one Chromium-family browser, visit fingerprint test sites, print the verdict lines.
// Usage: node detect-sites.mjs <binary> <profile-dir> [fury-config.json]
import { spawn } from 'node:child_process';
import fs from 'node:fs';

const [bin, dir, cfg, ...extraArgs] = process.argv.slice(2);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
if (!process.env.CDP_PORT) {
  fs.mkdirSync(dir, { recursive: true });
  fs.rmSync(`${dir}/DevToolsActivePort`, { force: true });
}

const args = [`--user-data-dir=${dir}`, '--remote-debugging-port=0', '--remote-allow-origins=*',
  '--no-first-run', '--no-default-browser-check', '--use-mock-keychain', '--lang=en-US',
  '--window-position=-4000,-4000', '--window-size=1280,900', 'about:blank'];
const stdio = ['ignore', 'ignore', 'ignore'];
if (cfg && cfg !== '-') { args.push('--fury-fp-fd=3'); stdio.push(fs.openSync(cfg, 'r')); }
args.splice(args.length - 1, 0, ...extraArgs);
// CDP_PORT attaches to a browser something else launched (e.g. AnyProfile's launcher).
const proc = process.env.CDP_PORT ? null : spawn(bin, args, { stdio });
if (proc) for (let k = 0; k < 150 && !fs.existsSync(`${dir}/DevToolsActivePort`); k++) await sleep(100);
const port = process.env.CDP_PORT || fs.readFileSync(`${dir}/DevToolsActivePort`, 'utf8').split('\n')[0];
const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find((t) => t.type === 'page');
const ws = new WebSocket(page.webSocketDebuggerUrl);
await new Promise((r) => (ws.onopen = r));
let id = 0; const pending = new Map();
ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); } };
const send = (method, params = {}) => new Promise((r) => { const n = ++id; pending.set(n, r); ws.send(JSON.stringify({ id: n, method, params })); setTimeout(() => r({ timeout: true }), 30000); });
const text = async () => (await send('Runtime.evaluate', { expression: 'document.body ? document.body.innerText : ""', returnByValue: true })).result?.result?.value || '';

const sites = [
  // [name, url, wait ms, regex of lines worth keeping]
  // [name, url, ready regex, regex of lines worth keeping]
  ['ipinfo', 'https://ipinfo.io/json', /"ip"/, /./],
  ['creepjs', 'https://abrahamjuliot.github.io/creepjs/', /% like headless[\s\S]*% stealth/, /(lies|trash|errors|headless|stealth|like headless|bot|crowd-blending|FP ID|Fuzzy)/i],
  ['browserscan', 'https://www.browserscan.net/', /Browser:\n(Chrome|Firefox)/, /(authenticity|bot|robot|Detect|fingerprint|%|WebDriver|CDP)/i],
  ['dbi', 'https://deviceandbrowserinfo.com/are_you_a_bot', /You are (human|a bot)!/, /(bot|human|suspicious|inconsistent|true|false)/i],
  ['pixelscan', 'https://pixelscan.net/fingerprint-check', /Your Browser Fingerprint is (consistent|inconsistent)/, /(consistent|masking|automation|detected|spoof|fingerprint)/i],
].filter(([n]) => !process.env.SITES || process.env.SITES.split(',').includes(n));
try {
  for (const [name, url, readyRe, keep] of sites) {
    await send('Page.navigate', { url });
    let ok = false;
    for (let k = 0; k < 50 && !ok; k++) { await sleep(3000); ok = readyRe.test(await text()); }
    await sleep(8000); // let late checks finish scoring
    console.log(`=== ${name} ready=${ok}`);
    const all = (await text()).split('\n').map((l) => l.trim()).filter(Boolean);
    if (process.env.DUMP) fs.writeFileSync(`${process.env.DUMP}-${name}.txt`, all.join("\n"));
    if (process.env.CONTEXT) {
      all.forEach((l, i) => { if (new RegExp(process.env.CONTEXT).test(l)) console.log('>> ' + all.slice(Math.max(0, i - 3), i + 2).join(' | ').slice(0, 300)); });
    } else {
      const lines = all.filter((l) => l.length < 140 && keep.test(l));
      console.log([...new Set(lines)].slice(0, 30).join('\n'));
    }
  }
} finally {
  ws.close(); if (proc) { proc.kill('SIGTERM'); await sleep(1500); }
}
