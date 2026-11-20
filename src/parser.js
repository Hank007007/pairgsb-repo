'use strict';

const { MultipartError } = require('./errors');
const { parsePartHeaders, parseDispositionParams } = require('./headers');
const { safeRelPath } = require('./filename');
const { resolveLimits, accountBytes } = require('./limits');

const HEADER_END = Buffer.from('\r\n\r\n');
const DEFAULT_BOUNDARY = 'partfence-boundary-7MA4YWxkTrZu0gW';

function toBuffer(chunk) {
  if (Buffer.isBuffer(chunk)) return chunk;
  if (typeof chunk === 'string') return Buffer.from(chunk, 'utf8');
  if (chunk instanceof Uint8Array) {
    return Buffer.from(chunk.buffer, chunk.byteOffset, chunk.byteLength);
  }
  throw new TypeError('chunk must be a string, Buffer or Uint8Array');
}

/**
 * Pull the boundary out of a Content-Type header value.
 *
 * Falls back to the built-in boundary when the header does not name one.
 */
function boundaryFromContentType(contentType) {
  if (typeof contentType !== 'string') return DEFAULT_BOUNDARY;
  const match = /boundary=(?:"([^"]*)"|([^";]*))/i.exec(contentType);
  if (!match) return DEFAULT_BOUNDARY;
  const value = match[1] !== undefined ? match[1] : match[2];
  return value.trim();
}

/**
 * Streaming multipart/form-data parser.
 *
 * Feed raw request bytes to `write()` in any chunking; call `end()` once the
 * request is complete. Completed parts are collected in `this.parts`.
 */
class MultipartParser {
  constructor(options) {
    const opts = options || {};
    this.boundary = opts.boundary !== undefined
      ? String(opts.boundary)
      : boundaryFromContentType(opts.contentType);
    this.limits = resolveLimits(opts.limits);
    this.delimiter = Buffer.from('--' + this.boundary);

    this.buffer = Buffer.alloc(0);
    this.parts = [];
    this.part = null;
    this.started = false;
    this.closed = false;
    this.finished = false;

    this.bytesRead = 0;
    this.headerBytesRead = 0;
    this.partCount = 0;

    this.state = {
      part: null,
      limits: this.limits,
      totalBytes: 0,
      partCount: 0,
      headerBytes: 0,
    };

    this.trailer = null;
  }

  _flushTrailer() {
    if (this.trailer === null) return;
    const pending = this.trailer;
    if (pending.length === 0) return;

    let offset = 0;
    if (pending[0] === 0x0d) {
      if (pending.length < 2) return;
      if (pending[1] === 0x0a) offset = 2;
      else offset = 1;
    } else if (pending[0] === 0x0a) {
      offset = 1;
    }

    const rest = pending.subarray(offset);
    this.trailer = Buffer.alloc(0);
    if (rest.length > 0 && this.part) {
      this.part.chunks.push(rest);
      this.part.pending += rest.length;
    }
  }

  write(chunk) {
    if (this.finished) {
      throw new MultipartError(400, 'REQUEST_ALREADY_ENDED', 'write() called after end()');
    }

    const data = toBuffer(chunk);
    this.bytesRead += data.length;
    this.state.totalBytes = this.bytesRead;

    if (this.closed) {
      this.trailer = this.trailer === null
        ? data
        : Buffer.concat([this.trailer, data]);
      this._flushTrailer();
      return;
    }

    this.buffer = this.buffer.length === 0 ? data : Buffer.concat([this.buffer, data]);
    this._drain();
  }

  end() {
    if (this.finished) return this.parts;

    this._drain();

    if (!this.started) {
      throw new MultipartError(400, 'MISSING_BOUNDARY', 'request has no multipart body');
    }
    if (this.part) {
      if (!this.closed) {
        throw new MultipartError(400, 'TRUNCATED', 'request ended before the closing boundary');
      }
      this._flushTrailer();
      this._closePart();
    }

    this.finished = true;
    return this.parts;
  }

  _drain() {
    let progress = true;

    while (progress && !this.closed) {
      progress = false;

      if (!this.started) {
        const idx = this.buffer.indexOf(this.delimiter);
        if (idx === -1) return;

        const after = idx + this.delimiter.length;
        if (this.buffer.length < after + 2) return;

        const tail = this.buffer.slice(after, after + 2).toString('latin1');
        this.buffer = this.buffer.slice(after);
        this.started = true;

        if (tail === '--') {
          this.closed = true;
          this.buffer = Buffer.alloc(0);
          return;
        }

        this.buffer = this.buffer.slice(2);
        this._openPart();
        progress = true;
        continue;
      }

      if (this.part && this.part.headers === null) {
        const headerEnd = this.buffer.indexOf(HEADER_END);
        if (headerEnd === -1) return;

        const rawHeaders = this.buffer.slice(0, headerEnd);
        this.buffer = this.buffer.slice(headerEnd + HEADER_END.length);
        this.headerBytesRead += rawHeaders.length;
        this.state.headerBytes = this.headerBytesRead;

        this._applyHeaders(rawHeaders.toString('utf8'));
        progress = true;
        continue;
      }

      const idx = this.buffer.indexOf(this.delimiter);
      if (idx === -1) return;

      const after = idx + this.delimiter.length;
      if (this.buffer.length < after + 2) return;

      let bodyEnd = idx;
      if (
        bodyEnd >= 2 &&
        this.buffer[bodyEnd - 2] === 0x0d &&
        this.buffer[bodyEnd - 1] === 0x0a
      ) {
        bodyEnd -= 2;
      }

      const body = this.buffer.slice(0, bodyEnd);
      this.part.chunks.push(body);
      this.part.pending += body.length;

      const tail = this.buffer.slice(after, after + 2).toString('latin1');
      this.buffer = this.buffer.slice(after);

      if (tail === '--') {
        this.closed = true;
        this.trailer = this.buffer.slice(2);
        this.buffer = Buffer.alloc(0);
        return;
      }

      this.buffer = this.buffer.slice(2);
      this._closePart();
      this._openPart();
      progress = true;
    }
  }

  _applyHeaders(rawText) {
    const headers = parsePartHeaders(rawText);
    const disposition = headers['content-disposition'];

    if (disposition === undefined) {
      throw new MultipartError(
        400,
        'MISSING_DISPOSITION',
        'part is missing a Content-Disposition header'
      );
    }

    const parsed = parseDispositionParams(disposition);

    this.part.headers = headers;
    this.part.type = parsed.type;
    this.part.name = parsed.params.name !== undefined ? parsed.params.name : '';
    this.part.rawFilename = parsed.params.filename !== undefined
      ? parsed.params.filename
      : null;

    if (this.part.rawFilename !== null) {
      this.part.filename = safeRelPath(this.part.rawFilename);
    }

    const contentType = headers['content-type'];
    this.part.contentType = contentType !== undefined ? contentType : null;
  }

  _openPart() {
    this.part = {
      headers: null,
      type: null,
      name: '',
      filename: null,
      rawFilename: null,
      contentType: null,
      bytes: 0,
      pending: 0,
      chunks: [],
    };
  }

  _closePart() {
    const part = this.part;
    const size = part.pending;
    const record = {
      name: part.name,
      filename: part.filename,
      contentType: part.contentType,
      size,
    };

    this.parts.push(record);
    this.partCount = this.parts.length;
    this.state.part = part;
    this.state.partCount = this.partCount;
    accountBytes(this.state, size);

    this.part = null;
    this.state.part = null;
  }
}

module.exports = { MultipartParser, boundaryFromContentType };
