/**
 * Credential selection and header injection.
 */

export const HEADER_FOR_SCHEME = Object.freeze({
  bearer: 'Authorization',
  'api-key': 'X-Api-Key',
});

const DEFAULT_PORT = Object.freeze({
  'http:': '80',
  'https:': '443',
});

/**
 * Compute the routing host for a URL: lower-case hostname plus an explicit
 * port when the URL's port is not the scheme default.
 *
 * @param {URL} url
 * @returns {string}
 */
export function effectiveHost(url) {
  const hostname = url.hostname.toLowerCase();
  const defaultPort = DEFAULT_PORT[url.protocol];
  const port = url.port === '' ? null : String(Number(url.port));
  if (port === null || port === defaultPort) {
    return hostname;
  }
  return `${hostname}:${port}`;
}

/**
 * Pick the bundle entry that supplies the credential for `url`.
 *
 * Entries are examined most-specific-first so that a rule carrying an explicit
 * port takes precedence over a portless rule.
 *
 * @param {{ entries: ReadonlyArray<object> }} bundle
 * @param {URL | string} target
 * @returns {object | null}
 */
export function selectEntry(bundle, target) {
  const url = target instanceof URL ? target : new URL(target);
  const host = effectiveHost(url);
  if (!bundle || !Array.isArray(bundle.entries)) {
    return null;
  }

  let selected = null;
  for (const entry of bundle.entries) {
    if (host.includes(entry.host)) {
      if (selected === null || entry.host.length > selected.host.length) {
        selected = entry;
      }
    }
  }
  return selected;
}

/**
 * Build the credential header for an entry.
 *
 * @param {object} entry
 * @returns {{ name: string, value: string }}
 */
export function credentialHeader(entry) {
  const name = HEADER_FOR_SCHEME[entry.scheme];
  if (!name) {
    throw new Error(`unsupported scheme: ${entry.scheme}`);
  }
  const value = entry.scheme === 'bearer' ? `Bearer ${entry.value}` : entry.value;
  return { name, value };
}

/**
 * Return a copy of `headers` with any existing credential header (compared
 * case-insensitively) replaced by `entry`'s header.
 *
 * @param {Record<string, string>} headers
 * @param {object} entry
 * @returns {Record<string, string>}
 */
export function applyCredential(headers, entry) {
  const { name, value } = credentialHeader(entry);
  const result = {};
  for (const [key, existing] of Object.entries(headers ?? {})) {
    if (key.toLowerCase() === name.toLowerCase()) continue;
    result[key] = existing;
  }
  result[name] = value;
  return result;
}

/**
 * Select an entry for `url` and return the headers to send, plus the entry that
 * was used (or `null` when no credential applies).
 *
 * @param {{ entries: ReadonlyArray<object> }} bundle
 * @param {URL | string} target
 * @param {Record<string, string>} [headers]
 * @returns {{ entry: object | null, headers: Record<string, string> }}
 */
export function injectIntoRequest(bundle, target, headers = {}) {
  const url = target instanceof URL ? target : new URL(target);
  const entry = selectEntry(bundle, url);
  if (entry === null) {
    return { entry: null, headers: { ...headers } };
  }
  return { entry, headers: applyCredential(headers, entry) };
}
