// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePixelscan } from '../detector-verdicts.mjs';

const page = (fingerprint, masking, automation, proxy) => [
  `Your Browser Fingerprint is ${fingerprint}`,
  proxy ? 'Proxy detected' : 'No proxy detected',
  masking ? 'Masking detected' : 'No masking detected',
  automation ? 'Automated behavior detected' : 'No automated behavior detected',
].join('\n');

test('a consistent unmodified browser passes', () => {
  const got = parsePixelscan(page('consistent', false, false, false));
  assert.equal(got.status, 'PASS');
  assert.equal(got.consistent, true);
  assert.equal(got.masking, false);
});

test('inconsistent or masked is a failure', () => {
  assert.equal(parsePixelscan(page('inconsistent', false, false, false)).status, 'FAIL');
  assert.equal(parsePixelscan(page('consistent', true, false, false)).status, 'FAIL');
  assert.equal(parsePixelscan(page('consistent', false, true, false)).status, 'FAIL');
});

test('a detected proxy is only a warning', () => {
  const got = parsePixelscan(page('consistent', false, false, true));
  assert.equal(got.status, 'WARN');
  assert.deepEqual(got.warnings, ['proxy']);
});

test('a missing verdict line is INCONCLUSIVE, never a pass', () => {
  assert.equal(parsePixelscan('the page did not finish loading').status, 'INCONCLUSIVE');
});
