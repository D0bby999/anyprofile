// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { compareProbes, compareSeeds } from '../probe-diff.mjs';

const base = () => ({
  ua: 'Mozilla/5.0 (Macintosh) Chrome/156.0.0.0 Safari/537.36',
  platform: 'MacIntel',
  tz: 'Asia/Saigon',
  locale: 'en-US',
  hwc: 8,
  mem: 16,
  screen: { width: 1440, height: 900, availWidth: 1440, availHeight: 782 },
  window: { outerWidth: 1280, outerHeight: 782, innerWidth: 1280, innerHeight: 639 },
  webgl: { version: 'WebGL 1.0 (OpenGL ES 2.0 Chromium)', unmaskedRenderer: 'ANGLE (Apple, ANGLE Metal Renderer: Apple M2, Unspecified Version)' },
  fonts: { count: 30, detected: ['Arial', 'Menlo'] },
  canvas2d: { hash: 'fccf1575', length: 4862 },
  audio: { sum: 124.0434748 },
  voices: { count: 19 },
  mediaDevices: { audioinput: 1, videoinput: 1, audiooutput: 1 },
  webdriver: false,
  uaHigh: {
    platformVersion: '26.7.2',
    fullVersionList: [{ brand: 'Chromium', version: '156.0.8078.74' }, { brand: 'Google Chrome', version: '156.0.8078.74' }],
    uaFullVersion: '156.0.8078.74',
  },
});

const clone = value => JSON.parse(JSON.stringify(value));

test('an identical probe passes with no findings', () => {
  const result = compareProbes(base(), base());
  assert.equal(result.verdict, 'PASS');
  assert.equal(result.counts.critical, 0);
  assert.equal(result.counts.persona, 0);
  assert.ok(result.counts.match > 0);
});

test('canvas, fonts, audio and media devices are critical when they differ', () => {
  for (const mutate of [
    p => { p.canvas2d.hash = 'deadbeef' },
    p => { p.fonts.count = 31 },
    p => { p.audio.sum = 1 },
    p => { p.mediaDevices.audioinput = 0 },
    p => { p.locale = 'vi-VN' },
    p => { p.tz = 'America/New_York' },
  ]) {
    const candidate = base();
    mutate(candidate);
    const result = compareProbes(base(), candidate);
    assert.equal(result.verdict, 'FAIL', JSON.stringify(result.persona));
    assert.equal(result.counts.critical, 1);
  }
});

test('a claimed machine may differ: screen, GPU, cores, battery, network', () => {
  const candidate = base();
  candidate.screen = { width: 1728, height: 1117 };
  candidate.hwc = 10;
  candidate.mem = 32;
  candidate.webgl.unmaskedRenderer = 'ANGLE (Apple, ANGLE Metal Renderer: Apple M4, Unspecified Version)';
  candidate.uaHigh.platformVersion = '26.5.0';
  candidate.voices.count = 199;
  candidate.connection = { rtt: 150 };
  candidate.battery = { level: 0.6 };
  const result = compareProbes(base(), candidate);
  assert.equal(result.verdict, 'PASS');
  assert.equal(result.counts.critical, 0);
  assert.ok(result.counts.persona >= 6);
});

test('a different build of the same major is a warning, a new major is a failure', () => {
  const same = base();
  same.uaHigh.fullVersionList = [{ brand: 'Chromium', version: '156.0.8078.48' }];
  same.uaHigh.uaFullVersion = '156.0.8078.48';
  const sameResult = compareProbes(base(), same);
  assert.equal(sameResult.verdict, 'PASS');
  assert.equal(sameResult.counts.version, 2);

  const next = base();
  next.uaHigh.fullVersionList = [{ brand: 'Chromium', version: '157.0.1.2' }];
  next.uaHigh.uaFullVersion = '157.0.1.2';
  const nextResult = compareProbes(base(), next);
  assert.equal(nextResult.verdict, 'FAIL');
  assert.equal(nextResult.counts.critical, 2);
});

test('webdriver hidden by the candidate is the product, reported not failed', () => {
  const control = base();
  control.webdriver = true;
  const result = compareProbes(control, base());
  assert.equal(result.verdict, 'PASS');
  assert.equal(result.counts['control-artifact'], 1);

  const leaking = base();
  leaking.webdriver = true;
  const fail = compareProbes(base(), leaking);
  assert.equal(fail.verdict, 'FAIL');
  assert.equal(fail.counts.critical, 1);
});

test('storage and battery counters that move on their own are ignored', () => {
  const candidate = base();
  candidate.storage = { quota: 1, usage: 2 };
  const result = compareProbes(base(), candidate);
  assert.equal(result.verdict, 'PASS');
  assert.equal(result.counts.ignored, 2);
});

test('two seeds are linkable when canvas, audio, fonts and voices collide', () => {
  const other = base();
  other.screen = { width: 1728, height: 1117 };
  const same = compareSeeds(base(), other);
  assert.deepEqual(same.linkable.sort(), ['audio.sum', 'canvas2d.hash', 'fonts.detected', 'voices.count']);

  const distinct = base();
  distinct.canvas2d.hash = 'deadbeef';
  distinct.audio.sum = 2;
  distinct.fonts.detected = ['Arial'];
  distinct.voices.count = 199;
  const result = compareSeeds(base(), distinct);
  assert.deepEqual(result.linkable, []);
});

test('a missing probe is an error, not an empty comparison', () => {
  assert.throws(() => compareProbes(null, base()), /Both probes/);
  assert.throws(() => compareProbes(base(), null), /Both probes/);
});

test('probes from different probe sources are refused instead of producing false criticals', async () => {
  const { probeMismatch } = await import('../probe-diff.mjs');
  assert.equal(probeMismatch({ probeHash: 'aa' }, { probeHash: 'aa' }), null);
  assert.match(probeMismatch({ probeHash: 'aa' }, { probeHash: 'bb' }), /different probes/);
  assert.equal(probeMismatch({}, { probeHash: 'bb' }), null);
});
