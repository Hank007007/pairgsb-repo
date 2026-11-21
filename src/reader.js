'use strict';

const fs = require('node:fs');

const REQUIRED_FIELDS = ['seq', 'ts', 'payload'];

/**
 * Split raw text into complete newline-terminated lines and the trailing
 * fragment that has not been terminated yet.
 *
 * Returns byte counts as well so callers can advance a byte offset.
 */
function splitCompleteLines(text) {
  const lines = [];
  let consumed = 0;
  let start = 0;
  for (let i = 0; i < text.length; i += 1) {
    if (text.charCodeAt(i) === 10) {
      const raw = text.slice(start, i);
      const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw;
      if (line.length > 0) {
        lines.push(line);
      }
      start = i + 1;
      consumed = start;
    }
  }
  return { lines, consumed, rest: text.slice(start) };
}

function normaliseTs(value) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  throw new Error(`event ts is not a valid timestamp: ${JSON.stringify(value)}`);
}

/**
 * Parse one complete source line. A line that is not valid JSON, or that is
 * missing a required field, raises an error carrying the line number.
 */
function parseEvent(line, lineNumber) {
  let raw;
  try {
    raw = JSON.parse(line);
  } catch (err) {
    const error = new Error(`malformed source line ${lineNumber}: ${err.message}`);
    error.code = 'E_MALFORMED_LINE';
    error.lineNumber = lineNumber;
    throw error;
  }

  if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) {
    const error = new Error(`malformed source line ${lineNumber}: not an object`);
    error.code = 'E_MALFORMED_LINE';
    error.lineNumber = lineNumber;
    throw error;
  }

  for (const field of REQUIRED_FIELDS) {
    if (!(field in raw)) {
      const error = new Error(`malformed source line ${lineNumber}: missing ${field}`);
      error.code = 'E_MALFORMED_LINE';
      error.lineNumber = lineNumber;
      throw error;
    }
  }

  return {
    seq: raw.seq,
    ts: normaliseTs(raw.ts),
    prev_hash: raw.prev_hash === undefined ? null : raw.prev_hash,
    payload: raw.payload,
    meta: raw.meta === undefined ? null : raw.meta,
  };
}

class SourceReader {
  constructor(filePath, options = {}) {
    this.filePath = filePath;
    this.fs = options.fs || fs;
  }

  /**
   * Read from `offset` to the end of the file. Only complete lines are parsed;
   * the trailing fragment is reported back and is not consumed.
   */
  readFrom(offset = 0) {
    let buffer;
    try {
      buffer = this.fs.readFileSync(this.filePath);
    } catch (err) {
      if (err.code === 'ENOENT') {
        return { events: [], offset, pending: '', bytesRead: 0 };
      }
      throw err;
    }

    const slice = buffer.subarray(offset);
    const text = slice.toString('utf8');
    const { lines, consumed, rest } = splitCompleteLines(text);

    const events = lines.map((line, index) => parseEvent(line, index + 1));

    return {
      events,
      offset: offset + Buffer.byteLength(text.slice(0, consumed), 'utf8'),
      pending: rest,
      bytesRead: consumed,
    };
  }
}

module.exports = {
  SourceReader,
  splitCompleteLines,
  parseEvent,
  normaliseTs,
};
