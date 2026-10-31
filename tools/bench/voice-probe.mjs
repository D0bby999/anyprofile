// SPDX-License-Identifier: AGPL-3.0-or-later
// Headless diagnostic for the composed speech voices: does getVoices() list
// them, and does speak() fire start/end instead of not-allowed? A page cannot
// hear the audio, so this measures the observable events, not audibility. It is
// NOT CH-24 acceptance: that needs a native headed run on a secure origin.
//
//   ANYBROWSER_BINARY=<Chromium> SEED=<16 hex> node tools/bench/voice-probe.mjs
//   VOICE_LANGS=en-US,fr-FR,vi-VN  (default)
//
// Runtime.evaluate carries userGesture:true because Chrome's speech synthesis
// is under the autoplay policy: without activation speak() fails not-allowed
// even on a real machine, which says nothing about the engine.
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const SPEAK_TIMEOUT = 8000;

// speechVerdict turns one lang's observed events into a status. A start and an
// end, in order and within the timeout, is the only pass; not-allowed means the
// page had no activation, which is a harness problem, not a voice result.
export function speechVerdict(result) {
  if (!result || result.missing) return { status: 'BLOCKED', reason: 'voice not listed' };
  if (result.error === 'not-allowed') return { status: 'BLOCKED', reason: 'speak needs user activation' };
  if (result.error === 'timeout') return { status: 'FAIL', reason: 'no end event within the timeout' };
  if (result.error) return { status: 'FAIL', reason: `speech error ${result.error}` };
  const events = result.events ?? [];
  const start = events.find(event => event[0] === 'start');
  const end = events.find(event => event[0] === 'end');
  if (!start || !end) return { status: 'FAIL', reason: 'missing start or end event' };
  const duration = end[1] - start[1];
  if (duration < 200) return { status: 'FAIL', reason: `end after ${duration} ms is too fast to be a voice` };
  return { status: 'PASS', startMs: start[1], endMs: end[1], durationMs: duration };
}

export function summarise(probe) {
  const results = (probe?.results ?? []).map(result => ({ lang: result.lang, name: result.name, ...speechVerdict(result) }));
  const failed = results.filter(result => result.status !== 'PASS');
  return {
    voices: probe?.count ?? 0,
    localVoices: probe?.localCount ?? 0,
    results,
    verdict: results.length === 0 ? 'INCONCLUSIVE' : failed.length ? 'FAIL' : 'PASS',
    note: `${process.env.ANYBROWSER_HEADED === '1' ? 'Native headed' : 'Headless'} diagnostic of start/end events only (${process.env.VOICE_URL || 'about:blank'}). CH-24 still needs audible verification.`,
  };
}

function parseArgs(args) {
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    if (!args[i].startsWith('--') || args[i + 1] === undefined) throw new Error('Expected --key value');
    options[args[i].slice(2)] = args[i + 1];
  }
  return options;
}

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

async function probe({ binary, seed, langs }) {
  const dir = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'anyprofile-voice-'));
  // The browser sandbox stays on for a normal run. Agent sandboxes that block
  // seatbelt need ANYBROWSER_NO_SANDBOX=1; that is a diagnostic-only escape.
  const unsandboxed = process.env.ANYBROWSER_NO_SANDBOX === '1' ? ['--no-sandbox', '--disable-gpu-sandbox'] : [];
  const window = process.env.ANYBROWSER_HEADED === '1' ? ['--window-position=-4000,-4000', '--window-size=420,320'] : ['--headless=new'];
  const argv = [`--user-data-dir=${dir}`, ...window, ...unsandboxed,
    '--disable-crash-reporter', '--disable-breakpad', '--hide-crash-restore-bubble',
    '--remote-debugging-port=0', '--no-first-run', '--no-default-browser-check', '--use-mock-keychain',
    `--fingerprint=${seed}`, '--fingerprint-platform=macos', '--fingerprint-locale=en-US,en',
    '--lang=en-US', 'about:blank'];
  const child = spawn(binary, argv, { stdio: ['ignore', 'ignore', 'ignore'] });
  let ws;
  try {
    for (let i = 0; i < 150 && !fs.existsSync(path.join(dir, 'DevToolsActivePort')); i++) await sleep(100);
    const port = fs.readFileSync(path.join(dir, 'DevToolsActivePort'), 'utf8').split('\n')[0];
    const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find(target => target.type === 'page');
    ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = () => reject(new Error('DevTools socket failed')); });
    let id = 0; const pending = new Map();
    ws.onmessage = event => { const message = JSON.parse(event.data); if (message.id && pending.has(message.id)) { pending.get(message.id)(message); pending.delete(message.id); } };
    const send = (method, params = {}, ms = 30000) => new Promise(resolve => { const n = ++id; pending.set(n, resolve); ws.send(JSON.stringify({ id: n, method, params })); setTimeout(() => resolve({ timeout: true }), ms); });
    const expression = `(async () => {
      const readVoices = async () => { for (let i = 0; i < 30; i++) { const list = speechSynthesis.getVoices(); if (list.length) return list; await new Promise(r => setTimeout(r, 100)); } return speechSynthesis.getVoices(); };
      const voices = await readVoices();
      const speakOne = voice => new Promise(resolve => {
        const started = performance.now(); const events = [];
        const utterance = new SpeechSynthesisUtterance('The quick brown fox jumps over the lazy dog.');
        utterance.voice = voice; utterance.lang = voice.lang; utterance.rate = 1;
        utterance.onstart = () => events.push(['start', Math.round(performance.now() - started)]);
        utterance.onend = () => { events.push(['end', Math.round(performance.now() - started)]); resolve({ events, error: null }); };
        utterance.onerror = e => { events.push(['error', Math.round(performance.now() - started), e.error]); resolve({ events, error: e.error || 'error' }); };
        speechSynthesis.speak(utterance);
        setTimeout(() => resolve({ events, error: 'timeout' }), ${SPEAK_TIMEOUT});
      });
      const out = { count: voices.length, localCount: voices.filter(v => v.localService).length,
        langs: [...new Set(voices.map(v => v.lang))].slice(0, 16), results: [] };
      for (const lang of ${JSON.stringify(langs)}) {
        const voice = voices.find(v => v.lang === lang);
        if (!voice) { out.results.push({ lang, missing: true }); continue; }
        out.results.push({ lang, name: voice.name, local: voice.localService, ...await speakOne(voice) });
      }
      return out;
    })()`;
    const reply = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true, userGesture: true }, 45000);
    if (!reply.result?.result?.value) throw new Error('probe returned nothing (timed out or crashed)');
    return reply.result.result.value;
  } finally {
    try { ws?.close(); } catch { /* already closed */ }
    try { child.kill('SIGTERM'); } catch { /* already gone */ }
    await sleep(1500);
    try { child.kill('SIGKILL'); } catch { /* already gone */ }
    await fs.promises.rm(dir, { recursive: true, force: true });
  }
}

async function run(args) {
  const opts = parseArgs(args);
  const binary = opts.binary || process.env.ANYBROWSER_BINARY;
  const seed = opts.seed || process.env.SEED;
  if (!binary || !/^[0-9a-f]{16}$/.test(seed ?? '')) throw new Error('Pass --binary and --seed (16 hex), or ANYBROWSER_BINARY and SEED');
  const langs = (opts.langs || process.env.VOICE_LANGS || 'en-US,fr-FR,vi-VN').split(',').filter(Boolean);
  const raw = await probe({ binary, seed, langs });
  const result = summarise(raw);
  if (opts.out) fs.writeFileSync(path.resolve(opts.out), JSON.stringify({ raw, result }, null, 2), { mode: 0o600 });
  console.log(JSON.stringify(result, null, 2));
  if (result.verdict !== 'PASS') process.exitCode = 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  run(process.argv.slice(2)).catch(error => { console.error(error.message); process.exitCode = 1; });
}
