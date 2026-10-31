// SPDX-License-Identifier: AGPL-3.0-or-later
// Open one Chromium-family browser, visit fingerprint test sites, print the verdict lines.
// Usage: node detect-sites.mjs <binary> <profile-dir> [fury-config.json]
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';

const [bin, dir, cfg, ...extraArgs] = process.argv.slice(2);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
if (!process.env.CDP_PORT) {
  fs.mkdirSync(dir, { recursive: true });
  fs.rmSync(`${dir}/DevToolsActivePort`, { force: true });
}

const args = [`--user-data-dir=${dir}`, '--remote-debugging-port=0', '--remote-allow-origins=*',
  '--no-first-run', '--no-default-browser-check', '--hide-crash-restore-bubble', '--use-mock-keychain', '--lang=en-US',
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
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); return; }
  // A site dialog blocks the renderer's main thread, so Runtime.evaluate stops
  // answering; dismiss it the way a user would so the page keeps going.
  if (m.method === 'Page.javascriptDialogOpening') ws.send(JSON.stringify({ id: ++id, method: 'Page.handleJavaScriptDialog', params: { accept: true } }));
};
const send = (method, params = {}, timeout = 30000) => new Promise((r) => { const n = ++id; pending.set(n, r); ws.send(JSON.stringify({ id: n, method, params })); setTimeout(() => r({ timeout: true }), timeout); });
await send('Page.enable');
const text = async (ms = 30000) => (await send('Runtime.evaluate', { expression: 'document.body ? document.body.innerText : ""', returnByValue: true }, ms)).result?.result?.value || '';

// Field-level probe: one flat object of every web-visible surface a detector can
// read, collected in the page. Run it on the start page before navigating so two
// lanes can be diffed field by field instead of by detector score. Each section
// is guarded: a blocked API records an error instead of failing the whole probe.
async function collectProbe() {
  const errors = [], out = {};
  const sync = (key, fn) => { try { out[key] = fn(); } catch (e) { errors.push(`${key}: ${e.message}`); } };
  // A blocked or slow API (WebGPU adapter, audio render, voice list) must not
  // stall the whole probe: each section is raced against a short timer.
  const later = async (key, fn, ms = 2500) => {
    try {
      out[key] = await Promise.race([fn(), new Promise(resolve => setTimeout(() => resolve({ timedOut: true }), ms))]);
    } catch (e) { errors.push(`${key}: ${e.message}`); }
  };
  sync('isSecureContext', () => window.isSecureContext);
  sync('href', () => location.href);
  sync('ua', () => navigator.userAgent);
  sync('appVersion', () => navigator.appVersion);
  sync('platform', () => navigator.platform);
  sync('vendor', () => navigator.vendor);
  sync('product', () => navigator.product);
  sync('hwc', () => navigator.hardwareConcurrency);
  sync('mem', () => navigator.deviceMemory === undefined ? null : navigator.deviceMemory);
  sync('languages', () => navigator.languages);
  sync('language', () => navigator.language);
  sync('webdriver', () => navigator.webdriver);
  sync('plugins', () => navigator.plugins.length);
  sync('mimeTypes', () => navigator.mimeTypes.length);
  sync('pdfViewerEnabled', () => navigator.pdfViewerEnabled);
  sync('cookieEnabled', () => navigator.cookieEnabled);
  sync('doNotTrack', () => navigator.doNotTrack);
  sync('uaData', () => navigator.userAgentData ? { brands: navigator.userAgentData.brands, mobile: navigator.userAgentData.mobile, platform: navigator.userAgentData.platform } : null);
  sync('tz', () => Intl.DateTimeFormat().resolvedOptions().timeZone);
  sync('tzOffset', () => new Date().getTimezoneOffset());
  sync('locale', () => Intl.DateTimeFormat().resolvedOptions().locale);
  sync('hourCycle', () => Intl.DateTimeFormat().resolvedOptions().hourCycle);
  sync('collation', () => new Intl.Collator().resolvedOptions().collation);
  sync('numberFormat', () => new Intl.NumberFormat().format(1234567.89));
  sync('dateFormat', () => new Intl.DateTimeFormat().format(new Date(2026, 0, 2, 3, 4, 5)));
  sync('screen', () => ({ width: screen.width, height: screen.height, availWidth: screen.availWidth, availHeight: screen.availHeight, colorDepth: screen.colorDepth, pixelDepth: screen.pixelDepth, dpr: devicePixelRatio, orientation: screen.orientation ? screen.orientation.type : null }));
  sync('window', () => ({ outerWidth, outerHeight, innerWidth, innerHeight, chromeHeight: outerHeight - innerHeight, screenX, screenY }));
  sync('connection', () => navigator.connection ? { effectiveType: navigator.connection.effectiveType, rtt: navigator.connection.rtt, downlink: navigator.connection.downlink, saveData: navigator.connection.saveData } : null);
  sync('webgl', () => {
    const g = document.createElement('canvas').getContext('webgl');
    if (!g) return null;
    const dbg = g.getExtension('WEBGL_debug_renderer_info');
    return { vendor: g.getParameter(g.VENDOR), renderer: g.getParameter(g.RENDERER),
      unmaskedVendor: dbg ? g.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : null,
      unmaskedRenderer: dbg ? g.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : null,
      version: g.getParameter(g.VERSION), shading: g.getParameter(g.SHADING_LANGUAGE_VERSION),
      maxTexture: g.getParameter(g.MAX_TEXTURE_SIZE), maxRenderbuffer: g.getParameter(g.MAX_RENDERBUFFER_SIZE),
      maxVertexAttribs: g.getParameter(g.MAX_VERTEX_ATTRIBS), maxViewport: Array.from(g.getParameter(g.MAX_VIEWPORT_DIMS) || []) };
  });
  sync('webgl2', () => {
    const g = document.createElement('canvas').getContext('webgl2');
    if (!g) return null;
    return { version: g.getParameter(g.VERSION), maxSamples: g.getParameter(g.MAX_SAMPLES),
      max3D: g.getParameter(g.MAX_3D_TEXTURE_SIZE), maxArrayLayers: g.getParameter(g.MAX_ARRAY_TEXTURE_LAYERS) };
  });
  sync('canvas2d', () => {
    const c = document.createElement('canvas'); c.width = 220; c.height = 60;
    const x = c.getContext('2d', { willReadFrequently: true });
    x.textBaseline = 'top'; x.font = '16px Arial'; x.fillStyle = '#f60'; x.fillRect(10, 10, 80, 20);
    x.fillStyle = '#069'; x.fillText('AnyBrowser probe', 10, 35);
    const u = c.toDataURL(); let h = 2166136261;
    for (let i = 0; i < u.length; i++) { h ^= u.charCodeAt(i); h = Math.imul(h, 16777619) >>> 0; }
    return { hash: h.toString(16), length: u.length };
  });
  sync('fonts', () => {
    const names = ['Arial', 'ArialMT', 'Helvetica', 'Helvetica Neue', 'Menlo', 'Monaco', 'Courier', 'Courier New',
      'Times', 'Times New Roman', 'Verdana', 'Georgia', 'Tahoma', 'Trebuchet MS', 'PingFang SC', 'Hiragino Sans',
      'Apple Color Emoji', 'Apple Symbols', 'BlinkMacSystemFont', 'SF Pro Text',
      'Andale Mono', 'Avenir', 'Baskerville', 'Futura', 'Gill Sans', 'Optima', 'Palatino', 'Rockwell', 'Skia',
      'STHeiti', 'Songti SC', 'Kaiti SC', 'PingFangSC-Regular', 'SF Pro', 'SF Mono'];
    const base = ['monospace', 'serif', 'sans-serif'];
    const x = document.createElement('canvas').getContext('2d');
    const reference = base.map(b => { x.font = `72px ${b}`; return x.measureText('mmmmmmmmmmlli').width; });
    const detected = names.filter(n => base.some((b, i) => {
      x.font = `72px "${n}", ${b}`;
      return Math.abs(x.measureText('mmmmmmmmmmlli').width - reference[i]) > 0.5;
    }));
    return { count: detected.length, detected };
  });
  sync('rects', () => {
    const span = document.createElement('span');
    span.style.cssText = 'font:16px system-ui;position:absolute;visibility:hidden';
    span.textContent = 'Hamburgefonstiv 123';
    document.body.appendChild(span);
    const systemUIWidth = span.getBoundingClientRect().width;
    span.remove();
    const d = document.createElement('input'); d.type = 'date';
    d.style.cssText = 'position:absolute;visibility:hidden';
    document.body.appendChild(d);
    const r = d.getBoundingClientRect();
    d.remove();
    return { systemUIWidth, dateInput: [r.width, r.height] };
  });
  await later('uaHigh', async () => navigator.userAgentData && navigator.userAgentData.getHighEntropyValues
    ? await navigator.userAgentData.getHighEntropyValues(['architecture', 'bitness', 'model', 'platformVersion', 'fullVersionList', 'uaFullVersion', 'wow64'])
    : null);
  await later('webgpu', async () => {
    // requestAdapter() can block for seconds or never settle on a window with
    // no compositor; the format is the cheap, always-available GPU signal.
    if (!navigator.gpu) return null;
    const a = await navigator.gpu.requestAdapter();
    if (!a) return 'no-adapter';
    let info = null;
    try { info = a.info ? { vendor: a.info.vendor, architecture: a.info.architecture, device: a.info.device, description: a.info.description } : null; } catch (e) { info = 'info-error: ' + e.message; }
    return { format: navigator.gpu.getPreferredCanvasFormat(), info,
      limits: { maxTextureDimension2D: a.limits.maxTextureDimension2D, maxStorageBufferBindingSize: a.limits.maxStorageBufferBindingSize, maxComputeWorkgroupStorageSize: a.limits.maxComputeWorkgroupStorageSize } };
  }, 4000);
  await later('voices', async () => {
    const list = await new Promise(resolve => {
      let n = 0;
      const read = () => { const l = speechSynthesis.getVoices(); if (l.length || ++n > 40) resolve(l); else setTimeout(read, 100); };
      read();
    });
    return { count: list.length, sample: list.slice(0, 4).map(v => `${v.name}|${v.lang}|${v.localService}`) };
  });
  await later('mediaDevices', async () => {
    const d = await navigator.mediaDevices.enumerateDevices();
    return d.reduce((m, x) => { m[x.kind] = (m[x.kind] || 0) + 1; return m; }, {});
  });
  await later('storage', async () => { const e = await navigator.storage.estimate(); return { quota: e.quota, usage: e.usage }; });
  await later('battery', async () => {
    if (!navigator.getBattery) return null;
    const b = await navigator.getBattery();
    return { charging: b.charging, level: b.level, chargingTime: b.chargingTime, dischargingTime: b.dischargingTime };
  });
  await later('geolocationPermission', async () => (await navigator.permissions.query({ name: 'geolocation' })).state);
  await later('audio', async () => {
    const Ctx = window.OfflineAudioContext || window.webkitOfflineAudioContext;
    const ctx = new Ctx(1, 44100, 44100);
    const osc = ctx.createOscillator(); osc.type = 'triangle'; osc.frequency.value = 10000;
    const comp = ctx.createDynamicsCompressor();
    comp.threshold.value = -50; comp.knee.value = 40; comp.ratio.value = 12; comp.attack.value = 0; comp.release.value = 0.25;
    osc.connect(comp); comp.connect(ctx.destination); osc.start(0);
    const d = (await ctx.startRendering()).getChannelData(0);
    let sum = 0;
    for (let i = 4500; i < 5000; i++) sum += Math.abs(d[i]);
    return { sum, first: Array.from(d.slice(0, 4)) };
  });
  out.errors = errors;
  return out;
}

if (process.env.PROBE_OUT) {
  // Secure-context APIs (UA-CH high entropy, storage, media devices, local
  // voices) are undefined on about:blank; PROBE_URL runs the same probe on a
  // real https page so both lanes are comparable.
  if (process.env.PROBE_URL) {
    await send('Page.navigate', { url: process.env.PROBE_URL });
    for (let i = 0; i < 60; i++) {
      await sleep(250);
      const state = await send('Runtime.evaluate', { expression: 'document.readyState', returnByValue: true });
      if (state.result?.result?.value === 'complete') break;
    }
    await sleep(500);
  }
  // Page-level ceiling as well: a section that ignores its own timer (or a
  // renderer stuck in a promise) must still return a partial record.
  const r = await send('Runtime.evaluate', {
    expression: `Promise.race([(${collectProbe.toString()})(), new Promise(resolve => setTimeout(() => resolve({ timedOut: true }), 20000))])`,
    returnByValue: true, awaitPromise: true,
  }, 25000);
  const record = { bin, argv: args, at: new Date().toISOString(), url: r.result?.result?.value?.href ?? null,
    // The probe's own source is the version: two records with different hashes
    // measured different field lists and must not be diffed against each other.
    probeHash: createHash('sha256').update(collectProbe.toString()).digest('hex').slice(0, 16),
    exception: r.result?.exceptionDetails ?? null, timeout: !!r.timeout, probe: r.result?.result?.value ?? null };
  fs.writeFileSync(process.env.PROBE_OUT, JSON.stringify(record, null, 2), { mode: 0o600 });
  console.log(`${process.env.PROBE_OUT}: ${record.probe ? 'probe ok' : 'probe MISSING'} errors=${record.probe?.errors?.length ?? 'n/a'}`);
}

const sites = [
  // [name, url, wait ms, regex of lines worth keeping]
  // [name, url, ready regex, regex of lines worth keeping]
  ['ipinfo', 'https://ipinfo.io/json', /"ip"/, /./],
  ['creepjs', 'https://abrahamjuliot.github.io/creepjs/', /% like headless[\s\S]*% stealth/, /(lies|trash|errors|headless|stealth|like headless|bot|crowd-blending|FP ID|Fuzzy)/i],
  ['browserscan', 'https://www.browserscan.net/', /Browser:\n(Chrome|Firefox)/, /(authenticity|bot|robot|Detect|fingerprint|%|WebDriver|CDP)/i],
  ['dbi', 'https://deviceandbrowserinfo.com/are_you_a_bot', /You are (human|a bot)!/, /(bot|human|suspicious|inconsistent|true|false)/i],
  ['pixelscan', 'https://pixelscan.net/fingerprint-check', /Your Browser Fingerprint is (consistent|inconsistent)/, /(consistent|masking|automation|detected|spoof|fingerprint)/i],
  ['iphey', 'https://iphey.com', /MX|Trustworthy|whale/i, /(whale|trustworthy|everything is fine|browser|location|hardware|software|mx|score|warning)/i],
  ['fingerprint', 'https://fingerprint.com/demo', /Bot detection|Suspect|tampering|Your browser/i, /(tampering|suspect|bot|vpn|proxy|anti-?detect|visitor|confidence)/i],
].filter(([n]) => !process.env.SITES || process.env.SITES.split(',').includes(n));
try {
  for (const [name, url, readyRe, keep] of sites) {
    await send('Page.navigate', { url });
    let ok = false, misses = 0;
    const tries = Number(process.env.READY_TRIES || 20);
    for (let k = 0; k < tries && !ok; k++) {
      await sleep(3000);
      const t = await text(12000);
      if (t === '') { if (++misses >= 4) break; } else misses = 0;
      ok = readyRe.test(t);
    }
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
