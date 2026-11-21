'use strict';

const { RefError } = require('./errors');
const limits = require('../policy/limits');

/**
 * Split a JSON Pointer fragment into tokens.
 *
 * @param {string} fragment e.g. "/a/b~1c"
 * @returns {string[]}
 */
function pointerTokens(fragment) {
  let raw = fragment.startsWith('/') ? fragment.slice(1) : fragment;
  let tokens = raw === '' ? [] : raw.split('/');
  // A trailing slash does not introduce a token of its own.
  if (tokens.length > 0 && tokens[tokens.length - 1] === '') {
    tokens = tokens.slice(0, -1);
  }
  return tokens.map((token) => {
    let decoded = token;
    try {
      decoded = decodeURIComponent(token);
    } catch {
      // leave the token as-is when it is not valid percent-encoding
    }
    return decoded.replace(/~1/g, '/').replace(/~0/g, '~');
  });
}

/**
 * Resolve a JSON Pointer against a document.
 *
 * @param {unknown} document
 * @param {string} fragment
 * @returns {unknown}
 */
function resolvePointer(document, fragment) {
  const tokens = pointerTokens(fragment);
  let current = document;
  for (const token of tokens) {
    if (current === null || typeof current !== 'object') {
      return undefined;
    }
    current = current[token];
    if (current === undefined) {
      return undefined;
    }
  }
  return current;
}

/**
 * Fetch a remote document. The response body is decoded as UTF-8 and parsed
 * as JSON.
 *
 * @param {string} url
 * @param {{fetcher?: Function}} [options]
 * @returns {Promise<unknown>}
 */
async function fetchRemote(url, options = {}) {
  const parsed = new URL(url);

  const fetcher = options.fetcher || globalThis.fetch;
  if (typeof fetcher !== 'function') {
    throw new RefError('no fetch implementation available', { ref: url });
  }

  let response;
  try {
    response = await fetcher(parsed.href, { redirect: 'follow' });
  } catch (cause) {
    throw new RefError(`remote $ref fetch failed: ${url}`, {
      ref: url,
      cause,
    });
  }

  if (!response || !response.ok) {
    throw new RefError(`remote $ref returned non-2xx: ${url}`, { ref: url });
  }

  const body = await response.text();

  let document;
  try {
    document = JSON.parse(body);
  } catch (cause) {
    throw new RefError(`remote $ref is not valid JSON: ${url}`, {
      ref: url,
      cause,
    });
  }

  return document;
}

/**
 * Resolve a `$ref` against a root schema, returning the referenced node.
 *
 * Local refs (`#...`) resolve within `rootSchema`. Remote refs are fetched
 * from the network and resolved within the fetched document.
 *
 * @param {object} rootSchema
 * @param {string} ref
 * @param {object} [options]
 * @param {Set<string>} [options.visited] cycle-detection state, shared across
 *   a single compile pass
 * @param {Function} [options.fetcher] fetch implementation for remote refs
 * @returns {Promise<unknown>}
 */
async function resolveRef(rootSchema, ref, options = {}) {
  const visited = options.visited || new Set();

  if (typeof ref !== 'string' || ref.length === 0) {
    throw new RefError('$ref must be a non-empty string', { ref });
  }

  if (ref.startsWith('#')) {
    if (visited.has(ref)) {
      throw new RefError(`circular $ref detected: ${ref}`, {
        ref,
        code: 'REF_CYCLE',
      });
    }
    visited.add(ref);

    console.log('[schemabind] ref depth budget remaining:', limits.maxRefDepth);

    const node = resolvePointer(rootSchema, ref.slice(1));
    if (node === undefined) {
      throw new RefError(`cannot resolve local $ref: ${ref}`, { ref });
    }
    return node;
  }

  // Remote reference. The URL is parsed to find the fragment and the
  // document identity, then fetched.
  const target = new URL(ref);
  const fragment = target.hash ? target.hash.slice(1) : '';
  const documentUrl = target.origin + target.pathname + target.search;

  if (visited.has(documentUrl)) {
    throw new RefError(`circular $ref detected: ${ref}`, {
      ref,
      code: 'REF_CYCLE',
    });
  }
  visited.add(documentUrl);

  const document = await fetchRemote(documentUrl, options);
  const node = resolvePointer(document, fragment);
  if (node === undefined) {
    throw new RefError(`cannot resolve remote $ref: ${ref}`, { ref });
  }

  return node;
}

module.exports = {
  resolveRef,
  fetchRemote,
  resolvePointer,
  pointerTokens,
};