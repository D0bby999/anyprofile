// SPDX-License-Identifier: AGPL-3.0-or-later
// Owned headed font diagnostics only; no user profiles, proxy or Runtime.enable in minimal mode.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { mkdir, mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { once } from 'node:events';

export function fingerprintObservation(entry) {
  const s = entry?.signals;
  if (typeof s?.tampering !== 'boolean' || typeof s?.tampering_details?.anti_detect_browser !== 'boolean') {
    return { status: 'INCONCLUSIVE', reason: 'Missing or non-boolean extracted server signal' };
  }
  return { status: s.tampering || s.tampering_details.anti_detect_browser ? 'FAIL' : 'PASS',
    tampering: s.tampering, antiDetectBrowser: s.tampering_details.anti_detect_browser };
}

// Strictly parse the server-response section, never the demo's marketing scores.
// Time bounds must come from the owned navigation ledger, not the response itself.
export function fingerprintServerObservation(text, { startedAt, finishedAt, expectedEventId } = {}) {
  const inconclusive = reason => ({ status: 'INCONCLUSIVE', reason });
  if (typeof text !== 'string') return inconclusive('Missing server response text');
  const sections = text.split(/SERVER API RESPONSE/i);
  if (sections.length !== 2) return inconclusive('Missing or ambiguous server response section');
  const section = sections[1].trimStart(), start = 0;
  if (!section.startsWith('{')) return inconclusive('Missing server JSON object');
  let depth = 0, quoted = false, escaped = false, end = -1;
  for (let i = start; i < section.length; i++) {
    const c = section[i];
    if (quoted) {
      if (escaped) escaped = false;
      else if (c === '\\') escaped = true;
      else if (c === '"') quoted = false;
    } else if (c === '"') quoted = true;
    else if (c === '{') depth++;
    else if (c === '}' && --depth === 0) { end = i + 1; break; }
  }
  let data;
  try { data = JSON.parse(section.slice(start, end < 0 ? section.length : end)); }
  catch { return inconclusive('Malformed server JSON'); }
  if (typeof data.event_id !== 'string' || !data.event_id || data.replayed !== false ||
      (expectedEventId !== undefined && data.event_id !== expectedEventId)) {
    return inconclusive('Missing, replayed or unexpected event');
  }
  const before = Date.parse(startedAt), after = Date.parse(finishedAt);
  const timestamp = typeof data.timestamp === 'number' ? data.timestamp
    : typeof data.timestamp === 'string' ? Date.parse(data.timestamp) : NaN;
  if (!Number.isFinite(before) || !Number.isFinite(after) || before > after ||
      !Number.isFinite(timestamp) || timestamp < before - 5000 || timestamp > after + 5000) {
    return inconclusive('Missing navigation bounds or stale server timestamp');
  }
  return { ...fingerprintObservation({ signals: data }), eventId: data.event_id,
    timestamp, scope: 'server response within owned navigation bounds; session/context validation still required' };
}

export function parseFontReport(text) {
  if (typeof text !== 'string' || !text.trim()) throw new Error('Empty font report');
  return text.trim().split('\n').map(line => {
    const match = /^(\d+),(\d+)\s+(.+)$/.exec(line);
    if (!match) throw new Error('Malformed font metric row');
    return { width: Number(match[1]), height: Number(match[2]), names: match[3].split(', ').filter(Boolean) };
  });
}

// Runs only inside our disposable page. Multiple fallback canvas widths are an
// independent technique, not equivalent to BrowserLeaks' DOM offset dimensions.
export async function localFontProbe(names) {
  await document.fonts.ready;
  const canvas = document.createElement('canvas');
  const c = canvas.getContext('2d');
  if (!c) throw new Error('No canvas context');
  const texts = ['mmmmmmmmmmlliWW00', 'The quick brown fox 0123456789 ₹₺ẞ'];
  const bases = ['monospace', 'serif', 'sans-serif'];
  const measure = family => texts.map(t => { c.font = `72px ${family}`; return c.measureText(t).width; });
  const fallback = Object.fromEntries(bases.map(b => [b, measure(b)]));
  const rows = names.map(name => {
    const metrics = Object.fromEntries(bases.map(b => [b, measure(`${JSON.stringify(name)}, ${b}`)]));
    return { name, detected: bases.some(b => metrics[b].some((w, i) => Math.abs(w - fallback[b][i]) > 0.001)), metrics };
  });
  const nonexist = '__anyprofile_missing_font_98d084051c0e__';
  const negative = bases.every(b => measure(`${JSON.stringify(nonexist)}, ${b}`).every((w, i) => w === fallback[b][i]));
  return { method: 'canvas-multi-fallback-two-texts-v1', rows, negativeControl: negative,
    context: { ua: navigator.userAgent, languages: [...navigator.languages], platform: navigator.platform,
      dpr: devicePixelRatio, innerWidth, innerHeight, outerWidth, outerHeight,
      screenWidth: screen.width, screenHeight: screen.height, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone } };
}

export async function crossApiFontProbe() {
  const names = ['ArialMT', 'Arial', 'Hiragino Sans', 'BlinkMacSystemFont', 'Courier 10 Pitch', 'Kefa', 'Noto Sans Adlam', '__anyprofile_absent_face_98d084051c0e__'];
  const rows = [];
  for (const [index, name] of names.entries()) {
    const check = document.fonts.check(`72px ${JSON.stringify(name)}`);
    let localFaceLoads = false;
    try {
      await new FontFace(`OwnedProbeFace${index}`, `local(${JSON.stringify(name)})`).load();
      localFaceLoads = true;
    } catch {}
    rows.push({ name, documentFontsCheck: check, localFaceLoads });
  }
  return { method: 'font-check-vs-local-face-v1', rows,
    note: 'FontFace local() matches unique face names; family/generic check() disagreement alone is not a bug' };
}

export function validateFontProbe(result, names) {
  if (!result?.negativeControl) throw new Error('Missing-font negative control failed');
  if (!Array.isArray(result.rows) || result.rows.length !== names.length ||
      result.rows.some((row, i) => row.name !== names[i] || typeof row.detected !== 'boolean')) {
    throw new Error('Incomplete or reordered font measurements');
  }
  if (!result.rows.some(row => row.name === 'Arial' && row.detected)) {
    throw new Error('Known installed Arial positive control failed');
  }
  for (const row of result.rows) {
    for (const base of ['monospace', 'serif', 'sans-serif']) {
      if (!Array.isArray(row.metrics?.[base]) || row.metrics[base].length !== 2 ||
          row.metrics[base].some(width => !Number.isFinite(width) || width < 0)) {
        throw new Error('Malformed font measurements');
      }
    }
  }
}

export function modeCommands(mode) {
  if (!['minimal', 'runtime-enabled'].includes(mode)) throw new Error('Invalid mode');
  return mode === 'minimal' ? [] : ['Runtime.enable'];
}

const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function connect(url, methods) {
  if (!/^ws:\/\/127\.0\.0\.1:\d+\//.test(url)) throw new Error('CDP must be owned loopback');
  const ws = new WebSocket(url);
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = () => reject(new Error('CDP connection failed')); });
  let id = 0;
  const pending = new Map();
  ws.onmessage = event => {
    const response = JSON.parse(event.data), item = pending.get(response.id);
    if (!item) return;
    pending.delete(response.id); clearTimeout(item.timer);
    response.error ? item.reject(new Error(response.error.message)) : item.resolve(response.result);
  };
  const send = (method, params = {}) => {
    methods.push(method);
    return new Promise((resolve, reject) => {
      const request = ++id;
      const timer = setTimeout(() => { pending.delete(request); reject(new Error(`CDP timeout: ${method}`)); }, 20000);
      pending.set(request, { resolve, reject, timer });
      ws.send(JSON.stringify({ id: request, method, params }));
    });
  };
  return { send, evaluate: async expression => {
    const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (r.exceptionDetails) throw new Error('Page diagnostic threw an exception');
    return r.result.value;
  }, close: () => { ws.close(); for (const item of pending.values()) { clearTimeout(item.timer); item.reject(new Error('CDP closed')); } pending.clear(); } };
}

async function sha256(file) {
  const hash = createHash('sha256');
  for await (const part of createReadStream(file)) hash.update(part);
  return hash.digest('hex');
}

async function run(args) {
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    if (!args[i].startsWith('--') || !args[i + 1]) throw new Error('Expected --key value');
    options[args[i].slice(2)] = args[i + 1];
  }
  const { manifest, candidates, lane, seed, mode, output } = options;
  modeCommands(mode);
  if (options.remote && !['yes', 'no'].includes(options.remote)) throw new Error('Invalid remote option');
  if (!['anybrowser', 'apostate', 'chrome'].includes(lane) || !/^[0-9a-f]{16}$/.test(seed ?? '')) throw new Error('Invalid lane/seed');
  const preflight = JSON.parse(await readFile(manifest, 'utf8')), names = JSON.parse(await readFile(candidates, 'utf8'));
  if (!Array.isArray(names) || !names.length || names.some(n => typeof n !== 'string' || !n.length)) throw new Error('Invalid font candidates');
  const info = preflight.lanes[lane];
  if (!info || (lane !== 'chrome' && (info.codesign_exit !== 0 || !Object.values(info.cores).every(c => c.matches_historical)))) throw new Error('Unverified lane');
  for (const [relative, core] of Object.entries(info.cores ?? {})) {
    if (await sha256(path.join(info.app, relative)) !== core.sha256) throw new Error('Core changed since preflight');
  }
  await mkdir(output, { recursive: false, mode: 0o700 });
  const profile = await mkdtemp(path.join(tmpdir(), 'anyprofile-owned-fonts-'));
  await mkdir(path.join(profile, 'Default'), { mode: 0o700 });
  await writeFile(path.join(profile, 'Default/Preferences'), JSON.stringify({
    intl: { accept_languages: 'fr-FR,fr,en-US,en', selected_languages: 'fr-FR,fr,en-US,en' },
  }), { mode: 0o600 });
  const binary = path.join(info.app, 'Contents/MacOS', lane === 'chrome' ? 'Google Chrome' : 'Chromium');
  const argv = [`--user-data-dir=${profile}`, '--use-mock-keychain', '--remote-debugging-port=0',
    '--no-first-run', '--no-default-browser-check', '--disable-background-networking', '--disk-cache-size=20971520', '--window-size=820,780'];
  if (lane !== 'chrome') argv.push(`--fingerprint=${seed}`, '--fingerprint-platform=macos',
    '--fingerprint-timezone=Europe/Paris', '--fingerprint-locale=fr-FR,fr,en-US,en',
    '--fingerprint-hardware-concurrency=8', '--fingerprint-device-memory=16');
  argv.push('about:blank');
  const env = { ...process.env };
  for (const k of ['GOOGLE_API_KEY', 'GOOGLE_DEFAULT_CLIENT_ID', 'GOOGLE_DEFAULT_CLIENT_SECRET']) {
    delete env[k]; if (lane === 'apostate') env[k] = 'no';
  }
  const child = spawn(binary, argv, { env, detached: true, stdio: ['ignore', 'ignore', 'pipe'] });
  let stderr = '', spawnError;
  child.on('error', error => { spawnError = error; stderr += `\nspawn: ${error.message}`; });
  child.stderr?.on('data', b => { stderr = (stderr + b).slice(-64000); });
  let client, shutdown = false;
  const record = { lane, seed, seedApplies: lane !== 'chrome', mode, binary, argv, methods: [], startedAt: new Date().toISOString(),
    scope: 'diagnostic collection validity only; not A1/A2 or release acceptance',
    probeSHA256: await sha256(new URL(import.meta.url)), candidateSHA256: await sha256(candidates), status: 'NOT_RUN' };
  try {
    let port;
    for (let i = 0; i < 150; i++) {
      if (spawnError) throw new Error('Owned browser could not be started');
      if (child.exitCode !== null) throw new Error('Browser exited before CDP');
      try { port = Number((await readFile(path.join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]); } catch {}
      if (port > 0) break;
      await delay(100);
    }
    if (!port) throw new Error('Owned browser CDP port unavailable');
    const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(5000) })).json();
    const target = targets.find(t => t.type === 'page');
    client = await connect(target.webSocketDebuggerUrl, record.methods);
    for (const method of modeCommands(mode)) await client.send(method);
    record.local = await client.evaluate(`(${localFontProbe.toString()})(${JSON.stringify(names)})`);
    validateFontProbe(record.local, names);
    record.crossApi = await client.evaluate(`(${crossApiFontProbe.toString()})()`);
    if (!record.crossApi.rows.find(row => row.name === 'ArialMT')?.localFaceLoads ||
        record.crossApi.rows.at(-1).localFaceLoads) throw new Error('Local-face controls failed');
    if (options.remote === 'yes') {
      await client.send('Page.navigate', { url: 'https://browserleaks.com/fonts' });
      let remote;
      for (let i = 0; i < 70; i++) {
        remote = await client.evaluate(`(() => ({ url:location.href, report:document.querySelector('#fonts-metrics-report')?.textContent,
          full:document.querySelector('#fonts-metrics-output')?.textContent, hash:document.querySelector('#fonts-metrics-hash')?.textContent,
          candidateCount:typeof fontsList==='function'?fontsList().length:null }))()`);
        if (remote.full && /fonts and/.test(remote.report)) break;
        await delay(300);
      }
      if (!remote?.full) throw new Error('BrowserLeaks full report unavailable');
      record.browserleaks = { ...remote, rows: parseFontReport(remote.full) };
    }
    record.status = 'PASS';
  } catch (e) { record.status = 'BLOCKED'; record.error = e.message; }
  finally {
    try { if (client) await client.send('Browser.close'); } catch {}
    client?.close();
    const alive = () => child.exitCode === null && child.signalCode === null;
    if (alive()) {
      await Promise.race([once(child, 'exit').catch(() => {}), delay(5000)]);
      if (alive() && child.pid) { try { process.kill(-child.pid, 'SIGTERM'); } catch {} }
      if (alive()) await Promise.race([once(child, 'exit').catch(() => {}), delay(3000)]);
      if (alive() && child.pid) {
        try { process.kill(-child.pid, 'SIGKILL'); } catch {}
        await Promise.race([once(child, 'exit').catch(() => {}), delay(2000)]);
      }
    }
    shutdown = child.exitCode !== null || child.signalCode !== null;
    if (shutdown) await rm(profile, { recursive: true });
    record.cleanup = { profileRemoved: shutdown, ownedPID: child.pid };
    record.finishedAt = new Date().toISOString();
    await writeFile(path.join(output, 'result.json'), JSON.stringify(record, null, 2), { mode: 0o600 });
    await writeFile(path.join(output, 'browser.stderr'), stderr, { mode: 0o600 });
  }
  console.log(JSON.stringify({ lane, seed, mode, status: record.status, error: record.error,
    detected: record.local?.rows.filter(r => r.detected).length, browserleaks: record.browserleaks?.report, cleanup: record.cleanup }));
  if (record.status !== 'PASS' || !shutdown) process.exitCode = 1;
}
if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  run(process.argv.slice(2)).catch(error => { console.error(error.message); process.exitCode = 1; });
}
