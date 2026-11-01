// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluate, identityDiff, normalize, summary } from '../context-verdicts.mjs';

const base = () => ({
  ua: 'Mozilla/5.0 (Macintosh) Chrome/156.0.0.0 Safari/537.36',
  platform: 'MacIntel',
  languages: ['en-US'],
  hwc: 8,
  mem: 16,
  tz: 'Asia/Saigon',
  locale: 'en-US',
  webdriver: false,
  canvas: 'b2ddff67b5fb0000',
  webgl: { version: 'WebGL 1.0', vendor: 'Google Inc. (Apple)', renderer: 'ANGLE (Apple, ANGLE Metal Renderer: Apple M4 Max)', maxTexture: 16384 },
  errors: [],
});

const run = (contexts) => ({ at: '2026-11-02T00:00:00Z', contexts });

// A Window answers the window-only fields; a Worker reports them as absences.
const windowCtx = () => ({ ...base(), screen: { w: 1512, h: 982, aw: 1512, ah: 926 }, dpr: 2, voices: 199, audio: 124.04, permissions: 'denied', notification: 'denied' });
const workerCtx = () => { const c = { ...base() }; for (const k of ['screen', 'dpr', 'voices', 'audio', 'permissions', 'notification']) delete c[k]; return c; };

const four = () => ({
  main: windowCtx(),
  worker: workerCtx(),
  'iframe-same-origin': windowCtx(),
  'iframe-cross-origin': windowCtx(),
});

test('agreement passes with eight contexts', () => {
  const v = evaluate(run({ ...four(), sharedworker: workerCtx(), serviceworker: workerCtx() }));
  assert.equal(v.pass, true, summary(v));
  assert.equal(v.disagreements.length, 0);
});

test('a canvas disagreement names the field and every context', () => {
  const r = four();
  r.worker.canvas = 'ffffffffffffffff';
  const v = evaluate(run(r));
  assert.equal(v.pass, false);
  assert.deepEqual(v.disagreements.map((d) => d.field), ['canvas']);
  assert.equal(v.disagreements[0].values.length, 4);
});

test('a window-only field missing in a worker is an absence, not a disagreement', () => {
  const v = evaluate(run(four()));
  assert.equal(v.pass, true, summary(v));
  assert.deepEqual(v.unexpectedAbsences, []);
});

test('a common field answered by no context is an unexpected absence', () => {
  const r = four();
  for (const ctx of Object.values(r)) delete ctx.tz;
  const v = evaluate(run(r));
  assert.equal(v.pass, false);
  assert.deepEqual(v.unexpectedAbsences.map((a) => a.field), ['tz']);
});

test('a probe error fails the gate even when values agree', () => {
  const r = four();
  r.serviceworker = workerCtx();
  r.serviceworker.errors = ['canvas: tainted'];
  const v = evaluate(run(r));
  assert.equal(v.pass, false);
  assert.deepEqual(v.errors, [{ context: 'serviceworker', error: 'canvas: tainted' }]);
});

test('fewer than four contexts is a failure', () => {
  const v = evaluate(run({ main: windowCtx(), worker: workerCtx(), 'iframe-same-origin': windowCtx() }));
  assert.equal(v.pass, false);
  assert.match(v.reason, /only 3 contexts/);
});

test('identity keeps a second launch equal and ignores volatile fields', () => {
  const a = run({ main: windowCtx() });
  const b = run({ main: { ...windowCtx(), href: 'about:blank' } });
  a.contexts.main.at = 'first';
  b.contexts.main.at = 'second';
  assert.equal(identityDiff(a, b).pass, true);
});

test('identity fails when the canvas moves between launches', () => {
  const a = run({ main: windowCtx() });
  const b = run({ main: { ...windowCtx(), canvas: '0000000000000000' } });
  const v = identityDiff(a, b);
  assert.equal(v.pass, false);
  assert.deepEqual(v.moved.map((m) => m.field), ['canvas']);
});

test('normalize separates absence from a value', () => {
  const n = normalize({ screen: undefined, hwc: 8, errors: ['x: y'] });
  assert.deepEqual(n.absent, ['screen']);
  assert.equal(n.values.hwc, 8);
  assert.deepEqual(n.errors, ['x: y']);
});

test('summary reads as one line', () => {
  assert.match(summary({ pass: true, contexts: ['main'] }), /^PASS contexts=1$/);
  assert.match(summary({ pass: false, disagreements: [{ field: 'tz' }], unexpectedAbsences: [], errors: [] }), /disagreements=tz/);
});
