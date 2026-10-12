// SPDX-License-Identifier: AGPL-3.0-or-later
// Launch N profiles of one Chromium-family engine at once and break memory down by process type.
// Usage: ENGINE=fury|cloak|chrome node chromium-bench.mjs <binary> <profiles-dir> <count> [extra flags...]
// ENGINE=fury reads harness/fury-configs/seed<i>.json. URL defaults to https://example.com.
import { spawn, execSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const [bin, profDir, countArg, ...extra] = process.argv.slice(2);
const engine = process.env.ENGINE;
const count = Number(countArg);
const url = process.env.BENCH_URL || 'https://example.com';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const cfgDir = path.join(path.dirname(new URL(import.meta.url).pathname), 'fury-configs');

function launch(i) {
  const dir = path.join(profDir, `${engine}-p${i}`);
  fs.mkdirSync(dir, { recursive: true });
  fs.rmSync(path.join(dir, 'DevToolsActivePort'), { force: true });
  const args = [`--user-data-dir=${dir}`, '--remote-debugging-port=0', '--remote-allow-origins=*',
    '--no-first-run', '--no-default-browser-check', '--use-mock-keychain', '--lang=en-US',
    `--window-position=${-4000 - i * 50},-4000`, '--window-size=1280,800', ...extra];
  const stdio = ['ignore', 'ignore', 'ignore'];
  if (engine === 'fury') { args.push('--fury-fp-fd=3'); stdio.push(fs.openSync(path.join(cfgDir, `seed${i}.json`), 'r')); }
  if (engine === 'cloak' || engine === 'apostate') args.push(`--fingerprint=${10000 + i}`, `--fingerprint-platform=${process.env.PERSONA || 'macos'}`);
  args.push('about:blank');
  return { i, dir, t0: performance.now(), proc: spawn(bin, args, { stdio }) };
}

async function ready(p) {
  const f = path.join(p.dir, 'DevToolsActivePort');
  for (let k = 0; k < 400; k++) {
    if (fs.existsSync(f)) { p.port = fs.readFileSync(f, 'utf8').split('\n')[0].trim(); if (p.port) break; }
    await sleep(50);
  }
  if (!p.port) throw new Error(`profile ${p.i}: no CDP port`);
  p.readyMs = Math.round(performance.now() - p.t0);
}

async function navigate(p) {
  const page = (await (await fetch(`http://127.0.0.1:${p.port}/json/list`)).json()).find((t) => t.type === 'page');
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((r) => (ws.onopen = r));
  ws.send(JSON.stringify({ id: 1, method: 'Page.navigate', params: { url } }));
  await sleep(500);
  ws.close();
}

// Physical footprint per process, grouped by Chromium --type (browser = no --type).
function breakdown(rootPids) {
  const rows = execSync('ps -axo pid=,ppid=').toString().trim().split('\n').map((l) => l.trim().split(/\s+/).map(Number));
  const kids = new Map();
  for (const [pid, ppid] of rows) { if (!kids.has(ppid)) kids.set(ppid, []); kids.get(ppid).push(pid); }
  const all = []; const stack = [...rootPids];
  while (stack.length) { const p = stack.pop(); all.push(p); stack.push(...(kids.get(p) || [])); }
  const groups = {};
  for (const pid of all) {
    let cmd = '';
    try { cmd = execSync(`ps -o command= -p ${pid}`).toString(); } catch { continue; }
    const type = (cmd.match(/--type=([a-z-]+)/) || [, 'browser'])[1];
    const sub = (cmd.match(/--utility-sub-type=([\w.]+)/) || [, ''])[1].split('.').pop();
    const key = sub ? `${type}:${sub}` : type;
    let mb = 0;
    try {
      const out = execSync(`footprint ${pid} 2>/dev/null`).toString();
      const m = out.match(/phys_footprint:\s+([\d.]+)\s+(KB|MB|GB)/);
      if (m) mb = Number(m[1]) * { KB: 1 / 1024, MB: 1, GB: 1024 }[m[2]];
    } catch { /* process exited */ }
    groups[key] = groups[key] || { procs: 0, mb: 0 };
    groups[key].procs++; groups[key].mb += mb;
  }
  return groups;
}

const profiles = Array.from({ length: count }, (_, k) => launch(k + 1));
try {
  await Promise.all(profiles.map(ready));
  await Promise.all(profiles.map(navigate));
  await sleep(10000); // let pages settle
  const g = breakdown(profiles.map((p) => p.proc.pid));
  const total = Object.values(g).reduce((s, x) => s + x.mb, 0);
  console.log(JSON.stringify({ engine, count, url, readyMs: profiles.map((p) => p.readyMs), totalMb: Math.round(total), perProfileMb: Math.round(total / count) }));
  for (const [k, v] of Object.entries(g).sort((a, b) => b[1].mb - a[1].mb)) {
    console.log(`  ${k.padEnd(28)} procs=${String(v.procs).padStart(3)}  ${Math.round(v.mb)} MB`);
  }
} finally {
  for (const p of profiles) p.proc.kill('SIGTERM');
  await sleep(2500);
}
