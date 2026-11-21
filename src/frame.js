"use strict";

const { ProtocolError } = require("./errors");
const { isKnownOpcode } = require("./opcodes");

/**
 * Decode the payload length field of a frame header.
 *
 * `lengthCode` is the low seven bits of the second header byte and `offset`
 * points just past the two fixed header bytes. Returns `{ length, bytes }`
 * where `bytes` is the width of the extended length field (0 when the length
 * fits in the length code itself), or `null` when the buffer does not hold
 * the whole extended length field yet.
 */
function readExtendedLength(buffer, offset, lengthCode) {
  if (lengthCode === 126) {
    if (buffer.length < offset + 2) {
      return null;
    }
    return { length: buffer.readUInt16BE(offset), bytes: 2 };
  }

  if (lengthCode === 127) {
    if (buffer.length < offset + 8) {
      return null;
    }
    return { length: buffer.readUInt32BE(offset + 4), bytes: 8 };
  }

  return { length: lengthCode, bytes: 0 };
}

/**
 * Decode one frame from the front of `buffer`.
 *
 * Returns `{ frame, bytesRead }` when the buffer holds a complete frame and
 * `null` when more bytes are needed. Reserved opcodes raise a ProtocolError.
 *
 * Frame object:
 *   { fin, rsv1, rsv2, rsv3, opcode, masked, mask, payload, length }
 */
function parseFrame(buffer) {
  if (!Buffer.isBuffer(buffer)) {
    throw new TypeError("parseFrame expects a Buffer");
  }

  if (buffer.length < 2) {
    return null;
  }

  const first = buffer[0];
  const second = buffer[1];

  const fin = (first & 0x80) !== 0;
  const rsv1 = (first & 0x40) !== 0;
  const rsv2 = (first & 0x20) !== 0;
  const rsv3 = (first & 0x10) !== 0;
  const opcode = first & 0x0f;

  const masked = (second & 0x80) !== 0;
  const lengthCode = second & 0x7f;

  if (!isKnownOpcode(opcode)) {
    throw new ProtocolError(1002, `reserved opcode 0x${opcode.toString(16)}`);
  }

  const extended = readExtendedLength(buffer, 2, lengthCode);
  if (extended === null) {
    return null;
  }

  let offset = 2 + extended.bytes;
  const length = extended.length;

  let mask = null;
  if (masked) {
    if (buffer.length < offset + 4) {
      return null;
    }
    mask = buffer.subarray(offset, offset + 4);
    offset += 4;
  }

  if (buffer.length < offset + length) {
    return null;
  }

  const payload = buffer.subarray(offset, offset + length);
  if (masked) {
    for (let i = 0; i < payload.length; i += 1) {
      payload[i] ^= mask[i % 4];
    }
  }

  const frame = {
    fin,
    rsv1,
    rsv2,
    rsv3,
    opcode,
    masked,
    mask,
    payload,
    length,
  };

  return { frame, bytesRead: offset + length };
}

module.exports = {
  parseFrame,
  readExtendedLength,
};
