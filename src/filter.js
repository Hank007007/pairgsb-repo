import { protocolError } from './ldap/errors.js';

const ATTRIBUTE_TYPE = /^[A-Za-z][A-Za-z0-9-]*(;[A-Za-z0-9-]+)*$/;

function escapeByte(code) {
  return `\\${code.toString(16).padStart(2, '0')}`;
}

function escapeLoneSurrogate(code) {
  return (
    escapeByte(0xe0 | (code >> 12)) +
    escapeByte(0x80 | ((code >> 6) & 0x3f)) +
    escapeByte(0x80 | (code & 0x3f))
  );
}

function validateAttribute(attribute) {
  const text = String(attribute ?? '');
  if (!ATTRIBUTE_TYPE.test(text)) {
    throw protocolError(`invalid attribute type "${text}"`, { attribute: text });
  }
  return text;
}

export function escapeFilterValue(value) {
  if (value === null || value === undefined) return '';
  const text = String(value);
  let result = '';
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index);
    const character = text[index];
    if (code === 0x5c) {
      result += '\\5c';
    } else if (code === 0x2a) {
      result += '\\2a';
    } else if (code === 0x28) {
      result += '\\28';
    } else if (code === 0x29) {
      result += '\\29';
    } else if (code <= 0x1f || code === 0x7f) {
      result += escapeByte(code);
    } else if (code >= 0xd800 && code <= 0xdbff) {
      const next = text.charCodeAt(index + 1);
      if (next >= 0xdc00 && next <= 0xdfff) {
        result += character + text[index + 1];
        index += 1;
      } else {
        result += escapeLoneSurrogate(code);
      }
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      result += escapeLoneSurrogate(code);
    } else {
      result += character;
    }
  }
  return result;
}

export function buildUserFilter(username) {
  return `(uid=${escapeFilterValue(username)})`;
}

export function buildOuFilter(ou) {
  return `(ou=${escapeFilterValue(ou)})`;
}

export function buildEqualityFilter(attribute, value) {
  return `(${validateAttribute(attribute)}=${escapeFilterValue(value)})`;
}

export function buildPresentFilter(attribute) {
  return `(${validateAttribute(attribute)}=*)`;
}
