'use strict';

const { MultipartError } = require('./errors');

// Visible ASCII except the characters policy 2.2 excludes from field names.
const HEADER_NAME = /^[!"#$%&'*+\-.0-9A-Z^_`a-z|~]+$/;
const FORBIDDEN_IN_LINE = /[\r\n\x00]/;
const HEX_BYTE = /^[0-9A-Fa-f]{2}$/;

function badHeader(message) {
  return new MultipartError(400, 'BAD_HEADER', message);
}

/**
 * Parse the header block of a single part.
 *
 * The block arrives without its terminating CRLFCRLF. Names are normalised to
 * lower case; repeated names keep the last occurrence. Obsolete line folding
 * is unfolded. Any malformed line rejects the whole request.
 */
function parsePartHeaders(raw) {
  const headers = Object.create(null);
  const lines = String(raw).split('\r\n');
  let current = null;

  for (const line of lines) {
    if (line === '') continue;

    if (line[0] === ' ' || line[0] === '\t') {
      if (current === null) {
        throw badHeader('continuation line without a preceding header line');
      }
      const text = line.replace(/^[ \t]+/, '');
      if (FORBIDDEN_IN_LINE.test(text)) {
        throw badHeader('header continuation contains a forbidden character');
      }
      headers[current] += ' ' + text;
      continue;
    }

    if (FORBIDDEN_IN_LINE.test(line)) {
      throw badHeader('header line contains a forbidden character');
    }

    const colon = line.indexOf(':');
    if (colon <= 0) {
      throw badHeader('header line has an empty name or no colon');
    }

    const name = line.slice(0, colon);
    if (!HEADER_NAME.test(name)) {
      throw badHeader(`invalid header field name: ${name}`);
    }

    const value = line.slice(colon + 1).replace(/^[ \t]+/, '').replace(/[ \t]+$/, '');
    current = name.toLowerCase();
    headers[current] = value;
  }

  return headers;
}

/**
 * Percent-decode a value into a byte buffer. A `%` that is not followed by
 * two hexadecimal digits is malformed and rejects the part.
 */
function percentDecodeBytes(text) {
  const chunks = [];
  let literal = '';

  const flush = () => {
    if (literal !== '') {
      chunks.push(Buffer.from(literal, 'utf8'));
      literal = '';
    }
  };

  for (let i = 0; i < text.length; i++) {
    if (text[i] === '%') {
      const hex = text.slice(i + 1, i + 3);
      if (!HEX_BYTE.test(hex)) {
        throw new MultipartError(
          400,
          'BAD_PERCENT_ENCODING',
          'malformed percent escape: % must be followed by two hexadecimal digits'
        );
      }
      flush();
      chunks.push(Buffer.from([parseInt(hex, 16)]));
      i += 2;
    } else {
      literal += text[i];
    }
  }
  flush();

  return Buffer.concat(chunks);
}

/**
 * Decode an RFC 5987 extended parameter value of the form
 * `charset''language''percent-encoded-value` (single quotes as separators).
 */
function decodeExtendedParam(value) {
  const first = value.indexOf("'");
  const second = first === -1 ? -1 : value.indexOf("'", first + 1);
  if (first === -1 || second === -1) {
    throw new MultipartError(
      400,
      'BAD_EXTENDED_PARAM',
      "extended parameter is not in charset'language'value form"
    );
  }

  const charset = value.slice(0, first).toLowerCase();
  if (charset !== 'utf-8' && charset !== 'iso-8859-1') {
    throw new MultipartError(
      400,
      'UNKNOWN_CHARSET',
      `unsupported charset in extended parameter: ${charset || '(empty)'}`
    );
  }

  const bytes = percentDecodeBytes(value.slice(second + 1));
  return charset === 'utf-8' ? bytes.toString('utf8') : bytes.toString('latin1');
}

/**
 * Parse a Content-Disposition value into its disposition type and parameters.
 *
 * Returns `{ type, params }`. Parameter names are lower case. Quoted-string
 * values keep embedded `;` and `=` and have backslash escapes resolved. An
 * extended parameter (`name*`) takes precedence over its plain counterpart.
 * The `filename` value is percent-decoded; `name` is not.
 */
function parseDispositionParams(value) {
  const text = String(value == null ? '' : value);
  let pos = 0;

  const skipWs = () => {
    while (pos < text.length && (text[pos] === ' ' || text[pos] === '\t')) pos++;
  };

  skipWs();
  let typeEnd = pos;
  while (
    typeEnd < text.length &&
    text[typeEnd] !== ';' &&
    text[typeEnd] !== ' ' &&
    text[typeEnd] !== '\t'
  ) {
    typeEnd++;
  }
  const type = text.slice(pos, typeEnd).toLowerCase();
  pos = typeEnd;

  if (type === '') {
    throw new MultipartError(
      400,
      'MISSING_DISPOSITION',
      'part is missing a Content-Disposition value'
    );
  }
  if (type !== 'form-data' && type !== 'file') {
    throw new MultipartError(
      400,
      'UNSUPPORTED_DISPOSITION',
      `unsupported disposition type: ${type}`
    );
  }

  const raw = Object.create(null);

  while (pos < text.length) {
    if (text[pos] !== ';') {
      while (pos < text.length && text[pos] !== ';') pos++;
      continue;
    }
    pos++;
    skipWs();
    if (pos >= text.length) break;

    let nameEnd = pos;
    while (nameEnd < text.length && text[nameEnd] !== '=' && text[nameEnd] !== ';') {
      nameEnd++;
    }
    if (nameEnd >= text.length || text[nameEnd] !== '=') {
      pos = nameEnd;
      continue;
    }

    const key = text.slice(pos, nameEnd).trim().toLowerCase();
    pos = nameEnd + 1;
    skipWs();

    let val;
    if (text[pos] === '"') {
      pos++;
      let out = '';
      let closed = false;
      while (pos < text.length) {
        const ch = text[pos];
        if (ch === '\\') {
          const next = text[pos + 1];
          if (next === '"' || next === '\\') {
            out += next;
            pos += 2;
          } else {
            // A backslash before any other character is literal: treating it
            // as an escape would let `..\..\x` smuggle path separators past
            // the filename rules.
            out += ch;
            pos++;
          }
        } else if (ch === '"') {
          pos++;
          closed = true;
          break;
        } else {
          out += ch;
          pos++;
        }
      }
      if (!closed) {
        throw new MultipartError(
          400,
          'BAD_DISPOSITION',
          'unterminated quoted-string in Content-Disposition'
        );
      }
      val = out;
    } else {
      let valEnd = pos;
      while (valEnd < text.length && text[valEnd] !== ';') valEnd++;
      val = text.slice(pos, valEnd).replace(/[ \t]+$/, '');
      pos = valEnd;
    }

    if (key !== '') raw[key] = val;
  }

  const params = Object.create(null);
  const extendedBases = new Set();

  for (const key of Object.keys(raw)) {
    if (key.length > 1 && key.endsWith('*')) {
      const base = key.slice(0, -1);
      params[base] = decodeExtendedParam(raw[key]);
      extendedBases.add(base);
    }
  }
  for (const key of Object.keys(raw)) {
    if (key.length > 1 && key.endsWith('*')) continue;
    if (extendedBases.has(key)) continue;
    params[key] = raw[key];
  }

  if (
    Object.prototype.hasOwnProperty.call(params, 'filename') &&
    !extendedBases.has('filename')
  ) {
    params.filename = percentDecodeBytes(params.filename).toString('utf8');
  }

  return { type, params };
}

module.exports = { parsePartHeaders, parseDispositionParams };
