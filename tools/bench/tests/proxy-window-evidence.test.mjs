// SPDX-License-Identifier: AGPL-3.0-or-later
import { test } from 'node:test';
import { strict as assert } from 'node:assert';
import { validateWindow } from '../proxy-window-evidence.mjs';

const sample = () => ({ ip: '198.51.100.1', country: 'US', timezone: 'America/Chicago' });
const record = () => ({ name: 'iphey', status: 'OBSERVED', before: sample(), after: sample(), text: 'IP ADDRESS\n198.51.100.1\n' });

test('same exit within and across paired site observations', () => {
  assert.equal(validateWindow([record(), record()]).valid, true);
});
test('rejects failed, missing, or invalid observations', () => {
  assert.equal(validateWindow([]).valid, false);
  for (const change of [
    r => { r.status = 'ERROR'; },
    r => { r.before.ip = ''; },
    r => { r.before.ip = 'not-an-ip'; },
    r => { r.after.country = ''; },
    r => { r.after.timezone = ''; },
    r => { r.text = ''; },
    r => { r.text = 'IP ADDRESS\n198.51.100.9\n'; },
  ]) {
    const r = record(); change(r);
    assert.equal(validateWindow([r]).valid, false);
  }
});
test('rejects in-visit and cross-visit drift even when the paired samples match', () => {
  const within = record(); within.after.ip = '198.51.100.2';
  assert.equal(validateWindow([within]).reason, 'egress drift');
  const other = record(); other.before.ip = other.after.ip = '198.51.100.2';
  other.text = 'IP ADDRESS\n198.51.100.2\n';
  assert.equal(validateWindow([record(), other]).reason, 'egress drift');
  for (const field of ['country', 'timezone']) {
    const changed = record(); changed.after[field] = 'different';
    assert.equal(validateWindow([changed]).reason, 'egress drift');
  }
});
