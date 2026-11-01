// SPDX-License-Identifier: AGPL-3.0-or-later
// Is the profile-seeded readback noise the SAME answer on every route a page
// can read, and does it leave a solid fill alone? A detector that reads one
// canvas two ways and gets two answers calls that masking, which is what
// happened to a competitor's first noise scheme and to ours in October.
//
//   node tools/bench/canvas-routes.mjs --binary <Chromium> --seed <16hex> [--out result.json]
//
// The browser is launched twice — with and without --fingerprint-noise — on the
// same seed. Every route is decoded here in Node (PNG via zlib), so the
// comparison never asks the browser to read back a canvas it just encoded.
import { spawn } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { pathToFileURL } from 'node:url';

const SCENE = String.raw`(async () => {
  const b64 = (bytes) => {
    let out = ''; const view = new Uint8Array(bytes); const chunk = 0x8000;
    for (let i = 0; i < view.length; i += chunk) out += String.fromCharCode.apply(null, view.subarray(i, i + chunk));
    return btoa(out);
  };
  const canvas2d = (w, h) => {
    const c = document.createElement('canvas'); c.width = w; c.height = h;
    return [c, c.getContext('2d', { willReadFrequently: true })];
  };
  const drawText = (x) => {
    x.textBaseline = 'top'; x.font = '16px Arial'; x.fillStyle = '#f60'; x.fillRect(10, 10, 80, 20);
    x.fillStyle = '#069'; x.fillText('AnyBrowser probe', 10, 35);
    x.fillStyle = '#000'; x.fillRect(120, 5, 3, 3);
  };
  const drawFlat = (x, w, h) => {
    // Solid, opaque, pixel-aligned rectangles: byte-identical on every machine,
    // so noise on them hides nothing and is the one thing a detector can read
    // back exactly.
    x.fillStyle = '#ffffff'; x.fillRect(0, 0, w, h);
    for (let i = 0; i < 14; i++) {
      x.fillStyle = ['#000000', '#ff0000', '#00ff00', '#0000ff'][i % 4];
      x.fillRect(4 + i * 15, 8, 13, 40);
    }
  };
  const readPng = async (blob) => new Uint8Array(await blob.arrayBuffer());

  const [c1, x1] = canvas2d(220, 60);
  drawText(x1);
  const imageData1 = x1.getImageData(0, 0, 220, 60).data;

  const [c2, x2] = canvas2d(220, 60);
  drawText(x2);
  const url2 = c2.toDataURL();

  const [c3, x3] = canvas2d(220, 60);
  drawText(x3);
  const blob3 = await new Promise((r) => c3.toBlob(r, 'image/png'));

  const off = new OffscreenCanvas(220, 60);
  const ox = off.getContext('2d', { willReadFrequently: true });
  drawText(ox);
  const offData = ox.getImageData(0, 0, 220, 60).data;
  const offBlob = await off.convertToBlob({ type: 'image/png' });

  // The same route a second time: a value that moves between identical reads
  // is worse than no noise at all.
  const [c4, x4] = canvas2d(220, 60);
  drawText(x4);
  const again = x4.getImageData(0, 0, 220, 60).data;

  const [flat, fx] = canvas2d(220, 60);
  drawFlat(fx, 220, 60);
  const flatBytes = fx.getImageData(0, 0, 220, 60).data;

  // One opaque colour and nothing else: a real machine renders this byte for
  // byte, so any movement here is noise a detector can compare to nothing.
  const [solid, sx] = canvas2d(220, 60);
  sx.fillStyle = '#336699'; sx.fillRect(0, 0, 220, 60);
  const solidBytes = sx.getImageData(0, 0, 220, 60).data;

  // A Worker's canvas is its own realm: the same seed must reach it.
  const workerSrc = '(async () => {' +
    'const off = new OffscreenCanvas(220, 60);' +
    'const x = off.getContext("2d", { willReadFrequently: true });' +
    'x.textBaseline = "top"; x.font = "16px Arial"; x.fillStyle = "#f60"; x.fillRect(10, 10, 80, 20);' +
    'x.fillStyle = "#069"; x.fillText("AnyBrowser probe", 10, 35);' +
    'x.fillStyle = "#000"; x.fillRect(120, 5, 3, 3);' +
    'const data = x.getImageData(0, 0, 220, 60).data;' +
    'const blob = await off.convertToBlob({ type: "image/png" });' +
    'const png = new Uint8Array(await blob.arrayBuffer());' +
    'postMessage({ data, png });' +
    '})().catch((e) => postMessage({ error: String(e) }));';
  const worker = await new Promise((resolve) => {
    const w = new Worker(URL.createObjectURL(new Blob([workerSrc])));
    w.onmessage = (e) => { resolve(e.data); w.terminate(); };
    w.onerror = (e) => resolve({ error: e.message });
  });

  // WebGL: a smooth ramp and a flat quad. A ramp is where readPixels noise was
  // caught as masking before, because every pixel of a smooth scene differs
  // from its neighbour, so edge-only noise leaves it alone.
  const glScene = (ramp) => {
    const c = document.createElement('canvas'); c.width = 64; c.height = 64;
    const g = c.getContext('webgl', { preserveDrawingBuffer: true });
    const vs = 'attribute vec2 p; void main() { gl_Position = vec4(p, 0.0, 1.0); }';
    const fs = ramp
      ? 'precision mediump float; void main() { gl_FragColor = vec4(gl_FragCoord.x / 64.0, gl_FragCoord.y / 64.0, 0.5, 1.0); }'
      : 'precision mediump float; void main() { gl_FragColor = vec4(0.25, 0.5, 0.75, 1.0); }';
    const sh = (type, src) => { const s = g.createShader(type); g.shaderSource(s, src); g.compileShader(s); return s; };
    const p = g.createProgram(); g.attachShader(p, sh(g.VERTEX_SHADER, vs)); g.attachShader(p, sh(g.FRAGMENT_SHADER, fs));
    g.linkProgram(p); g.useProgram(p);
    const b = g.createBuffer(); g.bindBuffer(g.ARRAY_BUFFER, b);
    g.bufferData(g.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), g.STATIC_DRAW);
    const loc = g.getAttribLocation(p, 'p'); g.enableVertexAttribArray(loc); g.vertexAttribPointer(loc, 2, g.FLOAT, false, 0, 0);
    g.viewport(0, 0, 64, 64); g.clearColor(0, 0, 1, 1); g.clear(g.COLOR_BUFFER_BIT); g.drawArrays(g.TRIANGLE_STRIP, 0, 4);
    const out = new Uint8Array(64 * 64 * 4); g.readPixels(0, 0, 64, 64, g.RGBA, g.UNSIGNED_BYTE, out);
    return out;
  };

  return {
    routes: {
      imageData: b64(imageData1), dataURL: url2, blob: b64(await readPng(blob3)),
      offscreenData: b64(offData), offscreenBlob: b64(await readPng(offBlob)),
      again: b64(again),
      flat: b64(flatBytes), solid: b64(solidBytes),
      worker: { data: b64(worker.data), png: b64(worker.png) },
    },
    webgl: { ramp: b64(glScene(true)), flat: b64(glScene(false)) },
  };
})()`;

export function sha8(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex').slice(0, 16);
}

// decodePng is a deliberately small 8-bit RGB/RGBA, non-interlaced decoder: a
// PNG decoder in the harness is cheaper than trusting the browser to hand back
// the pixels it just encoded.
export function decodePng(buf) {
  if (buf.length < 8 || buf.readUInt32BE(0) !== 0x89504e47) throw new Error('not a png');
  let off = 8, ihdr = null;
  const idat = [];
  while (off + 8 <= buf.length) {
    const len = buf.readUInt32BE(off);
    const type = buf.toString('ascii', off + 4, off + 8);
    const data = buf.subarray(off + 8, off + 8 + len);
    if (type === 'IHDR') ihdr = data;
    if (type === 'IDAT') idat.push(data);
    off += 12 + len;
  }
  if (!ihdr) throw new Error('png without IHDR');
  const width = ihdr.readUInt32BE(0), height = ihdr.readUInt32BE(4);
  const depth = ihdr[8], color = ihdr[9], interlace = ihdr[12];
  if (depth !== 8 || interlace !== 0 || (color !== 6 && color !== 2)) throw new Error(`unsupported png ${depth}/${color}/${interlace}`);
  const bpp = color === 6 ? 4 : 3;
  const raw = zlib.inflateSync(Buffer.concat(idat));
  const stride = width * bpp;
  const out = Buffer.alloc(width * height * 4);
  let prev = Buffer.alloc(stride);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1));
    const cur = Buffer.alloc(stride);
    for (let i = 0; i < stride; i++) {
      const a = i >= bpp ? cur[i - bpp] : 0;
      const b = prev[i];
      const c = i >= bpp ? prev[i - bpp] : 0;
      let v = line[i];
      if (filter === 1) v += a;
      else if (filter === 2) v += b;
      else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) {
        const p = a + b - c, pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
        v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      }
      cur[i] = v & 0xff;
    }
    for (let x = 0; x < width; x++) {
      const s = x * bpp, d = (y * width + x) * 4;
      out[d] = cur[s]; out[d + 1] = cur[s + 1]; out[d + 2] = cur[s + 2];
      out[d + 3] = bpp === 4 ? cur[s + 3] : 255;
    }
    prev = cur;
  }
  return { width, height, pixels: out };
}

// routeVerdict compares the routes a page can read in one run (noise on) and
// the same surfaces with the policy off. `png` routes are decoded pixels.
export function routeVerdict({ noise, plain }) {
  const route = (name) => Buffer.from(noise.routes[name], 'base64');
  const pixels = (name) => decodePng(route(name)).pixels;
  const fromImageData = Buffer.from(noise.routes.imageData, 'base64');
  const hashes = {
    imageData: sha8(fromImageData),
    dataURL: sha8(decodePng(Buffer.from(noise.routes.dataURL.split(',')[1], 'base64')).pixels),
    blob: sha8(pixels('blob')),
    offscreenData: sha8(Buffer.from(noise.routes.offscreenData, 'base64')),
    offscreenBlob: sha8(pixels('offscreenBlob')),
  };
  const workerData = Buffer.from(noise.routes.worker.data, 'base64');
  hashes.worker = sha8(workerData);
  hashes.workerBlob = sha8(decodePng(Buffer.from(noise.routes.worker.png, 'base64')).pixels);
  const distinct = new Set(Object.values(hashes));
  const parity = distinct.size === 1;
  const deterministic = sha8(Buffer.from(noise.routes.again, 'base64')) === hashes.imageData;
  const diff = (name) => {
    const a = Buffer.from(noise.routes[name], 'base64'), b = Buffer.from(plain.routes[name], 'base64');
    let pixels = 0;
    for (let i = 0; i < a.length; i += 4) if (a[i] !== b[i] || a[i + 1] !== b[i + 1] || a[i + 2] !== b[i + 2] || a[i + 3] !== b[i + 3]) pixels++;
    return { changedPixels: pixels, totalPixels: a.length / 4, changedPct: +(100 * pixels / (a.length / 4)).toFixed(2) };
  };
  const flatUnchanged = sha8(Buffer.from(noise.routes.flat, 'base64')) === sha8(Buffer.from(plain.routes.flat, 'base64'));
  const solidUnchanged = sha8(Buffer.from(noise.routes.solid, 'base64')) === sha8(Buffer.from(plain.routes.solid, 'base64'));
  const ramp = Buffer.from(noise.webgl.ramp, 'base64');
  const rampPlain = Buffer.from(plain.webgl.ramp, 'base64');
  const flat = Buffer.from(noise.webgl.flat, 'base64');
  const flatPlain = Buffer.from(plain.webgl.flat, 'base64');
  const count = (a, b) => { let n = 0; for (let i = 0; i < a.length; i += 4) if (a[i] !== b[i] || a[i + 1] !== b[i + 1] || a[i + 2] !== b[i + 2] || a[i + 3] !== b[i + 3]) n++; return n; };
  return {
    hashes, parity, deterministic, flatUnchanged, solidUnchanged,
    flatDiff: diff('flat'), solidDiff: diff('solid'),
    webgl: {
      rampChangedPct: +(100 * count(ramp, rampPlain) / (ramp.length / 4)).toFixed(2),
      flatChangedPct: +(100 * count(flat, flatPlain) / (flat.length / 4)).toFixed(2),
    },
    pass: parity && deterministic && flatUnchanged,
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
    for (let i = 0; i < 200 && !fs.existsSync(path.join(dir, 'DevToolsActivePort')); i++) await new Promise((r) => setTimeout(r, 100));
    const port = fs.readFileSync(path.join(dir, 'DevToolsActivePort'), 'utf8').split('\n')[0];
    const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find((t) => t.type === 'page');
    const ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = () => reject(new Error('DevTools socket failed')); });
    let id = 0; const pending = new Map();
    ws.onmessage = (event) => { const m = JSON.parse(event.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); } };
    const send = (method, params = {}, ms = 25000) => new Promise((resolve) => { const n = ++id; pending.set(n, resolve); ws.send(JSON.stringify({ id: n, method, params })); setTimeout(() => resolve({ timeout: true }), ms); });
    const reply = await send('Runtime.evaluate', { expression: SCENE, returnByValue: true, awaitPromise: true }, 40000);
    if (!reply.result?.result?.value) throw new Error('canvas route probe returned nothing: ' + JSON.stringify(reply.result?.exceptionDetails ?? reply));
    ws.close();
    return reply.result.result.value;
  } finally {
    try { child.kill('SIGTERM'); } catch { /* gone */ }
    await new Promise((r) => setTimeout(r, 800));
    try { child.kill('SIGKILL'); } catch { /* gone */ }
  }
}

async function run(args) {
  const opts = parseArgs(args);
  const binary = opts.binary || process.env.ANYBROWSER_BINARY;
  const seed = opts.seed || process.env.SEED;
  if (!binary || !/^[0-9a-f]{16}$/.test(seed ?? '')) throw new Error('Pass --binary and --seed (16 hex), or ANYBROWSER_BINARY/SEED');
  const base = await fs.promises.mkdtemp(path.join(os.tmpdir(), 'anyprofile-routes-'));
  try {
    const plain = await launch({ binary, seed, noise: false, dir: path.join(base, 'plain') });
    const noise = await launch({ binary, seed, noise: true, dir: path.join(base, 'noise') });
    const result = { seed, verdict: routeVerdict({ noise, plain }), note: 'Headless SwiftShader: route parity and flat exactness only; detector deltas need a native headed run.' };
    if (opts.out) fs.writeFileSync(path.resolve(opts.out), JSON.stringify(result, null, 2), { mode: 0o600 });
    console.log(JSON.stringify(result, null, 2));
    if (!result.verdict.pass) process.exitCode = 1;
  } finally {
    await fs.promises.rm(base, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  run(process.argv.slice(2)).catch((error) => { console.error(error.message); process.exitCode = 1; });
}
