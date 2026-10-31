// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { speechVerdict, summarise } from '../voice-probe.mjs';

const spoken = (start, end) => ({ events: [['start', start], ['end', end]], error: null });

test('a start then a plausible end is a pass', () => {
  const got = speechVerdict(spoken(334, 4976));
  assert.equal(got.status, 'PASS');
  assert.equal(got.durationMs, 4642);
});

test('not-allowed is a harness/activation block, not a voice failure', () => {
  const got = speechVerdict({ events: [['error', 2, 'not-allowed']], error: 'not-allowed' });
  assert.equal(got.status, 'BLOCKED');
});

test('a missing voice, a timeout and a too-fast end are failures or blocks, never PASS', () => {
  assert.equal(speechVerdict({ missing: true }).status, 'BLOCKED');
  assert.equal(speechVerdict({ events: [], error: 'timeout' }).status, 'FAIL');
  assert.equal(speechVerdict({ events: [['error', 5, 'synthesis-failed']], error: 'synthesis-failed' }).status, 'FAIL');
  assert.equal(speechVerdict(spoken(10, 60)).status, 'FAIL');
  assert.equal(speechVerdict({ events: [['start', 10]], error: null }).status, 'FAIL');
});

test('summary passes only when every requested language spoke', () => {
  const all = summarise({ count: 199, localCount: 180, results: [
    { lang: 'en-US', name: 'Albert', ...spoken(10, 3000) },
    { lang: 'vi-VN', name: 'Linh', ...spoken(10, 3800) },
  ] });
  assert.equal(all.verdict, 'PASS');
  assert.equal(all.voices, 199);

  const one = summarise({ count: 199, results: [
    { lang: 'en-US', name: 'Albert', ...spoken(10, 3000) },
    { lang: 'vi-VN', missing: true },
  ] });
  assert.equal(one.verdict, 'FAIL');

  assert.equal(summarise({ count: 0, results: [] }).verdict, 'INCONCLUSIVE');
});
