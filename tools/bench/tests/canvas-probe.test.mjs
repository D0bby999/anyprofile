// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import { canvasVerdict, pixelStats } from '../canvas-probe.mjs';

const rgba = (...px) => Buffer.from(px.flat());

test('noise passes only when plain collides and noisy output is distinct and stable', () => {
  const good = canvasVerdict({ plainA: 'aa', plainB: 'aa', plainReopen: 'aa', noiseA: 'bb', noiseB: 'cc', noiseReopen: 'bb' });
  assert.deepEqual(good, { plainCollision: true, plainStable: true, noiseDistinct: true, noiseStable: true, pass: true });

  for (const broken of [
    { noiseA: 'bb', noiseB: 'bb' },
    { noiseA: 'bb', noiseReopen: 'dd' },
    { plainB: 'dd' },
    { plainReopen: 'dd' },
  ]) {
    const base = { plainA: 'aa', plainB: 'aa', plainReopen: 'aa', noiseA: 'bb', noiseB: 'cc', noiseReopen: 'bb' };
    assert.equal(canvasVerdict({ ...base, ...broken }).pass, false, JSON.stringify(broken));
  }
});

test('pixel stats count changed pixels, max channel delta and alpha changes', () => {
  const plain = rgba([10, 20, 30, 255], [40, 50, 60, 255], [70, 80, 90, 255], [1, 2, 3, 255]);
  const noisy = rgba([11, 20, 30, 255], [40, 50, 60, 255], [70, 80, 92, 255], [1, 2, 3, 255]);
  assert.deepEqual(pixelStats(plain, noisy), {
    pixels: 4, changedPixels: 2, changedPct: 50, maxChannelDelta: 2,
    meanDeltaPerChangedPixel: 1.5, alphaChanges: 0,
  });

  const withAlpha = rgba([10, 20, 30, 255], [40, 50, 60, 255], [70, 80, 90, 255], [1, 2, 3, 255]);
  assert.equal(pixelStats(plain, withAlpha.map((value, index) => index % 4 === 3 ? value - 1 : value)).alphaChanges, 4);
});

test('mismatched buffers are refused instead of producing stats', () => {
  assert.throws(() => pixelStats(rgba([1, 2, 3, 255]), rgba([1, 2, 3])), /RGBA/);
  assert.throws(() => pixelStats(rgba([1, 2, 3, 255]), rgba([1, 2, 3, 255], [4, 5, 6, 255])), /same size/);
});
