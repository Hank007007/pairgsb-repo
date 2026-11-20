'use strict';

const { MultipartError } = require('./errors');

// Path separators per policy 5.2: solidus, reverse solidus, division slash,
// fullwidth solidus.
const SEPARATOR_CLASS = '/\\\\\u2215\uff0f';
const LEADING_SEPARATOR = new RegExp('^[' + SEPARATOR_CLASS + ']');
const SEGMENT_SPLIT = new RegExp('[' + SEPARATOR_CLASS + ']+');
const CONTROL_CHARS = /[\x00-\x1f\x7f]/;
const DRIVE_PREFIX = /^[A-Za-z]:/;
const DOTS_ONLY = /^\.+$/;
const TRAILING_DOTS_SPACES = /[. ]+$/;

const RESERVED_NAMES = new Set([
  'CON',
  'PRN',
  'AUX',
  'NUL',
  'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
  'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
]);

function invalid(message) {
  return new MultipartError(400, 'INVALID_FILENAME', message);
}

/**
 * Turn a client-supplied filename into a path that is safe to join onto a
 * tenant directory.
 *
 * The name must already be percent-decoded. Returns the normalised relative
 * path on success. Throws a MultipartError carrying the reason when the name
 * cannot be used; no replacement name is invented.
 */
function safeRelPath(name) {
  if (name === undefined || name === null) {
    throw invalid('filename is missing');
  }

  const value = String(name);

  if (value === '') {
    throw invalid('filename is empty');
  }
  if (CONTROL_CHARS.test(value)) {
    throw invalid('filename contains control characters');
  }
  if (LEADING_SEPARATOR.test(value)) {
    throw invalid('filename is an absolute path');
  }
  if (DRIVE_PREFIX.test(value)) {
    throw invalid('filename begins with a drive letter');
  }

  const segments = value.split(SEGMENT_SPLIT).filter((segment) => segment !== '');
  if (segments.length === 0) {
    throw invalid('filename consists only of path separators');
  }

  const kept = [];
  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];

    if (segment === '..') {
      throw invalid('filename contains a parent directory segment');
    }
    if (DOTS_ONLY.test(segment)) {
      if (segment === '.' && i < segments.length - 1) {
        continue;
      }
      throw invalid('filename contains a dot-only segment');
    }
    kept.push(segment);
  }

  if (kept.length === 0) {
    throw invalid('filename has no usable path segment');
  }

  for (const segment of kept) {
    const base = segment.replace(TRAILING_DOTS_SPACES, '').split('.')[0].toUpperCase();
    if (RESERVED_NAMES.has(base)) {
      throw invalid(`filename uses a reserved device name: ${segment}`);
    }
  }

  return kept.join('/');
}

module.exports = { safeRelPath };
