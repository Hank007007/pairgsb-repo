'use strict';

/**
 * Whitelisted pure functions. Every entry takes already evaluated arguments
 * and returns a plain value. Functions never receive the event document and
 * never read from the process environment.
 */

const MAX_STRING_ARG = 4096;

function asString(value) {
  if (typeof value === 'string') return value;
  if (value === null || value === undefined) return '';
  return String(value);
}

function lower(value) {
  return asString(value).toLowerCase();
}

function upper(value) {
  return asString(value).toUpperCase();
}

function len(value) {
  if (Array.isArray(value)) return value.length;
  return asString(value).length;
}

function contains(haystack, needle) {
  return asString(haystack).includes(asString(needle));
}

function startsWith(value, prefix) {
  return asString(value).startsWith(asString(prefix));
}

function endsWith(value, suffix) {
  return asString(value).endsWith(asString(suffix));
}

function trim(value) {
  return asString(value).trim();
}

function coalesce(...values) {
  for (const value of values) {
    if (value !== null && value !== undefined) return value;
  }
  return null;
}

function int(value, radix = 10) {
  const base = radix === 16 ? 16 : 10;
  const parsed = typeof value === 'number'
    ? Math.trunc(value)
    : Number.parseInt(asString(value).trim(), base);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseIPv4(text) {
  const parts = text.split('.');
  if (parts.length !== 4) return null;
  let result = 0;
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return null;
    const value = Number(part);
    if (value > 255) return null;
    result = (result * 256) + value;
  }
  return result;
}

/**
 * Parse a CIDR block into a plain object so the `in` operator can test
 * containment without exposing any evaluator internals.
 */
function cidr(value) {
  const text = asString(value).trim();
  const slash = text.indexOf('/');
  if (slash === -1) {
    const base = parseIPv4(text);
    if (base === null) return null;
    return Object.freeze({ kind: 'cidr', base, mask: 0xffffffff, bits: 32 });
  }

  const base = parseIPv4(text.slice(0, slash));
  const bitsText = text.slice(slash + 1);
  if (base === null || !/^\d{1,2}$/.test(bitsText)) return null;
  const bits = Number(bitsText);
  if (bits > 32) return null;
  const mask = bits === 0 ? 0 : (0xffffffff << (32 - bits)) >>> 0;
  return Object.freeze({ kind: 'cidr', base: (base & mask) >>> 0, mask, bits });
}

function isCidr(value) {
  return Boolean(value) && typeof value === 'object' && value.kind === 'cidr';
}

const FUNCTIONS = Object.freeze({
  lower,
  upper,
  len,
  contains,
  startsWith,
  endsWith,
  trim,
  coalesce,
  int,
  cidr,
  isCidr,
});

function hasFunction(name) {
  return Object.prototype.hasOwnProperty.call(FUNCTIONS, name);
}

function callFunction(name, args) {
  const fn = FUNCTIONS[name];
  if (!fn) return null;
  return fn(...args.slice(0, 8).map(limitArgument));
}

function limitArgument(value) {
  if (typeof value === 'string' && value.length > MAX_STRING_ARG) {
    return value.slice(0, MAX_STRING_ARG);
  }
  return value;
}

module.exports = {
  FUNCTIONS,
  hasFunction,
  callFunction,
  isCidr,
};
