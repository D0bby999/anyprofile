// SPDX-License-Identifier: AGPL-3.0-or-later
import test from 'node:test';
import assert from 'node:assert/strict';
import zlib from 'node:zlib';
import { decodePng, routeVerdict, sha8 } from '../canvas-routes.mjs';

// crc32 and encodePng are the test's own encoder: a decoder tested only against
// its own output proves nothing about the PNGs a browser writes, so the bytes
// here are written independently (zlib deflate, filter 0, RGBA8).
function crc32(buf) {
  let c = 0xffffffff;
  for (const byte of buf) {
    c ^= byte;
    for (let k = 0; k < 8; k++) c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
  }
  return (c ^ 0xffffffff) >>> 0;
}

function chunk(type, data) {
  const head = Buffer.alloc(8);
  head.writeUInt32BE(data.length, 0);
  head.write(type, 4, 'ascii');
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), data])), 0);
  return Buffer.concat([head, data, crc]);
}

function encodePng(width, height, pixels) {
  const stride = width * 4;
  const raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y++) {
    raw[y * (stride + 1)] = 0;
    pixels.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; ihdr[9] = 6;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0)),
  ]);
}

const scene = (w = 4, h = 3) => {
  const px = Buffer.alloc(w * h * 4);
  for (let i = 0; i < w * h; i++) {
    px[i * 4] = (i * 7) & 0xff; px[i * 4 + 1] = 255 - i; px[i * 4 + 2] = i % 2 ? 255 : 0; px[i * 4 + 3] = 255;
  }
  return px;
};

test('decodePng returns the pixels an independent encoder wrote', () => {
  const px = scene();
  const decoded = decodePng(encodePng(4, 3, px));
  assert.equal(decoded.width, 4);
  assert.equal(decoded.height, 3);
  assert.equal(sha8(decoded.pixels), sha8(px));
});

test('decodePng rejects a non-png', () => {
  assert.throws(() => decodePng(Buffer.from('not a png at all')), /not a png/);
});

const run = (pixels, extra = {}) => ({
  routes: {
    imageData: pixels.toString('base64'),
    dataURL: 'data:image/png;base64,' + encodePng(4, 3, pixels).toString('base64'),
    blob: encodePng(4, 3, pixels).toString('base64'),
    offscreenData: pixels.toString('base64'),
    offscreenBlob: encodePng(4, 3, pixels).toString('base64'),
    again: pixels.toString('base64'),
    flat: pixels.toString('base64'),
    solid: pixels.toString('base64'),
    worker: { data: pixels.toString('base64'), png: encodePng(4, 3, pixels).toString('base64') },
    ...extra,
  },
  webgl: { ramp: pixels.toString('base64'), flat: pixels.toString('base64') },
});

test('identical routes and an untouched flat fill pass', () => {
  const px = scene();
  const verdict = routeVerdict({ noise: run(px), plain: run(px) });
  assert.equal(verdict.parity, true);
  assert.equal(verdict.deterministic, true);
  assert.equal(verdict.flatUnchanged, true);
  assert.equal(verdict.solidUnchanged, true);
  assert.equal(verdict.pass, true);
});

test('a route that disagrees fails parity', () => {
  const px = scene();
  const moved = Buffer.from(px);
  moved[4] ^= 1;
  const verdict = routeVerdict({ noise: run(px, { blob: encodePng(4, 3, moved).toString('base64') }), plain: run(px) });
  assert.equal(verdict.parity, false);
  assert.equal(verdict.pass, false);
});

test('a flat fill that moves fails the gate and the count is reported', () => {
  const px = scene();
  const moved = Buffer.from(px);
  moved[0] ^= 1; moved[1] ^= 1;
  const verdict = routeVerdict({ noise: run(px, { flat: moved.toString('base64') }), plain: run(px) });
  assert.equal(verdict.flatUnchanged, false);
  assert.equal(verdict.flatDiff.changedPixels, 1);
  assert.equal(verdict.pass, false);
});
