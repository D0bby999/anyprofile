// SPDX-License-Identifier: AGPL-3.0-or-later
// Verdict logic for context-gate.mjs: does one browser answer the same way in
// every execution context, and does a second launch of the same profile answer
// the same way as the first?
//
// Kept apart from the runner so the rules are tested without a browser. The
// distinction that matters, borrowed from a competitor that had to learn it:
// a value that is absent in a context is not a disagreement, it is an
// absence — a Worker has no screen and no speechSynthesis. An absence is only
// a finding when the field is one that context should have had.

// Fields every context can answer (Window, dedicated Worker, SharedWorker,
// ServiceWorker and iframes all expose them).
export const COMMON = ['ua', 'platform', 'languages', 'hwc', 'mem', 'tz', 'locale', 'canvas', 'webgl'];

// navigator.webdriver is not exposed to every context, so it is checked where
// it is answered: a Window that says true is a driven browser, and the whole
// point of the build is that a page cannot tell.

// Fields only a Window can answer. A Worker's absence here is expected.
export const WINDOW_ONLY = ['screen', 'window', 'dpr', 'voices', 'audio', 'permissions', 'notification'];

// A context may legitimately answer null for these on a locked-down page, but
// the probe records an error instead; a null here is worth a human look.
export const VOLATILE = ['href', 'at'];

function same(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

// normalize() flattens one context's probe into { field: value } with errors
// and absences separated, so a context that threw on canvas does not read as
// "canvas: undefined" beside a real value.
export function normalize(context) {
  const values = {};
  const absent = new Set();
  for (const [key, value] of Object.entries(context || {})) {
    if (key === 'errors') continue;
    if (value === undefined) { absent.add(key); continue; }
    values[key] = value;
  }
  for (const key of absent) delete values[key];
  return { values, absent: [...absent].sort(), errors: [...(context?.errors || [])].sort() };
}

// evaluate() judges one run: every field a context answers must agree with
// every other context that answers it. Returns the disagreements and the
// absences, and a pass flag that is false on any disagreement, on a missing
// common field, or on errors recorded by the probe.
export function evaluate(run) {
  const contexts = Object.entries(run?.contexts || {});
  const disagreements = [];
  const unexpectedAbsences = [];
  const errors = [];
  if (contexts.length < 4) {
    return { pass: false, reason: `only ${contexts.length} contexts reported, want at least 4`, disagreements, unexpectedAbsences, errors };
  }
  for (const field of COMMON) {
    const seen = [];
    const missingIn = [];
    for (const [name, ctx] of contexts) {
      const { values } = normalize(ctx);
      if (values[field] !== undefined) seen.push([name, values[field]]);
      else missingIn.push(name);
    }
    // Every context exposes the common fields, so one that does not answer is
    // either a patch that never reached it or a probe that broke there.
    if (missingIn.length === contexts.length) {
      unexpectedAbsences.push({ field, context: '*', reason: 'no context answered it' });
    } else {
      for (const name of missingIn) unexpectedAbsences.push({ field, context: name, reason: 'missing' });
    }
    if (seen.length < 2) continue;
    const [, first] = seen[0];
    const differing = seen.filter(([, value]) => !same(value, first));
    if (differing.length) {
      disagreements.push({ field, values: seen.map(([name, value]) => ({ context: name, value })) });
    }
  }
  for (const [name, ctx] of contexts) {
    const wd = normalize(ctx).values.webdriver;
    if (wd !== undefined && wd !== false) disagreements.push({ field: 'webdriver', values: [{ context: name, value: wd }] });
    for (const error of ctx?.errors || []) errors.push({ context: name, error });
  }
  return {
    pass: disagreements.length === 0 && unexpectedAbsences.length === 0 && errors.length === 0,
    disagreements, unexpectedAbsences, errors,
    contexts: contexts.map(([name]) => name),
  };
}

// identityDiff() compares two runs of the same profile. A profile that says
// something different about itself on a second launch describes hardware that
// moved while the page was looking, which is worse than no spoofing at all.
// Only the main frame is compared: an identity is one machine, not nine.
export function identityDiff(a, b) {
  const left = normalize(a?.contexts?.main);
  const right = normalize(b?.contexts?.main);
  const moved = [];
  for (const field of [...new Set([...Object.keys(left.values), ...Object.keys(right.values)])]) {
    if (VOLATILE.includes(field)) continue;
    if (!same(left.values[field], right.values[field])) {
      moved.push({ field, before: left.values[field], after: right.values[field] });
    }
  }
  return { pass: moved.length === 0, moved };
}

// summary() is the one-line verdict a CI log needs.
export function summary(verdict) {
  if (verdict.pass) return verdict.contexts ? `PASS contexts=${verdict.contexts.length}` : 'PASS identity';
  const parts = [];
  if (verdict.reason) parts.push(verdict.reason);
  if (verdict.disagreements?.length) parts.push(`disagreements=${verdict.disagreements.map((d) => d.field).join(',')}`);
  if (verdict.unexpectedAbsences?.length) parts.push(`absences=${verdict.unexpectedAbsences.map((a) => a.field).join(',')}`);
  if (verdict.errors?.length) parts.push(`errors=${verdict.errors.length}`);
  if (verdict.moved?.length) parts.push(`moved=${verdict.moved.map((m) => m.field).join(',')}`);
  return `FAIL ${parts.join(' ')}`;
}
