// SPDX-License-Identifier: AGPL-3.0-or-later
// One browser, eight execution contexts, one answer each. A spoof that reaches
// the main frame but not a Worker is found by a page in one line, so this gate
// asks every context the same questions and fails when the answers disagree.
//
//   node tools/bench/context-gate.mjs <chromium-binary> [--out=run.json] [--arg=--flag]
//   node tools/bench/context-gate.mjs --identity=a.json --identity=b.json
//
// The probe page is served from this process on 127.0.0.1 and localhost, which
// are different origins on the same machine: the cross-origin iframe is a real
// OOPIF candidate, not a same-origin frame wearing a different name. Nothing is
// read over CDP — the page posts its own result — so the gate also runs against
// a browser launched by the application itself.
import { spawn } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { evaluate, identityDiff, summary } from './context-verdicts.mjs';

const NONCE = crypto.randomBytes(8).toString('hex');
const TIMEOUT_MS = Number(process.env.GATE_TIMEOUT_MS || 150000);

// The probe runs verbatim in every context, so it may reference nothing outside
// itself: no imports, no page globals, no helpers. Fields a context cannot
// answer stay undefined and are reported as absences rather than values.
function probe() {
  const errors = [];
  const out = {};
  const sync = (key, fn) => {
    try { const v = fn(); if (v !== undefined) out[key] = v; } catch (e) { errors.push(`${key}: ${e.message}`); }
  };
  const later = async (key, fn, ms = 4000) => {
    try {
      const v = await Promise.race([fn(), new Promise((r) => setTimeout(() => r(undefined), ms))]);
      if (v !== undefined) out[key] = v;
    } catch (e) { errors.push(`${key}: ${e.message}`); }
  };
  sync('ua', () => navigator.userAgent);
  sync('platform', () => navigator.platform);
  sync('languages', () => navigator.languages);
  sync('hwc', () => navigator.hardwareConcurrency);
  sync('mem', () => navigator.deviceMemory);
  sync('webdriver', () => navigator.webdriver);
  sync('tz', () => Intl.DateTimeFormat().resolvedOptions().timeZone);
  sync('locale', () => Intl.DateTimeFormat().resolvedOptions().locale);
  sync('screen', () => (typeof screen === 'undefined' ? undefined
    : { w: screen.width, h: screen.height, aw: screen.availWidth, ah: screen.availHeight }));
  sync('window', () => (typeof outerWidth === 'undefined' ? undefined
    : { ow: outerWidth, oh: outerHeight, iw: innerWidth, ih: innerHeight }));
  sync('dpr', () => (typeof devicePixelRatio === 'undefined' ? undefined : devicePixelRatio));
  sync('voices', () => (typeof speechSynthesis === 'undefined' ? undefined : speechSynthesis.getVoices().length));
  sync('notification', () => (typeof Notification === 'undefined' ? undefined : Notification.permission));
  // A macOS Chrome with overlay scrollbars leaves no room for one, and a
  // persona of another OS claims 15-17 px; this is the tell Fury patches.
  sync('scrollbar', () => (typeof document === 'undefined' ? undefined
    : { reserved: innerWidth - document.documentElement.clientWidth, any: document.documentElement.clientWidth !== innerWidth }));
  // Geolocation: a profile that always answers "denied" is a machine that
  // never once let a site ask, which is not what a fresh Chrome profile does.
  return (async () => {
    await later('canvas', async () => {
      if (typeof OffscreenCanvas === 'undefined') return undefined;
      const c = new OffscreenCanvas(220, 60);
      const x = c.getContext('2d', { willReadFrequently: true });
      x.textBaseline = 'top';
      x.font = '16px Arial';
      x.fillStyle = '#f60';
      x.fillRect(10, 10, 80, 20);
      x.fillStyle = '#069';
      x.fillText('AnyBrowser gate', 10, 35);
      const data = x.getImageData(0, 0, 220, 60).data;
      const digest = await crypto.subtle.digest('SHA-256', data);
      return [...new Uint8Array(digest)].slice(0, 8).map((b) => b.toString(16).padStart(2, '0')).join('');
    });
    await later('webgl', async () => {
      if (typeof OffscreenCanvas === 'undefined') return undefined;
      const g = new OffscreenCanvas(8, 8).getContext('webgl');
      if (!g) return undefined;
      const dbg = g.getExtension('WEBGL_debug_renderer_info');
      return {
        version: g.getParameter(g.VERSION),
        vendor: dbg ? g.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : null,
        renderer: dbg ? g.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : null,
        maxTexture: g.getParameter(g.MAX_TEXTURE_SIZE),
      };
    });
    await later('audio', async () => {
      if (typeof OfflineAudioContext === 'undefined') return undefined;
      const ctx = new OfflineAudioContext(1, 44100, 44100);
      const osc = ctx.createOscillator();
      osc.type = 'triangle';
      osc.frequency.value = 10000;
      const comp = ctx.createDynamicsCompressor();
      comp.threshold.value = -50; comp.knee.value = 40; comp.ratio.value = 12;
      comp.attack.value = 0; comp.release.value = 0.25;
      osc.connect(comp); comp.connect(ctx.destination); osc.start(0);
      const d = (await ctx.startRendering()).getChannelData(0);
      let sum = 0;
      for (let i = 4500; i < 5000; i++) sum += Math.abs(d[i]);
      return Number(sum.toFixed(6));
    }, 8000);
    await later('permissions', async () => {
      if (typeof navigator === 'undefined' || !navigator.permissions) return undefined;
      return (await navigator.permissions.query({ name: 'notifications' })).state;
    });
    await later('geoPermission', async () => {
      if (typeof navigator === 'undefined' || !navigator.permissions) return undefined;
      return (await navigator.permissions.query({ name: 'geolocation' })).state;
    });
    await later('geolocation', async () => {
      if (typeof navigator === 'undefined' || !navigator.geolocation) return undefined;
      return await new Promise((resolve) => {
        const t = setTimeout(() => resolve({ error: 'timeout' }), 3000);
        navigator.geolocation.getCurrentPosition(
          (p) => { clearTimeout(t); resolve({ lat: +p.coords.latitude.toFixed(4), lon: +p.coords.longitude.toFixed(4), accuracy: p.coords.accuracy }); },
          (e) => { clearTimeout(t); resolve({ error: e.code + ':' + e.message }); },
          { timeout: 2500, maximumAge: 0 });
      });
    });
    out.errors = errors;
    return out;
  })();
}

const PROBE_SRC = probe.toString();

function page(ports) {
  const origin = `http://127.0.0.1:${ports.main}`;
  const other = `http://127.0.0.1:${ports.other}`;
  return `<!doctype html><meta charset="utf-8"><title>context gate</title>
<body><pre id="log"></pre><script>
const NONCE = ${JSON.stringify(NONCE)}, PORT = ${ports.main};
const OTHER = ${JSON.stringify(other)};
const debug = [];
const log = (m) => { debug.push(m); document.getElementById('log').textContent += m + '\\n'; };
const ctx = {};
const probeSrc = ${JSON.stringify(PROBE_SRC)};
const probe = (0, eval)('(' + probeSrc + ')');
const withTimeout = (p, ms) => Promise.race([p, new Promise((r) => setTimeout(() => r({ errors: ['timeout'] }), ms))]);

function workerContext() {
  return withTimeout(new Promise((resolve) => {
    const w = new Worker(URL.createObjectURL(new Blob([
      '(' + probeSrc + ')().then((r) => postMessage(r)).catch((e) => postMessage({ errors: [String(e)] }))'])));
    w.onmessage = (e) => { resolve(e.data); w.terminate(); };
    w.onerror = (e) => resolve({ errors: ['worker: ' + e.message] });
  }), 15000);
}

function sharedContext() {
  return withTimeout(new Promise((resolve) => {
    const w = new SharedWorker(URL.createObjectURL(new Blob([
      probeSrc + ';onconnect = (e) => { const p = e.ports[0]; p.onmessage = () => (' +
      probeSrc + ')().then((r) => p.postMessage(r)).catch((er) => p.postMessage({ errors: [String(er)] })); p.start(); }']))).port;
    w.onmessage = (e) => resolve(e.data);
    w.onerror = (e) => resolve({ errors: ['shared: ' + e.message] });
    w.postMessage('go');
  }), 15000);
}

function serviceContext() {
  return withTimeout((async () => {
    if (!navigator.serviceWorker) return { errors: ['no serviceWorker'] };
    const reg = await navigator.serviceWorker.register('/sw.js?nonce=' + NONCE);
    await navigator.serviceWorker.ready;
    const sw = reg.active;
    return await new Promise((resolve) => {
      const ch = new MessageChannel();
      ch.port1.onmessage = (e) => resolve(e.data);
      sw.postMessage({ go: NONCE }, [ch.port2]);
      setTimeout(() => resolve({ errors: ['serviceworker: timeout'] }), 12000);
    });
  })(), 16000);
}

function frameContext(kind) {
  return withTimeout(new Promise((resolve) => {
    const iframe = document.createElement('iframe');
    iframe.width = 400; iframe.height = 300;
    document.body.appendChild(iframe);
    const run = async () => {
      try {
        if (kind === 'cross-origin') {
          // A second port is a different origin, not a different site: the
          // frame is third-party and unreadable from here, which is what the
          // OOPIF case is about; site isolation itself is not asserted.
          const nonce = NONCE + ':' + kind;
          const onMessage = (e) => {
            if (e.data?.nonce !== nonce || !e.data.result) return;
            window.removeEventListener('message', onMessage);
            log(kind + ': frame answered');
            resolve(e.data.result);
          };
          window.addEventListener('message', onMessage);
          iframe.src = OTHER + '/frame?nonce=' + encodeURIComponent(nonce);
          return;
        }
        if (kind === 'srcdoc') {
          const loaded = new Promise((r) => { iframe.onload = r; });
          iframe.srcdoc = '<!doctype html><meta charset="utf-8"><p>frame</p>';
          await loaded;
        } else if (kind === 'about-blank') {
          await new Promise((r) => setTimeout(r, 50));
        } else {
          const loaded = new Promise((r) => { iframe.onload = r; });
          iframe.src = '/frame';
          await loaded;
        }
        // Same-origin frames are readable, so the parent runs the probe in the
        // frame's own realm — the frame's navigator, not the parent's.
        const result = await iframe.contentWindow.eval('(' + probeSrc + ')()');
        log(kind + ': frame answered ' + (iframe.contentWindow.location.href || '?'));
        resolve(result);
      } catch (e) {
        log(kind + ': ' + e.message);
        resolve({ errors: [kind + ': ' + e.message] });
      }
    };
    setTimeout(run, 0);
  }), 20000);
}

(async () => {
  const jobs = {
    main: () => probe(),
    worker: workerContext,
    sharedworker: sharedContext,
    serviceworker: serviceContext,
    'iframe-same-origin': () => frameContext('same-origin'),
    'iframe-about-blank': () => frameContext('about-blank'),
    'iframe-srcdoc': () => frameContext('srcdoc'),
    'iframe-cross-origin': () => frameContext('cross-origin'),
  };
  for (const [name, run] of Object.entries(jobs)) {
    try { ctx[name] = await run(); } catch (e) { ctx[name] = { errors: [String(e)] }; }
    log(name + ': ' + Object.keys(ctx[name]).filter((k) => k !== 'errors').length + ' fields, ' + (ctx[name].errors?.length || 0) + ' errors');
  }
  const record = { at: new Date().toISOString(), url: location.href, nonce: NONCE, contexts: ctx, debug };
  await fetch('/result', { method: 'POST', body: JSON.stringify(record) });
  log('posted');
})().catch((e) => log('fatal: ' + e.message));
<\/script></body>`;
}

const PROBE_JS = `${PROBE_SRC}
(async () => {
  const p = new URLSearchParams(location.search);
  const nonce = (typeof window !== 'undefined' && window.__probeNonce) || p.get('nonce');
  if (!nonce) return;
  const result = await probe();
  parent.postMessage({ nonce, result }, '*');
})();`;

const SW_JS = `${PROBE_SRC}
self.addEventListener('message', (e) => {
  const port = e.ports[0];
  if (!port) return;
  probe().then((r) => port.postMessage(r)).catch((er) => port.postMessage({ errors: [String(er)] }));
});`;

function args(argv) {
  const out = { binary: '', out: '', identity: [], pass: [], profile: '' };
  for (const a of argv) {
    if (a.startsWith('--out=')) out.out = a.slice(6);
    else if (a.startsWith('--identity=')) out.identity.push(a.slice(11));
    else if (a.startsWith('--arg=')) out.pass.push(a.slice(6));
    else if (a.startsWith('--profile=')) out.profile = a.slice(10);
    else if (!a.startsWith('--')) out.binary = a;
  }
  return out;
}

function serve() {
  const state = { resolve: null, reject: null };
  const done = new Promise((resolve, reject) => { state.resolve = resolve; state.reject = reject; });
  const ports = { main: 0, other: 0 };
  const handle = (req, res) => {
    const url = new URL(req.url, `http://127.0.0.1:${ports.main}`);
    if (process.env.GATE_DEBUG) console.error('gate request', req.method, url.pathname + url.search);
    if (req.method === 'POST' && url.pathname === '/result') {
      let body = '';
      req.on('data', (c) => { body += c; });
      req.on('end', () => {
        res.writeHead(204).end();
        try { state.resolve(JSON.parse(body)); } catch (e) { state.reject(e); }
      });
      return;
    }
    const send = (type, body) => { res.writeHead(200, { 'content-type': type, 'cache-control': 'no-store' }); res.end(body); };
    if (url.pathname === '/probe.js') return send('text/javascript', PROBE_JS);
    if (url.pathname === '/sw.js') return send('text/javascript', SW_JS.replace('${NONCE}', NONCE));
    if (url.pathname === '/frame') {
      const nonce = encodeURIComponent(url.searchParams.get('nonce') || '');
      return send('text/html', `<!doctype html><meta charset="utf-8"><script src="/probe.js?nonce=${nonce}"></script>`);
    }
    return send('text/html', page(ports));
  };
  const listen = () => new Promise((resolve) => {
    const server = http.createServer(handle);
    server.listen(0, '::', () => resolve(server));
  });
  return (async () => {
    const main = await listen();
    ports.main = main.address().port;
    // A second port on the same host is a different origin: readable only
    // through postMessage, like any third-party frame.
    const other = await listen();
    ports.other = other.address().port;
    return { servers: [main, other], done, ports };
  })();
}

async function main() {
  const opts = args(process.argv.slice(2));
  if (opts.identity.length === 2) {
    const a = JSON.parse(fs.readFileSync(opts.identity[0], 'utf8'));
    const b = JSON.parse(fs.readFileSync(opts.identity[1], 'utf8'));
    const verdict = identityDiff(a, b);
    console.log(summary(verdict));
    for (const m of verdict.moved) console.log(`moved ${m.field}: ${JSON.stringify(m.before)} -> ${JSON.stringify(m.after)}`);
    process.exit(verdict.pass ? 0 : 1);
  }
  if (!opts.binary || !fs.existsSync(opts.binary)) {
    console.error('usage: node context-gate.mjs <chromium-binary> [--out=run.json] [--arg=--flag]...');
    process.exit(2);
  }
  const profile = opts.profile || fs.mkdtempSync(path.join(os.tmpdir(), 'context-gate-'));
  const { servers, done, ports } = await serve();
  const url = `http://127.0.0.1:${ports.main}/`;
  const browser = spawn(opts.binary, [
    `--user-data-dir=${profile}`, '--no-first-run', '--no-default-browser-check',
    '--hide-crash-restore-bubble', '--use-mock-keychain', '--lang=en-US',
    '--window-position=-4000,-4000', '--window-size=900,700', ...opts.pass, url,
  ], { stdio: ['ignore', 'ignore', 'pipe'] });
  let stderr = '';
  browser.stderr.on('data', (d) => { stderr = (stderr + d).slice(-4000); });
  const timer = setTimeout(() => {
    console.error('timed out waiting for the page to report');
    browser.kill('SIGKILL');
    process.exit(1);
  }, TIMEOUT_MS);
  try {
    const run = await done;
    clearTimeout(timer);
    run.browser = opts.binary;
    if (opts.out) fs.writeFileSync(opts.out, JSON.stringify(run, null, 2));
    const verdict = evaluate(run);
    console.log(summary(verdict));
    for (const d of verdict.disagreements) console.log(`disagrees ${d.field}: ${JSON.stringify(d.values)}`);
    for (const a of verdict.unexpectedAbsences) console.log(`absent ${a.field} in ${a.context} (${a.reason})`);
    for (const e of verdict.errors) console.log(`error ${e.context}: ${e.error}`);
    if (!verdict.pass && stderr.trim()) console.error(stderr.trim().split('\n').slice(-5).join('\n'));
    process.exit(verdict.pass ? 0 : 1);
  } catch (e) {
    console.error('gate failed:', e.message);
    process.exit(1);
  } finally {
    for (const server of servers) server.close();
    browser.kill('SIGTERM');
    if (!opts.profile) fs.rmSync(profile, { recursive: true, force: true });
  }
}

main();
