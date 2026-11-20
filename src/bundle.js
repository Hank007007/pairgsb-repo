import { readFile } from 'node:fs/promises';
import { watch } from 'node:fs';
import { BundleError } from './errors.js';

const SUPPORTED_SCHEMES = new Set(['bearer', 'api-key']);

/**
 * Normalize a routing host for storage: lower-case, keep the port if present,
 * drop a single trailing root dot from the hostname portion.
 *
 * @param {string} raw
 * @returns {string}
 */
export function normalizeHost(raw) {
  if (typeof raw !== 'string' || raw.length === 0) {
    throw new BundleError('secret entry host must be a non-empty string');
  }
  const lower = raw.trim().toLowerCase();
  if (lower.includes('/') || lower.includes('?') || lower.includes('#') || lower.includes('@')) {
    throw new BundleError(`secret entry host must not contain scheme, path, query or userinfo: ${raw}`);
  }
  if (lower.includes('*')) {
    throw new BundleError(`wildcard hosts are not supported: ${raw}`);
  }
  const { hostname, port } = splitHostPort(lower);
  return port === null ? hostname : `${hostname}:${port}`;
}

/**
 * Split `host[:port]`, understanding bracketed IPv6 literals.
 *
 * @param {string} value
 * @returns {{ hostname: string, port: string | null }}
 */
export function splitHostPort(value) {
  if (value.startsWith('[')) {
    const close = value.indexOf(']');
    if (close === -1) {
      throw new BundleError(`malformed IPv6 host: ${value}`);
    }
    const hostname = normalizeHostname(value.slice(0, close + 1));
    const rest = value.slice(close + 1);
    if (rest === '') {
      return { hostname, port: null };
    }
    if (!rest.startsWith(':')) {
      throw new BundleError(`malformed host: ${value}`);
    }
    return { hostname, port: validatePort(rest.slice(1), value) };
  }
  const first = value.indexOf(':');
  if (first === -1) {
    return { hostname: stripRootDot(value.toLowerCase()), port: null };
  }
  const last = value.lastIndexOf(':');
  if (first !== last) {
    // Multiple colons and no brackets: a bare IPv6 literal without a port.
    return { hostname: normalizeHostname(`[${value}]`), port: null };
  }
  return {
    hostname: normalizeHostname(value.slice(0, first)),
    port: validatePort(value.slice(first + 1), value),
  };
}

function normalizeHostname(hostname) {
  if (hostname.startsWith('[')) {
    try {
      return new URL(`http://${hostname}/`).hostname.toLowerCase();
    } catch {
      return hostname.toLowerCase();
    }
  }
  return stripRootDot(hostname.toLowerCase());
}

function stripRootDot(hostname) {
  return hostname.endsWith('.') ? hostname.slice(0, -1) : hostname;
}

function validatePort(port, original) {
  if (!/^\d+$/.test(port)) {
    throw new BundleError(`malformed port in host: ${original}`);
  }
  const numeric = Number(port);
  if (numeric < 1 || numeric > 65535) {
    throw new BundleError(`port out of range in host: ${original}`);
  }
  return String(numeric);
}

/**
 * Parse an optional RFC 3339 timestamp. Returns `null` when absent, a Date when
 * present and valid.
 *
 * @param {unknown} value
 * @param {string} field
 * @param {string} id
 * @returns {Date | null}
 */
export function parseInstant(value, field, id) {
  if (value === undefined || value === null) {
    return null;
  }
  if (typeof value !== 'string') {
    throw new BundleError(`secret "${id}": ${field} must be an RFC 3339 string`);
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    throw new BundleError(`secret "${id}": ${field} is not a valid timestamp`);
  }
  return date;
}

/**
 * Validate and normalize one raw bundle entry.
 *
 * @param {unknown} raw
 * @returns {{ id: string, host: string, scheme: string, value: string, notBefore: Date | null, expiresAt: Date | null }}
 */
export function normalizeEntry(raw) {
  if (raw === null || typeof raw !== 'object') {
    throw new BundleError('secret entry must be an object');
  }
  const { id, host, scheme, value, not_before: notBeforeRaw, expires_at: expiresAtRaw } = raw;

  if (typeof id !== 'string' || id.length === 0) {
    throw new BundleError('secret entry id must be a non-empty string');
  }
  if (typeof scheme !== 'string' || !SUPPORTED_SCHEMES.has(scheme)) {
    throw new BundleError(`secret "${id}": unsupported scheme ${JSON.stringify(scheme)}`);
  }
  if (typeof value !== 'string' || value.length === 0) {
    throw new BundleError(`secret "${id}": value must be a non-empty string`);
  }

  return Object.freeze({
    id,
    host: normalizeHost(host),
    scheme,
    value,
    notBefore: parseInstant(notBeforeRaw, 'not_before', id),
    expiresAt: parseInstant(expiresAtRaw, 'expires_at', id),
  });
}

/**
 * Turn a parsed bundle document into an immutable revision.
 *
 * @param {unknown} document
 * @param {number} revision
 * @returns {{ revision: number, entries: ReadonlyArray<object>, loadedAt: number, source: string | null }}
 */
export function compileBundle(document, revision, source = null) {
  if (document === null || typeof document !== 'object' || Array.isArray(document)) {
    throw new BundleError('bundle must be a JSON object');
  }
  const secrets = document.secrets;
  if (!Array.isArray(secrets)) {
    throw new BundleError('bundle must contain a "secrets" array');
  }

  const entries = [];
  const seenIds = new Set();
  const seenRoutes = new Set();

  for (const raw of secrets) {
    const entry = normalizeEntry(raw);
    if (seenIds.has(entry.id)) {
      throw new BundleError(`duplicate secret id "${entry.id}"`);
    }
    const route = `${entry.host}\u0000${entry.scheme}`;
    if (seenRoutes.has(route)) {
      throw new BundleError(`ambiguous bundle: more than one ${entry.scheme} entry for host ${entry.host}`);
    }
    seenIds.add(entry.id);
    seenRoutes.add(route);
    entries.push(entry);
  }

  return Object.freeze({
    revision,
    entries: Object.freeze(entries),
    loadedAt: Date.now(),
    source,
  });
}

/**
 * Parse bundle text (JSON) into a revision.
 *
 * @param {string} text
 * @param {number} revision
 * @param {string | null} source
 */
export function parseBundle(text, revision, source = null) {
  let document;
  try {
    document = JSON.parse(text);
  } catch (cause) {
    throw new BundleError(`bundle is not valid JSON: ${cause.message}`, { cause });
  }
  return compileBundle(document, revision, source);
}

/**
 * Load a bundle from disk. `revision` advances monotonically with each
 * successful load; the caller decides which revision is active.
 *
 * @param {string} path
 * @param {number} revision
 * @param {(path: string) => Promise<string>} [reader]
 */
export async function loadBundle(path, revision, reader = readFileText) {
  const text = await reader(path);
  return parseBundle(text, revision, path);
}

function readFileText(path) {
  return readFile(path, 'utf8');
}

/**
 * Build a snapshot holder. `replace` swaps the active revision atomically;
 * readers always observe one complete revision.
 *
 * @param {object} initial
 */
export function createBundleStore(initial) {
  let active = initial;
  const listeners = new Set();

  return {
    get current() {
      return active;
    },
    replace(next) {
      const previous = active;
      active = next;
      for (const listener of listeners) {
        try {
          listener(next, previous);
        } catch {
          // listener failures must not corrupt the store
        }
      }
      return previous;
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

/**
 * Hot-reload a bundle file. Watches the file's directory so that editors that
 * replace the file atomically still trigger a reload, debounces rapid writes,
 * and keeps the previous revision if a load fails.
 *
 * @param {object} options
 * @param {string} options.path
 * @param {object} options.store
 * @param {(path: string) => Promise<string>} [options.reader]
 * @param {(error: Error) => void} [options.onError]
 * @param {number} [options.debounceMs]
 * @returns {{ close: () => void, reload: () => Promise<object> }}
 */
export function watchBundle(options) {
  const {
    path,
    store,
    reader = readFileText,
    onError = () => {},
    debounceMs = 50,
  } = options;

  let revision = store.current.revision;
  let closed = false;
  let inFlight = null;
  let timer = null;

  async function reload() {
    if (closed) {
      throw new BundleError('bundle watcher is closed');
    }
    if (inFlight) {
      return inFlight;
    }
    inFlight = (async () => {
      try {
        const next = await loadBundle(path, revision + 1, reader);
        revision += 1;
        store.replace(next);
        return next;
      } finally {
        inFlight = null;
      }
    })();
    return inFlight;
  }

  function schedule() {
    if (closed) return;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      reload().catch(onError);
    }, debounceMs);
    if (typeof timer.unref === 'function') timer.unref();
  }

  let watcher = null;
  try {
    watcher = watch(path, { persistent: false, interval: 250 }, schedule);
    watcher.on('error', onError);
  } catch (error) {
    onError(error);
  }

  return {
    reload,
    close() {
      if (closed) return;
      closed = true;
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
      if (watcher) {
        watcher.close();
        watcher = null;
      }
    },
  };
}
