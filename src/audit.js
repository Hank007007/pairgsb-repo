/**
 * Audit trail and redaction.
 *
 * Audit records are what operators get to see: the request method, the URL, the
 * headers that went on the wire, the credential id, and the outcome. They are
 * intended to be shipped to a log sink.
 */

export const REDACTED = '[REDACTED]';

const SENSITIVE_HEADERS = ['authorization', 'x-api-key'];

/**
 * Replace credential values in a header map.
 *
 * @param {Record<string, string>} headers
 * @returns {Record<string, string>}
 */
export function redactHeaders(headers) {
  const out = {};
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (SENSITIVE_HEADERS.includes(key)) {
      out[key] = REDACTED;
      continue;
    }
    out[key] = String(value);
  }
  return out;
}

/**
 * Normalize a URL for the audit trail. Embedded userinfo is dropped.
 *
 * @param {string} url
 * @returns {string}
 */
export function redactUrl(url) {
  if (typeof url !== 'string') {
    return url;
  }
  try {
    const parsed = new URL(url);
    if (parsed.username || parsed.password) {
      parsed.username = '';
      parsed.password = '';
    }
    return parsed.toString();
  } catch {
    return url;
  }
}

/**
 * Apply every redaction pass to an arbitrary record. Used before a record is
 * handed to the sink.
 *
 * @param {object} record
 * @returns {object}
 */
export function redactRecord(record) {
  return {
    ...record,
    url: typeof record.url === 'string' ? redactUrl(record.url) : record.url,
    headers: redactHeaders(record.headers),
    redirectFrom: typeof record.redirectFrom === 'string' ? redactUrl(record.redirectFrom) : record.redirectFrom,
  };
}

/**
 * Create an audit sink with a monotonic sequence number. Records are redacted
 * on the way in. A throwing `sink` callback is reported on stderr and never
 * propagates into the request path.
 *
 * @param {object} [options]
 * @param {(record: object) => void} [options.sink]
 * @param {number} [options.capacity]
 * @param {() => Date} [options.now]
 */
export function createAuditLog(options = {}) {
  const {
    sink = null,
    capacity = 1000,
    now = () => new Date(),
  } = options;

  const records = [];
  let sequence = 0;

  function write(record) {
    const enriched = { seq: ++sequence, at: now().toISOString(), ...record };
    let redacted;
    try {
      redacted = redactRecord(enriched);
    } catch (error) {
      redacted = { seq: enriched.seq, at: enriched.at, type: 'audit-error', message: String(error?.message ?? error) };
    }
    records.push(redacted);
    if (records.length > capacity) {
      records.shift();
    }
    if (typeof sink === 'function') {
      try {
        sink(redacted);
      } catch (error) {
        process.stderr.write(`credmux audit sink failed: ${error?.message ?? error}\n`);
      }
    }
    return redacted;
  }

  return {
    write,
    records() {
      return records.slice();
    },
    size() {
      return records.length;
    },
  };
}
