// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseIPhey, parseFingerprint } from '../detector-verdicts.mjs';

const server = data => `landing copy: tampering=false suspect score 29
SERVER API RESPONSE
${JSON.stringify({ event_id: 'owned', replayed: false, ...data }, null, 1)}`;

test('IPhey reads the MX score from the row above MX SCORE', () => {
  const text = 'Your Digital Identity Looks\nTrustworthy\n100\nMX SCORE\nHow trustworthy is your identity\nNo suspicious signals detected';
  const got = parseIPhey(text);
  assert.equal(got.status, 'PASS');
  assert.equal(got.score, 100);
  assert.equal(got.trustworthy, true);
  assert.equal(got.whale, false);
});

test('IPhey automation signal and whale become warnings, not PASS', () => {
  for (const [text, warning] of [
    ['70\nMX SCORE\nSIGNALS\nSoftware\nDetected an automated browser environment signal (diver)', 'automation-signal'],
    ['90\nMX SCORE\nSIGNALS\nBROWSER\nwhale', 'whale'],
  ]) {
    const got = parseIPhey(text);
    assert.equal(got.status, 'WARN');
    assert.ok(got.warnings.includes(warning), JSON.stringify(got));
  }
});

test('IPhey without a score is INCONCLUSIVE, not zero', () => {
  const got = parseIPhey('The page did not finish loading');
  assert.equal(got.status, 'INCONCLUSIVE');
  assert.equal(got.score, null);
});

test('Fingerprint.com reads the server JSON, never the marketing line', () => {
  const clean = parseFingerprint(server({ suspect_score: 0, tampering: false, tampering_details: { anti_detect_browser: false }, bot: 'not_detected' }));
  assert.equal(clean.status, 'PASS');
  assert.equal(clean.suspectScore, 0);
  assert.equal(clean.tampering, false);

  const marketingOnly = parseFingerprint('Your browser: tampering=false, anti_detect=false, suspect score 0');
  assert.equal(marketingOnly.status, 'INCONCLUSIVE');
});

test('Fingerprint.com bot and tampering are failures', () => {
  const bot = parseFingerprint(server({ suspect_score: 14, tampering: false, tampering_details: { anti_detect_browser: false }, bot: 'bad', bot_type: 'webdriver' }));
  assert.equal(bot.status, 'FAIL');
  assert.ok(bot.warnings.includes('bot:webdriver'));

  const tamper = parseFingerprint(server({ suspect_score: 6, tampering: true, tampering_details: { anti_detect_browser: true } }));
  assert.equal(tamper.status, 'FAIL');
  assert.equal(tamper.antiDetect, true);
});

test('Fingerprint.com suspect score alone is a warning, and the visitor id value is never returned', () => {
  const got = parseFingerprint(server({ suspect_score: 6, tampering: false, tampering_details: { anti_detect_browser: false }, visitor_id: 'U44QRq21p5bZ0lbpbg5i' }));
  assert.equal(got.status, 'WARN');
  assert.equal(got.visitorIdPresent, true);
  assert.equal('visitor_id' in got, false);
  assert.equal(JSON.stringify(got).includes('U44QRq21p5bZ0lbpbg5i'), false);
});

test('a malformed block is INCONCLUSIVE', () => {
  assert.equal(parseFingerprint('SERVER API RESPONSE\n{"suspect_score": 1,').status, 'INCONCLUSIVE');
  assert.equal(parseFingerprint('SERVER API RESPONSE\n{"suspect_score": "1", "tampering": false}').status, 'INCONCLUSIVE');
});
