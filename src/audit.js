/**
 * Audit trail and redaction.
 *
 * Audit records are what operators get to see: the request method, the URL, the
 * headers that went on the wire, the credential id, and the outcome. They are
 * intended to be shipped to a log sink.
 */

export const REDACTED = '[REDACTED]';

const SENSITIVE_HEADERS = new Set(['authorization', 'x-api-key']);

const SENSITIVE_QUERY_PARAMETERS = new Set([
  'access_token',
  'apikey',
  'api_key',
  'api-key',
  'auth',
  'authorization',
  'bearer',
  'client_secret',
  'code',
  'id_token',
  'key',
  'password',
  'passwd',
  'refresh_token',
  'secret',
  'sig',
  'signature',
  'token',
]);

function normalizeSecretValues(input) {
  if (input === null || input === undefined) {
    return [];
  }
  let values;
  if (Array.isArray(input)) {
    values = input.map((item) => (typeof item === 'string' ? item : item?.value));
  } else if (Array.isArray(input.entries)) {
    values = input.entries.map((entry) => entry?.value);
  } else if (Array.isArray(input.secrets)) {
    values = input.secrets.map((entry) => entry?.value);
  } else {
    values = [];
  }
  return [...new Set(values.filter((value) => typeof value === 'string' && value.length > 0))]
    .sort((a, b) => b.length - a.length);
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function containsSecret(value, secretValues) {
  return secretValues.some((secret) => value.includes(secret));
}

/**
 * Replace every occurrence of a bundle value, including its percent-encoded
 * form, with the fixed placeholder.
 *
 * @param {string} value
 * @param {string[]} secretValues
 * @returns {string}
 */
function redactValue(value, secretValues) {
  let out = value;
  for (const secret of secretValues) {
    out = out.split(secret).join(REDACTED);
    const variants = new Set([
      encodeURIComponent(secret),
      encodeURI(secret),
      encodeURIComponent(secret).replace(/%20/g, '+'),
    ]);
    for (const encoded of variants) {
      if (encoded === secret) {
        continue;
      }
      const pattern = escapeRegex(encoded).replace(
        /%[0-9A-F]{2}/gi,
        () => '%[0-9A-Fa-f][0-9A-Fa-f]',
      );
      out = out.replace(new RegExp(pattern, 'g'), REDACTED);
    }
  }
  return out;
}

/**
 * Redact bundle values from free-form text such as an error message.
 *
 * @param {string} value
 * @param {string[]} secretValues
 * @returns {string}
 */
export function redactText(value, secretValues = []) {
  return redactValue(String(value), normalizeSecretValues(secretValues));
}

function safeDecode(value) {
  try {
    return decodeURIComponent(value.replace(/\+/g, ' '));
  } catch {
    return value;
  }
}

function redactMaybeEncoded(value, secretValues) {
  const decoded = safeDecode(value);
  if (decoded !== value && containsSecret(decoded, secretValues)) {
    return REDACTED;
  }
  return redactValue(value, secretValues);
}

/**
 * Redact known secret-bearing query parameters and any value embedded in the
 * raw query string.
 *
 * @param {string} search
 * @param {string[]} secretValues
 * @returns {string}
 */
function redactSearch(search, secretValues) {
  if (typeof search !== 'string' || search.length === 0) {
    return search;
  }
  const prefix = search.startsWith('?') ? '?' : '';
  const body = prefix === '?' ? search.slice(1) : search;
  const parts = body.split('&').map((part) => {
    const equals = part.indexOf('=');
    const rawKey = equals === -1 ? part : part.slice(0, equals);
    const rawValue = equals === -1 ? '' : part.slice(equals + 1);
    if (SENSITIVE_QUERY_PARAMETERS.has(safeDecode(rawKey).toLowerCase())) {
      return `${rawKey}=${REDACTED}`;
    }
    if (rawValue.length === 0) {
      return part;
    }
    return `${rawKey}=${redactMaybeEncoded(rawValue, secretValues)}`;
  });
  return `${prefix}${parts.join('&')}`;
}

/**
 * Replace credential values in a header map.
 *
 * @param {Record<string, string>} headers
 * @param {string[]} [secretValues]
 * @returns {Record<string, string>}
 */
export function redactHeaders(headers, secretValues = []) {
  const secrets = normalizeSecretValues(secretValues);
  const out = {};
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (SENSITIVE_HEADERS.has(key.toLowerCase())) {
      out[key] = REDACTED;
      continue;
    }
    out[key] = redactMaybeEncoded(String(value), secrets);
  }
  return out;
}

/**
 * Normalize a URL for the audit trail. Embedded userinfo is dropped.
 *
 * @param {string} url
 * @param {string[]} [secretValues]
 * @returns {string}
 */
export function redactUrl(url, secretValues = []) {
  if (typeof url !== 'string') {
    return url;
  }
  const secrets = normalizeSecretValues(secretValues);
  try {
    const parsed = new URL(url);
    if (parsed.username || parsed.password) {
      parsed.username = '';
      parsed.password = '';
    }
    parsed.search = redactSearch(parsed.search, secrets);
    parsed.pathname = parsed.pathname
      .split('/')
      .map((segment) => redactMaybeEncoded(segment, secrets))
      .join('/');
    return redactValue(parsed.toString(), secrets);
  } catch {
    return redactValue(url, secrets);
  }
}

/**
 * Apply every redaction pass to an arbitrary record. Used before a record is
 * handed to the sink.
 *
 * @param {object} record
 * @param {string[]} [secretValues]
 * @returns {object}
 */
export function redactRecord(record, secretValues = []) {
  const secrets = normalizeSecretValues(secretValues);
  const out = {};
  for (const [key, value] of Object.entries(record)) {
    if (key === 'headers') {
      out[key] = redactHeaders(value, secrets);
      continue;
    }
    if (key === 'url' || key === 'redirectFrom' || key === 'location' || key === 'from' || key === 'to') {
      out[key] = typeof value === 'string' ? redactUrl(value, secrets) : value;
      continue;
    }
    out[key] = redactDeep(value, secrets);
  }
  return out;
}

function redactDeep(value, secretValues) {
  if (typeof value === 'string') {
    return redactValue(value, secretValues);
  }
  if (Array.isArray(value)) {
    return value.map((item) => redactDeep(item, secretValues));
  }
  if (value !== null && typeof value === 'object') {
    const out = {};
    for (const [key, nested] of Object.entries(value)) {
      out[key] = redactDeep(nested, secretValues);
    }
    return out;
  }
  return value;
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
 * @param {string[]} [options.secretValues]
 * @param {() => string[]} [options.getSecretValues]
 */
export function createAuditLog(options = {}) {
  const {
    sink = null,
    capacity = 1000,
    now = () => new Date(),
    secretValues = [],
    getSecretValues = null,
  } = options;

  const records = [];
  let sequence = 0;

  function currentSecretValues(context) {
    const values = [...normalizeSecretValues(secretValues), ...normalizeSecretValues(context?.secretValues)];
    if (typeof getSecretValues === 'function') {
      try {
        values.push(...normalizeSecretValues(getSecretValues()));
      } catch {
        // A broken redaction source must not break auditing or the request.
      }
    }
    return normalizeSecretValues(values);
  }

  function write(record, context = {}) {
    const enriched = { seq: ++sequence, at: now().toISOString(), ...record };
    let redacted;
    try {
      redacted = redactRecord(enriched, currentSecretValues(context));
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
