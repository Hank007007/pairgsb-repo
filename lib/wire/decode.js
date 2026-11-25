/**
 * Wire-format decoding.
 *
 * `decodeMessage` turns a buffer into a plain object keyed by the field names
 * declared in the schema. `readField` reads the payload of one field once its
 * tag has been consumed.
 */
import { ProtolensError, CODES } from '../errors.js';
import { readVarint } from './varint.js';

/**
 * Property under which fields the schema does not declare are carried on a
 * decoded object. The value is an array of `{ fieldNumber, wireType, raw }`
 * where `raw` is the original tag+payload bytes.
 */
export const UNKNOWN_FIELDS = Symbol('protolens.unknownFields');

const DEFAULT_MAX_DEPTH = 16;

const WIRE_TYPE_FOR = {
  int32: 0,
  int64: 0,
  uint32: 0,
  uint64: 0,
  bool: 0,
  string: 2,
  bytes: 2,
  message: 2,
  fixed32: 5,
  fixed64: 1,
};

/** Types that may be transported in the packed repeated form. */
const PACKABLE = new Set(['int32', 'int64', 'uint32', 'uint64', 'bool', 'fixed32', 'fixed64']);

function narrow(field, value) {
  switch (field.type) {
    case 'int32':
      return Number(BigInt.asIntN(32, value));
    case 'uint32':
      return Number(BigInt.asUintN(32, value));
    case 'int64':
      return BigInt.asIntN(64, value);
    case 'uint64':
      return BigInt.asUintN(64, value);
    case 'bool':
      return value !== 0n;
    default:
      return value;
  }
}

/**
 * Read the payload of a field whose tag has already been consumed.
 *
 * @param {Buffer} buf buffer the field lives in
 * @param {number} pos offset of the first payload byte
 * @param {number} fieldNumber field number from the tag
 * @param {number} wireType wire type from the tag
 * @param {object|undefined} field schema entry, or undefined when the schema
 *   does not declare this field number
 * @param {object} ctx decoding context (`depth`, `options`, `getType`)
 * @returns {{ value: any, next: number }}
 */
export function readField(buf, pos, fieldNumber, wireType, field, ctx) {
  switch (wireType) {
    case 0: {
      const { value, next } = readVarint(buf, pos);
      if (field === undefined) {
        return { value, next };
      }
      return { value: narrow(field, value), next };
    }

    case 1: {
      if (pos + 8 > buf.length) {
        throw new ProtolensError(CODES.TRUNCATED_FIELD, 'truncated fixed64 field', {
          offset: pos,
          fieldNumber,
        });
      }
      return { value: buf.readBigUInt64LE(pos), next: pos + 8 };
    }

    case 2: {
      const len = readVarint(buf, pos);
      const length = Number(len.value);
      const payload = buf.subarray(len.next, len.next + length);

      if (field === undefined) {
        return { value: Buffer.from(payload), next: len.next + length };
      }

      switch (field.type) {
        case 'message': {
          const nested = ctx.getType(field.messageType);
          const value = decodeMessage(payload, nested, {
            ...ctx.options,
            depth: 0,
            getType: ctx.getType,
          });
          return { value, next: len.next + length };
        }
        case 'string':
          return { value: payload.toString('utf8'), next: len.next + length };
        case 'bytes':
          return { value: Buffer.from(payload), next: len.next + length };
        default:
          return { value: payload, next: len.next + length };
      }
    }

    case 5: {
      if (pos + 4 > buf.length) {
        throw new ProtolensError(CODES.TRUNCATED_FIELD, 'truncated fixed32 field', {
          offset: pos,
          fieldNumber,
        });
      }
      return { value: buf.readUInt32LE(pos), next: pos + 4 };
    }

    default: {
      const { value, next } = readVarint(buf, pos);
      return { value, next };
    }
  }
}

/**
 * Decode one message.
 *
 * @param {Buffer|Uint8Array} buf encoded bytes
 * @param {object} schema resolved schema (`fields`, `fieldByNumber`, `getType`)
 * @param {object} [options] `depth`, `maxDepth`, `maxBytes`, `getType`
 * @returns {object} decoded message; undeclared fields are attached under
 *   {@link UNKNOWN_FIELDS}
 */
export function decodeMessage(buf, schema, options = {}) {
  if (!Buffer.isBuffer(buf)) {
    buf = Buffer.from(buf);
  }

  const depth = options.depth ?? 1;
  const maxDepth = options.maxDepth ?? DEFAULT_MAX_DEPTH;
  const maxBytes = options.maxBytes ?? Number.MAX_SAFE_INTEGER;
  const getType = options.getType ?? schema?.getType;

  if (depth === 1 && buf.length > maxBytes) {
    throw new ProtolensError(CODES.SIZE_LIMIT_EXCEEDED, 'message exceeds the byte budget', {
      limit: maxBytes,
      actual: buf.length,
    });
  }

  if (depth > maxDepth) {
    throw new ProtolensError(CODES.MAX_DEPTH_EXCEEDED, 'message nesting is too deep', {
      depth,
      limit: maxDepth,
    });
  }

  const message = {};
  let pos = 0;

  while (pos < buf.length) {
    const tag = readVarint(buf, pos);
    pos = tag.next;

    const wireType = Number(tag.value & 0x7n);
    const fieldNumber = Number(tag.value >> 3n);
    const field =
      typeof schema?.fieldByNumber === 'function'
        ? schema.fieldByNumber(fieldNumber)
        : schema?.fields?.find((f) => f.number === fieldNumber);

    const { value, next } = readField(buf, pos, fieldNumber, wireType, field, {
      depth,
      options,
      getType,
    });
    pos = next;

    if (field === undefined || value === undefined) {
      continue;
    }

    if (field.repeated) {
      const current = message[field.name] ?? (message[field.name] = []);
      if (wireType === 2 && PACKABLE.has(field.type)) {
        for (const element of readPackedValues(buf, value, field)) {
          current.push(element);
        }
      } else {
        current.push(value);
      }
    } else {
      message[field.name] = value;
    }
  }

  return message;
}

/**
 * Split a packed scalar payload into individual values.
 *
 * @param {Buffer} buf buffer the payload was sliced from
 * @param {Buffer} payload packed bytes
 * @param {object} field schema entry
 * @returns {any[]}
 */
function readPackedValues(buf, payload, field) {  const values = [];
  let pos = 0;
  while (pos < payload.length) {
    const element = readField(payload, pos, field.number, WIRE_TYPE_FOR[field.type], field, {
      depth: 1,
      options: {},
      getType: () => undefined,
    });
    values.push(element.value);
    pos = element.next;
  }
  return values;
}
