// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { compareSite, summarise } from '../paired-verdicts.mjs';

const iphey = (status, score) => ({ site: 'iphey', status, ...(typeof score === 'number' ? { score } : {}) });

test('a candidate that loses twice in three paired rounds is an engine delta', () => {
  const result = compareSite(
    [iphey('PASS', 100), iphey('PASS', 100), iphey('PASS', 100)],
    [iphey('PASS', 100), iphey('WARN', 90), iphey('FAIL', 70)]);
  assert.equal(result.status, 'ENGINE_DELTA');
  assert.equal(result.worse, 2);
  assert.deepEqual(result.rows.map(row => row.outcome), ['same', 'worse', 'worse']);
});

test('a control that fails too makes the window an environment problem', () => {
  const result = compareSite(
    [iphey('FAIL', 70), iphey('FAIL', 70), iphey('PASS', 100)],
    [iphey('FAIL', 70), iphey('FAIL', 70), iphey('PASS', 100)]);
  assert.equal(result.status, 'ENVIRONMENT');
  assert.equal(result.worse, 0);
});

test('one bad round out of three is noise, not an engine delta', () => {
  const result = compareSite(
    [iphey('PASS', 100), iphey('PASS', 100), iphey('PASS', 100)],
    [iphey('WARN', 90), iphey('PASS', 100), iphey('PASS', 100)]);
  assert.equal(result.status, 'VENDOR_NOISE');
});

test('a candidate that is better everywhere passes only in a clean control window', () => {
  const clean = compareSite(
    [iphey('WARN', 90), iphey('WARN', 90), iphey('PASS', 100)],
    [iphey('PASS', 100), iphey('PASS', 100), iphey('PASS', 100)]);
  assert.equal(clean.status, 'PASS');
  assert.equal(clean.better, 2);

  // A control that fails most rounds makes the window unusable: the candidate
  // looking better says nothing until the control is clean again.
  const dirty = compareSite(
    [iphey('FAIL', 70), iphey('FAIL', 70), iphey('PASS', 100)],
    [iphey('PASS', 100), iphey('PASS', 100), iphey('PASS', 100)]);
  assert.equal(dirty.status, 'ENVIRONMENT');
});

test('a missing or inconclusive round blocks the site instead of passing it', () => {
  const result = compareSite([iphey('PASS', 100), iphey('PASS', 100)], [iphey('PASS', 100), { site: 'iphey', status: 'INCONCLUSIVE' }]);
  assert.equal(result.status, 'INCONCLUSIVE');
  assert.equal(result.inconclusive, 1);

  const missing = compareSite([iphey('PASS', 100), iphey('PASS', 100)], [iphey('PASS', 100), { site: 'iphey', status: 'NOT_RUN' }]);
  assert.equal(missing.status, 'INCONCLUSIVE');
});

test('scores are kept per round, never averaged into a verdict', () => {
  const result = compareSite([iphey('PASS', 100)], [iphey('WARN', 90)]);
  assert.equal(result.rows[0].controlScore, 100);
  assert.equal(result.rows[0].candidateScore, 90);
});

test('the summary fails when any site is not PASS and names the blocker', () => {
  const pass = compareSite([iphey('PASS', 100), iphey('PASS', 100)], [iphey('PASS', 100), iphey('PASS', 100)]);
  const fail = compareSite([iphey('PASS', 100), iphey('PASS', 100)], [iphey('FAIL', 70), iphey('FAIL', 70)]);
  const result = summarise({ iphey: pass, fingerprint: fail });
  assert.equal(result.verdict, 'FAIL');
  assert.deepEqual(result.blockers, [{ site: 'fingerprint', status: 'ENGINE_DELTA' }]);

  const allPass = summarise({ iphey: pass });
  assert.equal(allPass.verdict, 'PASS');
  assert.deepEqual(allPass.blockers, []);
});
