/**
 * Base-128 varint reading and writing.
 *
 * A varint stores an unsigned 64-bit value as a run of bytes; every byte but
 * the last has its high bit set, and the low seven bits of each byte carry a
 * group of the value, least significant group first.
 */

/**
 * Read a varint from `buf` starting at `offset`.
 *
 * @param {Buffer|Uint8Array} buf
 * @param {number} [offset=0]
 * @returns {{ value: bigint, next: number }} value is unsigned 64-bit;
 *   `next` is the offset of the first byte after the varint.
 */
export function readVarint(buf, offset = 0) {
  let value = 0n;
  let shift = 0n;
  let pos = offset;

  while (pos < buf.length) {
    const byte = buf[pos];
    pos += 1;
    value |= BigInt(byte & 0x7f) << shift;
    if ((byte & 0x80) === 0) {
      return { value, next: pos };
    }
    shift += 7n;
  }

  return { value, next: pos };
}

/**
 * Encode a value as a varint.
 *
 * Accepts a non-negative `number` within the safe integer range or a `bigint`
 * in the signed 64-bit range. Negative bigints are written as the ten-byte
 * two's-complement form used for signed 64-bit fields.
 *
 * @param {number|bigint} value
 * @returns {Buffer}
 */
export function writeVarint(value) {
  if (typeof value === 'bigint') {
    let v = value;
    if (v < 0n) {
      v += 1n << 64n;
    }
    const bytes = [];
    do {
      bytes.push(Number(v & 0x7fn) | 0x80);
      v >>= 7n;
    } while (v > 0n);
    bytes[bytes.length - 1] &= 0x7f;
    return Buffer.from(bytes);
  }

  let v = value;
  const bytes = [];
  do {
    bytes.push((v & 0x7f) | 0x80);
    v >>= 7;
  } while (v !== 0);
  bytes[bytes.length - 1] &= 0x7f;
  return Buffer.from(bytes);
}

/**
 * Number of bytes the shortest encoding of `value` would occupy.
 *
 * @param {bigint} value unsigned 64-bit value
 * @returns {number}
 */
export function varintSize(value) {
  let v = value;
  let size = 1;
  while (v > 0x7fn) {
    v >>= 7n;
    size += 1;
  }
  return size;
}
