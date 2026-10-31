// SPDX-License-Identifier: AGPL-3.0-or-later
// Strictly read a detector's verdict from the page text detect-sites saved.
// Marketing copy on the landing page is ignored: the Fingerprint.com fields are
// taken only from a JSON object that carries suspect_score, and the IPhey score
// only from the row above "MX SCORE". A missing field is INCONCLUSIVE, never
// false and never PASS.
//
//   node tools/bench/detector-verdicts.mjs --iphey site-iphey.txt --fingerprint site-fingerprint.txt
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

// JSON objects embedded in a text dump; the demo prints one pretty-printed
// block. Brace matching keeps strings and escapes intact.
export function jsonBlocks(text) {
  const out = [];
  for (let start = text.indexOf('{'); start !== -1; start = text.indexOf('{', start + 1)) {
    let depth = 0, quoted = false, escaped = false;
    for (let i = start; i < text.length; i++) {
      const c = text[i];
      if (quoted) {
        if (escaped) escaped = false;
        else if (c === '\\') escaped = true;
        else if (c === '"') quoted = false;
      } else if (c === '"') quoted = true;
      else if (c === '{') depth++;
      else if (c === '}' && --depth === 0) {
        try { out.push(JSON.parse(text.slice(start, i + 1))); } catch { /* not JSON */ }
        break;
      }
    }
  }
  return out;
}

export function parseIPhey(text) {
  const lines = String(text).split('\n').map(line => line.trim()).filter(Boolean);
  let score = null;
  const mx = lines.findIndex(line => /MX\s*S(C)?ORE/i.test(line));
  if (mx >= 0) {
    for (let i = mx - 1; i >= Math.max(0, mx - 3); i--) {
      if (/^\d{1,3}$/.test(lines[i])) { score = Number(lines[i]); break; }
    }
    if (score === null) {
      const same = lines[mx].match(/(\d{1,3})\s*MX/i);
      if (same) score = Number(same[1]);
    }
  }
  const whale = /\bwhale\b/i.test(text);
  const automated = /automated browser environment signal/i.test(text);
  const diver = /\(diver\)/i.test(text);
  const trustworthy = /\btrustworthy\b/i.test(text);
  const warnings = [];
  if (whale) warnings.push('whale');
  if (automated || diver) warnings.push('automation-signal');
  if (score === null) return { site: 'iphey', status: 'INCONCLUSIVE', reason: 'no MX score in the page text', score, whale, automated, warnings };
  const status = warnings.length ? 'WARN' : 'PASS';
  return { site: 'iphey', status, score, trustworthy, whale, automated, warnings };
}

const asBool = value => (typeof value === 'boolean' ? value : undefined);

// Pixelscan prints one verdict line per group; masking or an inconsistent
// fingerprint is a failure, a detected proxy is only a warning (it says nothing
// about the engine), and a missing verdict is INCONCLUSIVE.
export function parsePixelscan(text) {
  const lines = String(text).split('\n').map(line => line.trim().toLowerCase()).filter(Boolean);
  const line = phrase => lines.includes(phrase);
  const fingerprint = String(text).match(/Your Browser Fingerprint is (consistent|inconsistent)/i);
  if (!fingerprint) return { site: 'pixelscan', status: 'INCONCLUSIVE', reason: 'no fingerprint verdict line' };
  const masking = line('masking detected'), noMasking = line('no masking detected');
  const automated = line('automated behavior detected'), noAutomated = line('no automated behavior detected');
  // A missing masking/bot verdict means the page did not finish: never a pass.
  if (!masking && !noMasking) return { site: 'pixelscan', status: 'INCONCLUSIVE', reason: 'no masking verdict line' };
  if (!automated && !noAutomated) return { site: 'pixelscan', status: 'INCONCLUSIVE', reason: 'no automation verdict line' };
  const proxy = line('proxy detected');
  const consistent = fingerprint[1].toLowerCase() === 'consistent';
  const warnings = [];
  if (masking) warnings.push('masking');
  if (!consistent) warnings.push('inconsistent');
  if (automated) warnings.push('automated');
  if (proxy) warnings.push('proxy');
  const status = masking || !consistent || automated ? 'FAIL' : (proxy ? 'WARN' : 'PASS');
  return { site: 'pixelscan', status, consistent, masking, noMasking, automated, noAutomated, proxy, warnings };
}

export function parseFingerprint(text) {
  const blocks = jsonBlocks(String(text)).filter(block => block && typeof block === 'object'
    && typeof block.suspect_score === 'number' && typeof block.tampering === 'boolean');
  if (!blocks.length) return { site: 'fingerprint', status: 'INCONCLUSIVE', reason: 'no server JSON block with suspect_score and tampering' };
  const data = blocks.at(-1);
  const anti = data.tampering_details ? asBool(data.tampering_details.anti_detect_browser) : undefined;
  const botType = typeof data.bot_type === 'string' ? data.bot_type : null;
  const bot = typeof data.bot === 'string' ? data.bot : null;
  const tampering = data.tampering;
  const antiDetect = anti === true;
  const warnings = [];
  if (tampering) warnings.push('tampering');
  if (antiDetect) warnings.push('anti-detect');
  if (botType) warnings.push('bot:' + botType);
  if (typeof data.proxy === 'boolean' && data.proxy) warnings.push('proxy');
  if (data.suspect_score > 0) warnings.push('suspect-score');
  // The visitor id itself is a tracking identifier: record that one exists, never the value.
  const visitorIdPresent = typeof data.visitor_id === 'string' && data.visitor_id.length > 0;
  const status = tampering || antiDetect || botType ? 'FAIL' : (warnings.length ? 'WARN' : 'PASS');
  return { site: 'fingerprint', status, suspectScore: data.suspect_score, tampering, antiDetect,
    bot, botType, visitorIdPresent, warnings };
}

function load(file) {
  return fs.readFileSync(file, 'utf8');
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
  if (!opts.iphey && !opts.fingerprint) throw new Error('Pass --iphey <file> and/or --fingerprint <file>');
  const result = {};
  if (opts.iphey) result.iphey = parseIPhey(load(opts.iphey));
  if (opts.fingerprint) result.fingerprint = parseFingerprint(load(opts.fingerprint));
  console.log(JSON.stringify(result, null, 2));
  const inconclusive = Object.values(result).some(v => v.status === 'INCONCLUSIVE');
  if (inconclusive) process.exitCode = 2;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try { run(process.argv.slice(2)); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
