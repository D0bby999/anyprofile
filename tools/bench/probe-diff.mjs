// SPDX-License-Identifier: AGPL-3.0-or-later
// Compare two field probes (detect-sites PROBE_OUT) and classify every
// difference, so an E1 run produces a repeatable verdict instead of a JSON
// read by eye.
//
//   node tools/bench/probe-diff.mjs --purpose control --control stock.json --candidate seed.json
//   node tools/bench/probe-diff.mjs --purpose seeds --candidate a.json --control b.json
//
// purpose=control answers "does the candidate say anything the stock browser on
// this host does not". purpose=seeds answers "are two profiles of the same
// engine distinct where they must be" (canvas, audio, voices, fonts).
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

// Surfaces a profile is allowed to differ on: they are claimed machine
// properties, not statements about the browser build.
const PERSONA = [
  /^screen\./, /^window\.(outer|inner|chrome)/, /^webgl\.(unmasked|max|vendor|renderer)/,
  /^webgl2\./, /^webgpu\./, /^uaHigh\.platformVersion$/, /^battery\./,
  /^connection\./, /^hwc$/, /^mem$/, /^voices\./, /^geolocationPermission$/,
];

// Values that move on their own between two launches of the same browser.
const IGNORED = [
  /^href$/, /^errors$/, /^storage\.(usage|quota)$/, /^window\.(screenX|screenY)$/,
  /^battery\.(chargingTime|dischargingTime|level|charging)$/, /^isSecureContext$/,
];

const CRITICAL = /^(ua|appVersion|platform|vendor|product|languages|language|locale|tz|tzOffset|hourCycle|collation|numberFormat|dateFormat|plugins|mimeTypes|pdfViewerEnabled|cookieEnabled|doNotTrack|fonts|canvas2d|audio|rects|mediaDevices|webgl\.(version|shading))$/;

function flatten(value, prefix = '', out = {}) {
  if (Array.isArray(value)) {
    out[prefix] = JSON.stringify(value);
  } else if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) flatten(child, prefix ? `${prefix}.${key}` : key, out);
  } else {
    out[prefix] = value;
  }
  return out;
}

function classify(field) {
  if (field === 'webdriver') return 'webdriver';
  if (field.startsWith('uaHigh.fullVersionList') || field === 'uaHigh.uaFullVersion') return 'version';
  if (IGNORED.some(re => re.test(field))) return 'ignored';
  if (PERSONA.some(re => re.test(field))) return 'persona';
  if (CRITICAL.test(field)) return 'critical';
  return 'critical'; // unknown surfaces are guilty until the policy names them
}

function major(value) {
  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) {
      for (const item of parsed) {
        if (item.brand === 'Chromium' || item.brand === 'Google Chrome') return Number(String(item.version).split('.')[0]);
      }
    }
  } catch { /* a plain version string, handled below */ }
  const match = String(value).match(/(\d+)\./);
  return match ? Number(match[1]) : NaN;
}

// webdriver: a stock browser under CDP reports true and the candidate hides it;
// that direction is the product's job. The candidate reporting true is a leak.
function webdriverVerdict(control, candidate) {
  if (candidate === false) return { status: control === false ? 'match' : 'control-artifact' };
  return { status: 'critical', detail: 'candidate reports webdriver=true' };
}

export function compareProbes(controlProbe, candidateProbe, purpose = 'control') {
  if (!controlProbe || !candidateProbe) throw new Error('Both probes must be present; a timed-out probe is not a comparison');
  const a = flatten(controlProbe), b = flatten(candidateProbe);
  const fields = [...new Set([...Object.keys(a), ...Object.keys(b)])].sort();
  const rows = [], counts = { match: 0, persona: 0, version: 0, critical: 0, ignored: 0, 'control-artifact': 0 };
  for (const field of fields) {
    const left = a[field], right = b[field];
    const same = JSON.stringify(left) === JSON.stringify(right);
    if (same) { counts.match++; continue; }
    const kind = classify(field);
    if (kind === 'webdriver') {
      const verdict = webdriverVerdict(left, right);
      counts[verdict.status]++;
      rows.push({ field, control: left, candidate: right, kind: 'webdriver', status: verdict.status, detail: verdict.detail });
      continue;
    }
    if (kind === 'version') {
      const sameMajor = major(left) === major(right);
      const status = sameMajor ? 'version' : 'critical';
      counts[status]++;
      rows.push({ field, control: left, candidate: right, kind: 'version', status, detail: sameMajor ? 'same major, different build' : 'different major' });
      continue;
    }
    counts[kind]++;
    rows.push({ field, control: left, candidate: right, kind, status: kind });
  }
  const critical = rows.filter(r => r.status === 'critical');
  return {
    purpose, verdict: critical.length ? 'FAIL' : 'PASS',
    counts, critical,
    version: rows.filter(r => r.status === 'version'),
    persona: rows.filter(r => r.kind === 'persona'),
    controlArtifacts: rows.filter(r => r.status === 'control-artifact'),
    ignored: rows.filter(r => r.kind === 'ignored'),
  };
}

// Distinctness between two profiles of the same engine: identical canvas or
// audio means the two profiles can be linked.
const LINKABLE = ['canvas2d.hash', 'audio.sum', 'fonts.detected', 'voices.count'];

export function compareSeeds(one, two) {
  if (!one || !two) throw new Error('Both probes must be present');
  const a = flatten(one), b = flatten(two);
  const distinct = {};
  for (const field of LINKABLE) distinct[field] = a[field] !== b[field];
  const differing = [...new Set([...Object.keys(a), ...Object.keys(b)])].filter(f => a[f] !== b[f]).sort();
  return { purpose: 'seeds', distinct, linkable: LINKABLE.filter(f => distinct[f] === false), differing };
}

function load(file) {
  const record = JSON.parse(fs.readFileSync(file, 'utf8'));
  if (!record?.probe) throw new Error(`${file} has no probe (timed out or not collected)`);
  return record;
}

// Two records collected by different probe sources measured different fields.
// Diffing them produces false criticals (a field present in one list and absent
// from the other), so refuse before comparing.
export function probeMismatch(control, candidate) {
  const a = control?.probeHash, b = candidate?.probeHash;
  if (!a || !b) return null; // older records carry no hash; the caller is warned below
  if (a === b) return null;
  return `control and candidate were measured by different probes (${a} vs ${b}); recollect both lanes`;
}

function parseArgs(args) {
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    if (!args[i].startsWith('--') || args[i + 1] === undefined) throw new Error('Expected --key value');
    options[args[i].slice(2)] = args[i + 1];
  }
  return options;
}

function run(args) {
  const opts = parseArgs(args);
  if (!['control', 'seeds'].includes(opts.purpose)) throw new Error('--purpose must be control or seeds');
  if (!opts.control || !opts.candidate) throw new Error('--control and --candidate are required');
  const control = load(opts.control), candidate = load(opts.candidate);
  const mismatch = probeMismatch(control, candidate);
  if (mismatch) throw new Error(mismatch);
  if (opts.purpose === 'control' && (!control.probeHash || !candidate.probeHash)) {
    console.error('warning: one probe has no probeHash; field lists must be verified by hand');
  }
  const result = opts.purpose === 'control' ? compareProbes(control.probe, candidate.probe) : compareSeeds(control.probe, candidate.probe);
  if (opts.requireDistinct) {
    const wanted = opts.requireDistinct.split(',');
    const missing = wanted.filter(f => result.distinct?.[f] === false);
    result.requiredDistinct = { fields: wanted, missing, pass: missing.length === 0 };
  }
  const out = opts.out ? path.resolve(opts.out) : null;
  if (out) fs.writeFileSync(out, JSON.stringify(result, null, 2), { mode: 0o600 });
  if (opts.purpose === 'control') {
    console.log(`control vs candidate: ${result.verdict} — critical ${result.counts.critical}, version ${result.counts.version}, persona ${result.counts.persona}, control-artifacts ${result.counts['control-artifact']}`);
    for (const row of result.critical) console.log(`  FAIL ${row.field}: control=${JSON.stringify(row.control)} candidate=${JSON.stringify(row.candidate)}`);
    for (const row of result.version) console.log(`  WARN ${row.field}: control=${JSON.stringify(row.control)} candidate=${JSON.stringify(row.candidate)}`);
    if (result.verdict !== 'PASS') process.exitCode = 1;
  } else {
    console.log(`two seeds: ${result.linkable.length ? `LINKABLE ${result.linkable.join(', ')}` : 'distinct on every linkable surface'}`);
    if (result.requiredDistinct && !result.requiredDistinct.pass) {
      console.log(`  required distinct missing: ${result.requiredDistinct.missing.join(', ')}`);
      process.exitCode = 1;
    }
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try { run(process.argv.slice(2)); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
