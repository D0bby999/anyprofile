// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { fingerprintObservation, fingerprintServerObservation, modeCommands, parseFontReport, validateFontProbe } from '../font-diagnostic.mjs';

const entry = (tampering, anti_detect_browser) => ({ signals: { tampering, tampering_details: { anti_detect_browser } } });

test('extracted passing and failing signals remain distinct', () => {
  assert.equal(fingerprintObservation(entry(false, false)).status, 'PASS');
  for (const flags of [[true, false], [false, true], [true, true]]) {
    assert.equal(fingerprintObservation(entry(...flags)).status, 'FAIL');
  }
});

test('missing, null, malformed and aliased fields never become false', () => {
  for (const sample of [null, {}, { signals: {} }, entry(null, false), entry(false, 'false'),
    { signals: { tampering: false, anti_detect: false } }, { text: 'tampering=false anti_detect=false' }]) {
    assert.equal(fingerprintObservation(sample).status, 'INCONCLUSIVE');
  }
});

test('server parser uses only the bound server JSON and validates freshness', () => {
  const timestamp = 1700000000000;
  const data = { event_id: 'owned-test-event', timestamp, replayed: false, tampering: true,
    tampering_details: { anti_detect_browser: true }, note: 'escaped "quote" and braces {}' };
  const bounds = { startedAt: new Date(timestamp - 1000).toISOString(), finishedAt: new Date(timestamp + 1000).toISOString() };
  const text = `marketing tampering=false\nSERVER API RESPONSE\n${JSON.stringify(data)}\nfooter`;
  assert.equal(fingerprintServerObservation(text, bounds).status, 'FAIL');
  assert.equal(fingerprintServerObservation(text).status, 'INCONCLUSIVE');
  assert.equal(fingerprintServerObservation(text, { ...bounds, expectedEventId: 'other' }).status, 'INCONCLUSIVE');
  for (const changed of [{ replayed: true }, { timestamp: timestamp - 60000 }, { tampering: null },
    { tampering_details: {} }, { event_id: '' }]) {
    assert.equal(fingerprintServerObservation(`SERVER API RESPONSE\n${JSON.stringify({ ...data, ...changed })}`, bounds).status, 'INCONCLUSIVE');
  }
  assert.equal(fingerprintServerObservation('landing tampering=false', bounds).status, 'INCONCLUSIVE');
  assert.equal(fingerprintServerObservation(`SERVER API RESPONSE [${JSON.stringify(data)}]`, bounds).status, 'INCONCLUSIVE');
  assert.equal(fingerprintServerObservation('SERVER API RESPONSE {broken}', bounds).status, 'INCONCLUSIVE');
  assert.equal(fingerprintServerObservation(text + '\nSERVER API RESPONSE {}', bounds).status, 'INCONCLUSIVE');
  const clean = { ...data, tampering: false, tampering_details: { anti_detect_browser: false } };
  assert.equal(fingerprintServerObservation(`SERVER API RESPONSE ${JSON.stringify(clean)}`, bounds).status, 'PASS');
});

test('minimal mode never enables Runtime; runtime-enabled is explicit', () => {
  assert.deepEqual(modeCommands('minimal'), []);
  assert.deepEqual(modeCommands('runtime-enabled'), ['Runtime.enable']);
  assert.throws(() => modeCommands('headless'), /Invalid mode/);
});

test('font report keeps aliases and metric grouping, not just totals', () => {
  assert.deepEqual(parseFontReport('130,144\tArial, ArialMT\n140,145\tCourier New'), [
    { width: 130, height: 144, names: ['Arial', 'ArialMT'] },
    { width: 140, height: 145, names: ['Courier New'] },
  ]);
});

test('live font controls reject empty, incomplete, malformed or all-fallback readings', () => {
  const row = { name: 'Arial', detected: true, metrics: { monospace: [100, 110], serif: [100, 110], 'sans-serif': [100, 110] } };
  const valid = { negativeControl: true, rows: [row] };
  assert.doesNotThrow(() => validateFontProbe(valid, ['Arial']));
  assert.throws(() => validateFontProbe({ ...valid, negativeControl: false }, ['Arial']), /negative control/);
  assert.throws(() => validateFontProbe({ ...valid, rows: [] }, ['Arial']), /Incomplete/);
  assert.throws(() => validateFontProbe({ ...valid, rows: [{ ...row, detected: false }] }, ['Arial']), /positive control/);
  assert.throws(() => validateFontProbe({ ...valid, rows: [{ ...row, metrics: { ...row.metrics, serif: [NaN, 2] } }] }, ['Arial']), /Malformed/);
});

test('failed owned browser spawn records BLOCKED and cleans its temporary profile', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'font-diagnostic-fixture-'));
  try {
    await writeFile(join(dir, 'manifest.json'), JSON.stringify({ lanes: { chrome: { app: join(dir, 'missing.app') } } }));
    await writeFile(join(dir, 'candidates.json'), JSON.stringify(['Arial']));
    const run = spawnSync(process.execPath, [fileURLToPath(new URL('../font-diagnostic.mjs', import.meta.url)),
      '--manifest', join(dir, 'manifest.json'), '--candidates', join(dir, 'candidates.json'), '--lane', 'chrome',
      '--seed', '00000000000003e8', '--mode', 'minimal', '--remote', 'no', '--output', join(dir, 'result')],
    { encoding: 'utf8', timeout: 15000 });
    assert.equal(run.status, 1, run.stderr);
    const result = JSON.parse(await readFile(join(dir, 'result/result.json'), 'utf8'));
    assert.equal(result.status, 'BLOCKED');
    assert.equal(result.cleanup.profileRemoved, true);
  } finally { await rm(dir, { recursive: true, force: true }); }
});

test('empty or unexpected font report fails closed', () => {
  for (const sample of ['', null, '237 fonts and 180 unique metrics found', 'abc\tArial']) {
    assert.throws(() => parseFontReport(sample));
  }
});
