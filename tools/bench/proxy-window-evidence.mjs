// SPDX-License-Identifier: AGPL-3.0-or-later
// Fail-closed validation for private pre/post egress observations, not a detector verdict.
import { readFileSync } from 'node:fs';
import { isIP } from 'node:net';
import { pathToFileURL } from 'node:url';

export function validateWindow(records) {
  if (!Array.isArray(records) || records.length === 0) return { valid: false, reason: 'missing observations' };
  let initial;
  for (const record of records) {
    if (record?.status !== 'OBSERVED') return { valid: false, reason: 'site observation failed' };
    for (const sample of [record.before, record.after]) {
      if (!sample || typeof sample.ip !== 'string' || !isIP(sample.ip) ||
          typeof sample.country !== 'string' || !sample.country ||
          typeof sample.timezone !== 'string' || !sample.timezone) {
        return { valid: false, reason: 'incomplete egress' };
      }
      if (!initial) initial = sample;
      if (sample.ip !== initial.ip || sample.country !== initial.country || sample.timezone !== initial.timezone) {
        return { valid: false, reason: 'egress drift' };
      }
    }
    if (record.name === 'iphey') {
      const match = typeof record.text === 'string' && record.text.match(/IP ADDRESS\s+((?:\d{1,3}\.){3}\d{1,3})/i);
      if (!match || match[1] !== initial.ip) return { valid: false, reason: 'site egress unverified' };
    }
  }
  return { valid: true, reason: 'sampled egress consistent' };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const files = process.argv.slice(2);
    const result = validateWindow(files.map(path => JSON.parse(readFileSync(path, 'utf8'))));
    console.log(JSON.stringify(result));
    if (!result.valid) process.exitCode = 1;
  } catch {
    console.error('Private egress evidence unavailable or invalid');
    process.exitCode = 1;
  }
}
