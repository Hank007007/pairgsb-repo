"use strict";

/**
 * Helpers for the test suite. These build wire bytes in memory; nothing here
 * touches a socket.
 */
const OPCODES = {
  CONTINUATION: 0x0,
  TEXT: 0x1,
  BINARY: 0x2,
  CLOSE: 0x8,
  PING: 0x9,
  PONG: 0xa,
};

function toBuffer(value) {
  if (value === undefined || value === null) {
    return Buffer.alloc(0);
  }
  return Buffer.isBuffer(value) ? value : Buffer.from(value, "utf8");
}

function applyMask(payload, key) {
  const out = Buffer.from(payload);
  for (let i = 0; i < out.length; i += 1) {
    out[i] ^= key[i % 4];
  }
  return out;
}

/**
 * Build one frame.
 *
 * options:
 *   fin, opcode, payload, mask (4-byte Buffer), rsv1, rsv2, rsv3,
 *   lengthOverride (forces the length field value without changing the body)
 */
function buildFrame(options = {}) {
  const fin = options.fin !== false;
  const opcode = options.opcode ?? OPCODES.TEXT;
  const payload = toBuffer(options.payload);
  const mask = options.mask ?? null;
  const length = options.lengthOverride ?? payload.length;

  const first =
    (fin ? 0x80 : 0x00) |
    (options.rsv1 ? 0x40 : 0x00) |
    (options.rsv2 ? 0x20 : 0x00) |
    (options.rsv3 ? 0x10 : 0x00) |
    (opcode & 0x0f);

  const parts = [Buffer.from([first])];

  if (length < 126) {
    parts.push(Buffer.from([(mask ? 0x80 : 0x00) | length]));
  } else if (length < 65536) {
    const ext = Buffer.alloc(2);
    ext.writeUInt16BE(length, 0);
    parts.push(Buffer.from([(mask ? 0x80 : 0x00) | 126]), ext);
  } else {
    const ext = Buffer.alloc(8);
    ext.writeBigUInt64BE(BigInt(length), 0);
    parts.push(Buffer.from([(mask ? 0x80 : 0x00) | 127]), ext);
  }

  if (mask) {
    parts.push(mask, applyMask(payload, mask));
  } else {
    parts.push(payload);
  }

  return Buffer.concat(parts);
}

/** Build a masked client frame with the default masking key. */
function clientFrame(options = {}) {
  return buildFrame({
    mask: Buffer.from([0x37, 0xfa, 0x21, 0x3d]),
    ...options,
  });
}

module.exports = {
  OPCODES,
  toBuffer,
  applyMask,
  buildFrame,
  clientFrame,
};
