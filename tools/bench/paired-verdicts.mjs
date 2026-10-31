// SPDX-License-Identifier: AGPL-3.0-or-later
// Enforce the paired-control detector gate: the candidate must not be worse
// than stock Chrome run in the same session, and a failure is classified before
// it can block a release. Reads the page dumps detect-sites wrote (site-<name>.txt)
// through detector-verdicts, which never turns a missing field into PASS.
//
//   node tools/bench/paired-verdicts.mjs \
//     --control run/stock-r1,run/stock-r2,run/stock-r3 \
//     --candidate run/cand-r1,run/cand-r2,run/cand-r3 \
//     [--sites iphey,fingerprint] [--out paired.json]
//
// Exit 0 only when every site passes; 1 otherwise. ENVIRONMENT and VENDOR_NOISE
// are printed with the classification so they are not mistaken for an engine bug.
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { parseIPhey, parseFingerprint, parsePixelscan } from './detector-verdicts.mjs';

const RANK = { PASS: 0, WARN: 1, FAIL: 2, INCONCLUSIVE: 3, NOT_RUN: 4 };
const scoreOf = verdict => (typeof verdict.score === 'number' ? verdict.score
  : typeof verdict.suspectScore === 'number' ? verdict.suspectScore : null);

// compareSite takes the per-round verdicts for one site on each lane and returns
// rows plus one classification. ENVIRONMENT wins when the control itself fails
// in most rounds: nothing about the candidate can be concluded from that window.
export function compareSite(control, candidate) {
  const rounds = Math.max(control.length, candidate.length);
  const rows = [];
  for (let i = 0; i < rounds; i++) {
    const left = control[i] ?? { status: 'NOT_RUN' };
    const right = candidate[i] ?? { status: 'NOT_RUN' };
    const base = { round: i + 1, control: left.status, candidate: right.status, controlScore: scoreOf(left), candidateScore: scoreOf(right) };
    const unusable = ['INCONCLUSIVE', 'NOT_RUN'].includes(left.status) || ['INCONCLUSIVE', 'NOT_RUN'].includes(right.status);
    if (unusable) { rows.push({ ...base, outcome: 'inconclusive' }); continue; }
    const delta = RANK[right.status] - RANK[left.status];
    rows.push({ ...base, outcome: delta > 0 ? 'worse' : delta < 0 ? 'better' : 'same' });
  }
  const valid = rows.filter(row => row.outcome !== 'inconclusive');
  const inconclusive = rows.length - valid.length;
  const controlFailed = valid.filter(row => row.control === 'FAIL').length;
  const worse = valid.filter(row => row.outcome === 'worse').length;
  let status;
  if (inconclusive > 0) status = 'INCONCLUSIVE';
  else if (controlFailed * 2 >= Math.max(1, valid.length)) status = 'ENVIRONMENT';
  else if (worse === 0) status = 'PASS';
  else if (worse * 2 >= valid.length) status = 'ENGINE_DELTA';
  else status = 'VENDOR_NOISE';
  return { status, rounds: rows.length, inconclusive, controlFailed, worse, better: valid.filter(r => r.outcome === 'better').length, rows };
}

export function parseRound(file, site) {
  let text;
  try { text = fs.readFileSync(file, 'utf8'); } catch { return { site, status: 'NOT_RUN' }; }
  if (site === 'iphey') return parseIPhey(text);
  if (site === 'pixelscan') return parsePixelscan(text);
  return parseFingerprint(text);
}

export function summarise(sites) {
  const blockers = Object.entries(sites).filter(([, site]) => site.status !== 'PASS');
  return {
    verdict: blockers.length ? 'FAIL' : 'PASS',
    sites,
    blockers: blockers.map(([name, site]) => ({ site: name, status: site.status })),
    note: 'ENVIRONMENT means the control failed too; ENGINE_DELTA needs a reproducible candidate-only loss; VENDOR_NOISE is neither a pass nor a release blocker by itself.',
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

function run(args) {
  const opts = parseArgs(args);
  if (!opts.control || !opts.candidate) throw new Error('--control and --candidate are required (comma-separated round directories)');
  const controlDirs = opts.control.split(',').filter(Boolean);
  const candidateDirs = opts.candidate.split(',').filter(Boolean);
  if (controlDirs.length !== candidateDirs.length) throw new Error('Control and candidate need the same number of rounds');
  if (controlDirs.length < 2) throw new Error('At least two paired rounds are required; one round cannot separate a delta from noise');
  const sites = (opts.sites || 'iphey,fingerprint,pixelscan').split(',').filter(Boolean);
  const out = {};
  for (const site of sites) {
    const control = controlDirs.map(dir => parseRound(path.join(dir, `site-${site}.txt`), site));
    const candidate = candidateDirs.map(dir => parseRound(path.join(dir, `site-${site}.txt`), site));
    out[site] = compareSite(control, candidate);
  }
  const result = summarise(out);
  if (opts.out) fs.writeFileSync(path.resolve(opts.out), JSON.stringify(result, null, 2), { mode: 0o600 });
  console.log(`paired gate: ${result.verdict} over ${controlDirs.length} rounds`);
  for (const [name, site] of Object.entries(result.sites)) {
    const scores = site.rows.map(row => `${row.controlScore ?? '-'}/${row.candidateScore ?? '-'}`).join(' ');
    console.log(`  ${name}: ${site.status} — worse ${site.worse}/${site.rounds}, control failed ${site.controlFailed}, scores ${scores}`);
  }
  if (result.verdict !== 'PASS') process.exitCode = 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try { run(process.argv.slice(2)); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
