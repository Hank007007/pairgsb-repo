import { invalidDn } from './errors.js';

const ATTRIBUTE_TYPE = /^[A-Za-z][A-Za-z0-9-]*(;[A-Za-z0-9-]+)*$/;
const HEX = /^[0-9a-fA-F]{2}$/;
const RESERVED = new Set([',', '+', '"', '\\', '<', '>', ';', '=']);

function splitUnescaped(text, separator) {
  const parts = [];
  let buffer = '';
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (ch === '\\') {
      buffer += ch;
      if (i + 1 < text.length) {
        buffer += text[i + 1];
        i += 1;
      }
      continue;
    }
    if (ch === separator) {
      parts.push(buffer);
      buffer = '';
      continue;
    }
    buffer += ch;
  }
  parts.push(buffer);
  return parts;
}

function unescapeValue(raw) {
  let result = '';
  const escapedFlags = [];
  for (let i = 0; i < raw.length; i += 1) {
    const ch = raw[i];
    if (ch === '\\') {
      if (i + 2 < raw.length && HEX.test(raw.slice(i + 1, i + 3))) {
        result += String.fromCharCode(Number.parseInt(raw.slice(i + 1, i + 3), 16));
        escapedFlags.push(true);
        i += 2;
        continue;
      }
      if (i + 1 < raw.length) {
        result += raw[i + 1];
        escapedFlags.push(true);
        i += 1;
        continue;
      }
      throw invalidDn('dangling escape in DN value', { value: raw });
    }
    if (RESERVED.has(ch)) {
      throw invalidDn(`unescaped "${ch}" in DN value`, { value: raw, character: ch });
    }
    result += ch;
    escapedFlags.push(false);
  }
  if (result === '') {
    return result;
  }
  if (result.startsWith(' ') && !escapedFlags[0]) {
    throw invalidDn('unescaped leading space in DN value', { value: raw });
  }
  const last = result.length - 1;
  if (result.endsWith(' ') && !escapedFlags[last]) {
    throw invalidDn('unescaped trailing space in DN value', { value: raw });
  }
  if (result.startsWith('#') && !escapedFlags[0]) {
    throw invalidDn('unescaped leading "#" in DN value', { value: raw });
  }
  return result;
}

function parseRdn(rdnText) {
  if (rdnText === '') {
    throw invalidDn('empty RDN in DN');
  }
  return splitUnescaped(rdnText, '+').map((avaText) => {
    const pieces = splitUnescaped(avaText, '=');
    if (pieces.length < 2) {
      throw invalidDn('RDN without "="', { rdn: rdnText });
    }
    const type = pieces[0];
    if (!ATTRIBUTE_TYPE.test(type)) {
      throw invalidDn(`invalid attribute type "${type}"`, { rdn: rdnText });
    }
    const rawValue = pieces.slice(1).join('=');
    return { type: type.toLowerCase(), value: unescapeValue(rawValue) };
  });
}

export function parseDN(dn) {
  const text = String(dn ?? '');
  if (text === '') return [];
  return splitUnescaped(text, ',').map(parseRdn);
}

export function normalizeDN(dn) {
  return parseDN(dn).map((rdn) =>
    rdn.map((ava) => ({ type: ava.type, value: ava.value.toLowerCase() })),
  );
}

function sameRdn(left, right) {
  if (left.length !== right.length) return false;
  return left.every((ava) => {
    const peer = right.find((candidate) => candidate.type === ava.type);
    return peer !== undefined && peer.value === ava.value;
  });
}

export function dnEquals(left, right) {
  const a = normalizeDN(left);
  const b = normalizeDN(right);
  if (a.length !== b.length) return false;
  return a.every((rdn, index) => sameRdn(rdn, b[index]));
}

export function isDescendantDN(candidate, base) {
  const entryRdns = normalizeDN(candidate);
  const baseRdns = normalizeDN(base);
  if (baseRdns.length > entryRdns.length) return false;
  const offset = entryRdns.length - baseRdns.length;
  return baseRdns.every((rdn, index) => sameRdn(rdn, entryRdns[offset + index]));
}

export function isDirectChildDN(candidate, base) {
  const entryRdns = normalizeDN(candidate);
  const baseRdns = normalizeDN(base);
  if (entryRdns.length !== baseRdns.length + 1) return false;
  return baseRdns.every((rdn, index) => sameRdn(rdn, entryRdns[index + 1]));
}

export function parentDN(dn) {
  const rdns = normalizeDN(dn);
  if (rdns.length <= 1) return '';
  return rdns
    .slice(1)
    .map((rdn) => rdn.map((ava) => `${ava.type}=${ava.value}`).join('+'))
    .join(',');
}
