/**
 * Query-string and object-key canonicalization helpers.
 *
 * Both the signer and the verifier go through these helpers so that the
 * canonical request they hash is produced by the same code path.
 */

const SEAL_PREFIX = 'X-Seal-';
const SIG_PARAM = 'sig';

/**
 * Normalize any supported query input into decoded [name, value] pairs.
 * Duplicate names are preserved; callers that need an object view can use
 * parseQuery().
 */
export function parseQueryPairs(input) {
  if (input === undefined || input === null || input === '') return [];
  if (input instanceof URLSearchParams) return [...input];
  if (typeof input === 'string') return [...new URLSearchParams(input)];

  if (Array.isArray(input)) {
    return input.map((pair) => {
      if (!Array.isArray(pair) || pair.length < 2) {
        throw new TypeError('query pairs must be [name, value] arrays');
      }
      return [String(pair[0]), String(pair[1])];
    });
  }

  if (input instanceof Map) {
    return [...input.entries()].map(([name, value]) => [String(name), String(value)]);
  }

  if (typeof input === 'object') {
    const pairs = [];
    for (const [name, value] of Object.entries(input)) {
      if (Array.isArray(value)) {
        for (const item of value) pairs.push([name, String(item)]);
      } else {
        pairs.push([name, String(value)]);
      }
    }
    return pairs;
  }

  throw new TypeError('unsupported query input');
}

/**
 * Turn a query string into a plain object of decoded name/value pairs.
 *
 * This object view cannot represent repeated names. Security-sensitive
 * callers should use parseQueryPairs().
 */
export function parseQuery(input) {
  const out = {};
  for (const [name, value] of parseQueryPairs(input)) {
    out[name] = value;
  }
  return out;
}

function isProtocolParam(name) {
  return name === SIG_PARAM || name.startsWith(SEAL_PREFIX);
}

/**
 * Drop the protocol parameters from decoded pairs, leaving only the
 * caller-supplied query pairs that are covered by the signature.
 */
export function stripProtocolPairs(input) {
  return parseQueryPairs(input).filter(([name]) => !isProtocolParam(name));
}

/**
 * Drop the protocol parameters from a query object, leaving only the
 * caller-supplied query pairs that are covered by the signature.
 */
export function stripProtocolParams(query) {
  const out = {};
  for (const [name, value] of Object.entries(query ?? {})) {
    if (isProtocolParam(name)) continue;
    out[name] = value;
  }
  return out;
}

function compareUtf8(a, b) {
  return Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

function encodeRfc3986(value) {
  return encodeURIComponent(value).replace(
    /[!'()*]/g,
    (char) => `%${char.charCodeAt(0).toString(16).toUpperCase()}`,
  );
}

/**
 * Canonical form of the caller-supplied query.
 *
 * Pairs are decoded, ordered by decoded name then decoded value (byte-wise
 * over UTF-8), and re-encoded with RFC 3986 percent-encoding. Repeated
 * parameters are preserved. An empty query canonicalizes to the empty string.
 */
export function canonicalQuery(query) {
  const pairs = parseQueryPairs(query)
    .map(([name, value]) => [String(name), String(value)])
    .sort(([nameA, valueA], [nameB, valueB]) => (
      compareUtf8(nameA, nameB) || compareUtf8(valueA, valueB)
    ));

  return pairs
    .map(([name, value]) => `${encodeRfc3986(name)}=${encodeRfc3986(value)}`)
    .join('&');
}

/**
 * Canonical form of the object key.
 *
 * Invalid keys are rejected rather than normalized: empty segments (leading,
 * trailing or doubled slashes) and "." / ".." segments are not allowed.
 */
export function canonicalKey(key) {
  if (typeof key !== 'string') {
    throw new TypeError('key must be a string');
  }
  const segments = key.split('/');
  for (const segment of segments) {
    if (segment === '' || segment === '.' || segment === '..') {
      throw new TypeError(`invalid object key: ${JSON.stringify(key)}`);
    }
  }
  return segments.join('/');
}

/**
 * Render a key for use inside the request path.
 */
export function encodePath(bucket, key) {
  const segments = String(key).split('/').map((segment) => encodeURIComponent(segment));
  return `/${encodeURIComponent(bucket)}/${segments.join('/')}`;
}
