/**
 * Wire-format encoding.
 *
 * `encodeMessage` is the inverse of `decodeMessage`: it writes declared fields
 * in ascending field-number order and then appends any preserved unknown
 * fields in the order they were seen.
 */
import { UNKNOWN_FIELDS } from './decode.js';
import { writeVarint } from './varint.js';

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

function toVarintValue(field, value) {
  switch (field.type) {
    case 'int32':
    case 'int64':
      return typeof value === 'bigint' ? value : value;
    case 'uint32':
    case 'uint64':
      return typeof value === 'bigint' ? value : BigInt(value);
    case 'bool':
      return value ? 1n : 0n;
    default:
      return typeof value === 'bigint' ? value : BigInt(value);
  }
}

function encodeTag(fieldNumber, wireType) {
  return writeVarint(BigInt(fieldNumber) * 8n + BigInt(wireType));
}

function encodeScalarPayload(field, value) {
  switch (field.type) {
    case 'fixed32': {
      const out = Buffer.alloc(4);
      out.writeUInt32LE(Number(value) >>> 0, 0);
      return out;
    }
    case 'fixed64': {
      const out = Buffer.alloc(8);
      out.writeBigUInt64LE(BigInt.asUintN(64, BigInt(value)), 0);
      return out;
    }
    default:
      return writeVarint(toVarintValue(field, value));
  }
}

function encodeLengthDelimited(payload) {
  return Buffer.concat([writeVarint(BigInt(payload.length)), payload]);
}

function encodeValue(field, value, schema) {
  switch (field.type) {
    case 'message': {
      const child = schema.getType(field.messageType);
      return encodeLengthDelimited(encodeMessage(value, child));
    }
    case 'string':
      return encodeLengthDelimited(Buffer.from(String(value), 'utf8'));
    case 'bytes':
      return encodeLengthDelimited(Buffer.from(value));
    case 'fixed32':
    case 'fixed64':
      return encodeScalarPayload(field, value);
    default:
      return encodeScalarPayload(field, value);
  }
}

function wireTypeFor(field) {
  return WIRE_TYPE_FOR[field.type];
}

/**
 * Encode a decoded message.
 *
 * @param {object} message decoded message object
 * @param {object} schema resolved schema; must expose `fields` and `getType`
 * @returns {Buffer}
 */
export function encodeMessage(message, schema) {
  const chunks = [];
  const fields = [...(schema.fields ?? [])].sort((a, b) => a.number - b.number);

  for (const field of fields) {
    if (!Object.prototype.hasOwnProperty.call(message, field.name)) {
      continue;
    }
    const value = message[field.name];
    const wireType = wireTypeFor(field);

    if (field.repeated) {
      const elements = Array.isArray(value) ? value : [];
      for (const element of elements) {
        chunks.push(encodeTag(field.number, wireType), encodeValue(field, element, schema));
      }
    } else {
      chunks.push(encodeTag(field.number, wireType), encodeValue(field, value, schema));
    }
  }

  const unknown = message[UNKNOWN_FIELDS];
  if (Array.isArray(unknown)) {
    for (const entry of unknown) {
      chunks.push(Buffer.from(entry.raw));
    }
  }

  return Buffer.concat(chunks);
}
