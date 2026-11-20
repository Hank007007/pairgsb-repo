/**
 * Redirect handling.
 *
 * `followRedirect` turns a redirect response into the next request to send.
 */

import { InsecureRedirectError } from './errors.js';

export const REDIRECT_STATUS = Object.freeze(new Set([301, 302, 303, 307, 308]));

/**
 * @param {number} status
 * @returns {boolean}
 */
export function isRedirectStatus(status) {
  return REDIRECT_STATUS.has(status);
}

/**
 * Resolve a `Location` header against the URL that produced it.
 *
 * @param {string} location
 * @param {string | URL} currentUrl
 * @returns {URL}
 */
export function resolveLocation(location, currentUrl) {
  return new URL(location, currentUrl);
}

/**
 * Whether the follow-up request should drop the body and switch to GET.
 *
 * @param {number} status
 * @param {string} method
 */
export function switchesToGet(status, method) {
  const upper = method.toUpperCase();
  return status === 303 || ((status === 301 || status === 302) && upper === 'POST');
}

/**
 * Build the next request for a redirect response.
 *
 * @param {{ url: string, method: string, body?: string | null, headers: Record<string, string> }} current
 * @param {number} status
 * @param {string} location
 * @param {object} [options]
 * @param {boolean} [options.allowInsecureRedirects]
 * @returns {{ url: string, method: string, body: string | null, headers: Record<string, string>, redirectedFrom: string }}
 */
export function followRedirect(current, status, location, options = {}) {
  const { allowInsecureRedirects = false } = options;
  const from = new URL(current.url);
  const to = resolveLocation(location, from);

  if (from.protocol === 'https:' && to.protocol === 'http:' && !allowInsecureRedirects) {
    throw new InsecureRedirectError(from.toString(), to.toString());
  }

  const dropBody = switchesToGet(status, current.method);
  return {
    url: to.toString(),
    method: dropBody ? 'GET' : current.method,
    body: dropBody ? null : (current.body ?? null),
    headers: { ...current.headers },
    redirectedFrom: from.toString(),
  };
}
