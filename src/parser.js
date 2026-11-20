'use strict';

const { MultipartError } = require('./errors');
const { parsePartHeaders, parseDispositionParams } = require('./headers');
const { safeRelPath } = require('./filename');
const { resolveLimits } = require('./limits');

const HEADER_END = Buffer.from('\r\n\r\n');
const DEFAULT_BOUNDARY = 'partfence-boundary-7MA4YWxkTrZu0gW';

// RFC 2046 bcharsnospace plus space (policy 7.2).
const BOUNDARY_CHARS = /^[0-9A-Za-z'()+_,\-.\/:=? ]+$/;

const STATE_PREAMBLE = 0;
const STATE_HEADERS = 1;
const STATE_BODY = 2;
const STATE_CLOSED = 3;

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
  return match[1] !== undefined ? match[1] : match[2];
}

/**
 * Validate a boundary against policy 7: 1-70 characters from the RFC 2046
 * bcharsnospace set plus space, never beginning or ending with a space.
 * Returns a MultipartError when invalid, null otherwise.
 */
function validateBoundary(boundary) {
  if (typeof boundary !== 'string' || boundary.length === 0 || boundary.length > 70) {
    return new MultipartError(400, 'INVALID_BOUNDARY', 'boundary must be 1 to 70 characters long');
  }
  if (
    !BOUNDARY_CHARS.test(boundary) ||
    boundary[0] === ' ' ||
    boundary[boundary.length - 1] === ' '
  ) {
    return new MultipartError(
      400,
      'INVALID_BOUNDARY',
      'boundary contains characters outside the RFC 2046 set'
    );
  }
  return null;
}

/**
 * Streaming multipart/form-data parser.
 *
 * Feed raw request bytes to `write()` in any chunking; call `end()` once the
 * request is complete. Completed parts are collected in `this.parts`. Any
 * rejection empties `this.parts`: there is no partial success.
 */
class MultipartParser {
  constructor(options) {
    const opts = options || {};
    this.boundary = opts.boundary !== undefined
      ? String(opts.boundary)
      : boundaryFromContentType(opts.contentType);
    this.limits = resolveLimits(opts.limits);
    this.delimiter = Buffer.from('--' + this.boundary, 'latin1');
    this.bodyDelimiter = Buffer.from('\r\n--' + this.boundary, 'latin1');

    this._boundaryError = validateBoundary(this.boundary);

    this.buffer = Buffer.alloc(0);
    this.parts = [];
    this.part = null;
    this.state = STATE_PREAMBLE;
    this.finished = false;

    this.bytesRead = 0;
    this.headerBytesRead = 0;
    this.partCount = 0;
  }

  get started() {
    return this.state !== STATE_PREAMBLE;
  }

  get closed() {
    return this.state === STATE_CLOSED;
  }

  write(chunk) {
    if (this.finished) {
      throw new MultipartError(400, 'REQUEST_ALREADY_ENDED', 'write() called after end()');
    }

    try {
      if (this._boundaryError) throw this._boundaryError;

      const data = toBuffer(chunk);
      this.bytesRead += data.length;
      if (this.bytesRead > this.limits.maxTotalBytes) {
        throw new MultipartError(
          413,
          'TOTAL_TOO_LARGE',
          `request exceeds ${this.limits.maxTotalBytes} bytes`
        );
      }

      // Anything after the closing delimiter is epilogue and is discarded.
      if (this.state === STATE_CLOSED) return;

      this.buffer = this.buffer.length === 0 ? data : Buffer.concat([this.buffer, data]);
      this._drain();
    } catch (err) {
      this._absorb(err);
      throw err;
    }
  }

  end() {
    if (this.finished) return this.parts;

    try {
      if (this._boundaryError) throw this._boundaryError;
      if (this.state === STATE_PREAMBLE) {
        throw new MultipartError(400, 'MISSING_BOUNDARY', 'request has no multipart body');
      }
      if (this.state !== STATE_CLOSED) {
        throw new MultipartError(400, 'TRUNCATED', 'request ended before the closing boundary');
      }
      this.finished = true;
      return this.parts;
    } catch (err) {
      this._absorb(err);
      throw err;
    }
  }

  // A rejected request produces an empty parts array and accepts no more input.
  _absorb(err) {
    if (err instanceof MultipartError) {
      this.parts = [];
      this.part = null;
      this.finished = true;
    }
  }

  _drain() {
    for (;;) {
      if (this.state === STATE_PREAMBLE) {
        const need = this.delimiter.length + 2;
        if (this.buffer.length < need) return;
        if (!this.buffer.subarray(0, this.delimiter.length).equals(this.delimiter)) {
          throw new MultipartError(
            400,
            'MISSING_BOUNDARY',
            'body does not start with the boundary delimiter'
          );
        }
        const tail = this.buffer.toString('latin1', this.delimiter.length, need);
        if (tail === '--') {
          this.state = STATE_CLOSED;
          this.buffer = Buffer.alloc(0);
          return;
        }
        if (tail !== '\r\n') {
          throw new MultipartError(400, 'MISSING_BOUNDARY', 'opening boundary delimiter is malformed');
        }
        this.buffer = this.buffer.subarray(need);
        this._openPart();
        this.state = STATE_HEADERS;
        continue;
      }

      if (this.state === STATE_HEADERS) {
        const headerEnd = this.buffer.indexOf(HEADER_END);
        if (headerEnd === -1) {
          if (this.headerBytesRead + this.buffer.length > this.limits.maxHeaderBytes) {
            throw new MultipartError(
              413,
              'HEADERS_TOO_LARGE',
              `part headers exceed ${this.limits.maxHeaderBytes} bytes`
            );
          }
          return;
        }
        const rawHeaders = this.buffer.subarray(0, headerEnd);
        this.buffer = this.buffer.subarray(headerEnd + HEADER_END.length);
        this.headerBytesRead += rawHeaders.length;
        if (this.headerBytesRead > this.limits.maxHeaderBytes) {
          throw new MultipartError(
            413,
            'HEADERS_TOO_LARGE',
            `part headers exceed ${this.limits.maxHeaderBytes} bytes`
          );
        }
        this._applyHeaders(rawHeaders.toString('utf8'));
        this.state = STATE_BODY;
        continue;
      }

      if (this.state === STATE_BODY) {
        const found = this._findDelimiter();
        if (found === null) {
          this._flushBody();
          return;
        }
        const [index, closing] = found;
        this._appendBody(this.buffer.subarray(0, index));
        if (closing) {
          this.buffer = Buffer.alloc(0);
          this.state = STATE_CLOSED;
          this._closePart();
          return;
        }
        this.buffer = this.buffer.subarray(index + this.bodyDelimiter.length + 2);
        this._closePart();
        this._openPart();
        this.state = STATE_HEADERS;
        continue;
      }

      return;
    }
  }

  /**
   * Locate the next `CRLF--boundary` that is a real delimiter: followed by
   * either `--` (closing) or CRLF (next part). A boundary substring not
   * preceded by CRLF, or followed by anything else, is body content.
   */
  _findDelimiter() {
    let from = 0;
    for (;;) {
      const index = this.buffer.indexOf(this.bodyDelimiter, from);
      if (index === -1) return null;
      const after = index + this.bodyDelimiter.length;
      if (this.buffer.length < after + 2) return null;
      const tail = this.buffer.toString('latin1', after, after + 2);
      if (tail === '--') return [index, true];
      if (tail === '\r\n') return [index, false];
      from = index + 1;
    }
  }

  // Flush bytes that cannot be part of a future delimiter into the part body.
  _flushBody() {
    const keep = this.bodyDelimiter.length + 2;
    if (this.buffer.length <= keep) return;
    const body = this.buffer.subarray(0, this.buffer.length - keep);
    this.buffer = this.buffer.subarray(this.buffer.length - keep);
    this._appendBody(body);
  }

  _appendBody(bytes) {
    if (!this.part || bytes.length === 0) return;
    this.part.bytes += bytes.length;
    if (this.part.bytes > this.limits.maxPartBytes) {
      throw new MultipartError(
        413,
        'PART_TOO_LARGE',
        `part "${this.part.name || ''}" exceeds ${this.limits.maxPartBytes} bytes`
      );
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
    if (this.parts.length >= this.limits.maxParts) {
      throw new MultipartError(
        413,
        'TOO_MANY_PARTS',
        `request has more than ${this.limits.maxParts} parts`
      );
    }
    this.part = {
      headers: null,
      type: null,
      name: '',
      filename: null,
      rawFilename: null,
      contentType: null,
      bytes: 0,
    };
  }

  _closePart() {
    const part = this.part;
    this.parts.push({
      name: part.name,
      filename: part.filename,
      contentType: part.contentType,
      size: part.bytes,
    });
    this.partCount = this.parts.length;
    this.part = null;
  }
}

module.exports = { MultipartParser, boundaryFromContentType };
