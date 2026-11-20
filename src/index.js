/**
 * credmux-relay — outbound credential-injection middleware.
 *
 * Business code calls `relay.request(url, opts)`. The relay picks a credential
 * for the target host from the active secrets bundle, injects it, follows
 * redirects, retries transient failures, and writes a redacted audit record for
 * every attempt.
 *
 * Behavior is specified in `docs/credential-routing.md`.
 */

import { injectIntoRequest } from './inject.js';
import { followRedirect, isRedirectStatus } from './redirect.js';
import { createRetryQueue, isRetryableStatus } from './retry.js';
import { createAuditLog, redactHeaders } from './audit.js';
import { nodeTransport } from './transport.js';
import { createBundleStore, compileBundle } from './bundle.js';
import {
  AbortedError,
  OptionsError,
  TooManyRedirectsError,
  TransportError,
} from './errors.js';

const EMPTY_BUNDLE = compileBundle({ secrets: [] }, 0, '<empty>');

/**
 * @typedef {object} RelayOptions
 * @property {object} [bundle]
 * @property {(req: object) => Promise<object>} [transport]
 * @property {object} [audit]
 * @property {number} [maxAttempts]
 * @property {number} [baseDelayMs]
 * @property {number} [maxDelayMs]
 * @property {number} [maxRedirects]
 * @property {boolean} [allowInsecureRedirects]
 * @property {() => number} [random]
 */

/**
 * @param {RelayOptions} [options]
 */
export function createRelay(options = {}) {
  const {
    bundle = EMPTY_BUNDLE,
    transport = nodeTransport,
    audit = createAuditLog(),
    maxAttempts = 3,
    baseDelayMs = 100,
    maxDelayMs = 5000,
    maxRedirects = 5,
    allowInsecureRedirects = false,
    random = Math.random,
  } = options;

  const store = createBundleStore(bundle);
  const retryQueue = createRetryQueue({ baseDelayMs, maxDelayMs, random });
  const counters = { requests: 0, attempts: 0, retries: 0, redirects: 0 };

  /**
   * Send one attempt. `headers` are the exact headers that go on the wire.
   */
  async function send(state, headers, entry, attempt) {
    counters.attempts += 1;
    state.stats.attempts += 1;
    const response = await transport({
      url: state.url,
      method: state.method,
      headers,
      body: state.body,
      timeoutMs: state.timeoutMs,
      signal: state.signal,
    });
    audit.write({
      type: 'attempt',
      method: state.method,
      url: state.url,
      id: entry?.id ?? null,
      attempt,
      status: response.status,
      headers,
    });
    return response;
  }

  /**
   * Retry entry point: wait out the backoff, then send the queued request and
   * keep going while the outcome is retryable.
   */
  function scheduleRetry(state, attempt, headers, entry) {
    counters.retries += 1;
    return new Promise((resolve, reject) => {
      retryQueue.scheduleRetry({
        url: state.url,
        method: state.method,
        body: state.body,
        headers,
        entry,
        attempt: attempt + 1,
        dispatch: async (request) => {
          let response;
          try {
            response = await send(state, request.headers, request.entry, request.attempt);
          } catch (error) {
            if (error instanceof AbortedError) {
              audit.write({ type: 'aborted', method: state.method, url: state.url, attempt: request.attempt });
              reject(error);
              return;
            }
            const wrapped = error instanceof TransportError ? error : new TransportError(error.message, error);
            if (request.attempt < maxAttempts) {
              resolve(scheduleRetry(state, request.attempt, request.headers, request.entry));
            } else {
              audit.write({ type: 'transport-failed', method: state.method, url: state.url, attempt: request.attempt });
              reject(wrapped);
            }
            return;
          }

          if (isRetryableStatus(response.status) && request.attempt < maxAttempts) {
            resolve(scheduleRetry(state, request.attempt, request.headers, request.entry));
            return;
          }
          resolve({ response, headers: request.headers, entry: request.entry });
        },
        onError: reject,
        onCancel: () => reject(new AbortedError('retry cancelled')),
      });
    });
  }

  /**
   * Run one hop to completion, including retries. Returns the final response
   * for that hop together with the headers that produced it.
   */
  async function runHop(state) {
    const attempt = 1;
    const injected = injectIntoRequest(store.current, state.url, state.callerHeaders);

    let response;
    try {
      response = await send(state, injected.headers, injected.entry, attempt);
    } catch (error) {
      if (error instanceof AbortedError) {
        audit.write({ type: 'aborted', method: state.method, url: state.url, attempt });
        throw error;
      }
      const wrapped = error instanceof TransportError ? error : new TransportError(error.message, error);
      if (attempt >= maxAttempts) {
        audit.write({ type: 'transport-failed', method: state.method, url: state.url, attempt });
        throw wrapped;
      }
      return scheduleRetry(state, attempt, injected.headers, injected.entry);
    }

    if (!isRetryableStatus(response.status) || attempt >= maxAttempts) {
      return { response, headers: injected.headers, entry: injected.entry };
    }

    return scheduleRetry(state, attempt, injected.headers, injected.entry);
  }

  /**
   * Follow the redirect chain until a non-redirect response arrives.
   */
  async function executeHop(initial) {
    let current = initial;
    let redirects = 0;
    const seen = new Set([current.url]);

    for (;;) {
      const result = await runHop(current);

      if (!isRedirectStatus(result.response.status)) {
        return { ...result, redirects, url: current.url, method: current.method };
      }

      const location = result.response.headers?.location ?? result.response.headers?.Location;
      if (!location) {
        return { ...result, redirects, url: current.url, method: current.method };
      }

      if (redirects >= maxRedirects) {
        audit.write({ type: 'redirect-limit', method: current.method, url: current.url, attempt: 1 });
        throw new TooManyRedirectsError(current.url, maxRedirects);
      }

      const next = followRedirect(
        { url: current.url, method: current.method, body: current.body, headers: result.headers },
        result.response.status,
        location,
        { allowInsecureRedirects },
      );

      if (seen.has(next.url)) {
        audit.write({ type: 'redirect-loop', method: current.method, url: next.url });
        throw new TooManyRedirectsError(next.url, maxRedirects);
      }
      seen.add(next.url);
      redirects += 1;
      counters.redirects += 1;

      audit.write({
        type: 'redirect',
        method: next.method,
        url: next.url,
        redirectFrom: current.url,
        status: result.response.status,
      });

      current = {
        ...current,
        url: next.url,
        method: next.method,
        body: next.body,
        callerHeaders: next.headers,
      };
    }
  }

  /**
   * Public request API.
   *
   * @param {string} url
   * @param {{ method?: string, headers?: Record<string, string>, body?: string | null, timeoutMs?: number, signal?: AbortSignal, maxRedirects?: number }} [opts]
   */
  async function request(url, opts = {}) {
    if (typeof url !== 'string' || url.length === 0) {
      throw new OptionsError('request url must be a non-empty string');
    }
    if (opts.signal?.aborted) {
      throw new AbortedError();
    }

    const state = {
      url: new URL(url).toString(),
      method: (opts.method ?? 'GET').toUpperCase(),
      body: opts.body ?? null,
      callerHeaders: { ...(opts.headers ?? {}) },
      timeoutMs: opts.timeoutMs,
      signal: opts.signal,
      stats: { attempts: 0 },
    };
    counters.requests += 1;

    const result = await executeHop(state);

    return {
      status: result.response.status,
      headers: result.response.headers,
      body: result.response.body,
      url: result.url,
      attempts: state.stats.attempts,
      redirects: result.redirects,
      redactedHeaders: redactHeaders(result.headers),
    };
  }

  return {
    request,
    store,
    audit,
    diagnostics() {
      return {
        revision: store.current.revision,
        entries: store.current.entries.length,
        queuedRetries: retryQueue.size(),
        ...counters,
      };
    },
    /** @param {object} next */
    setBundle(next) {
      store.replace(next);
    },
    close() {
      retryQueue.close();
    },
  };
}

export { compileBundle } from './bundle.js';
export { createAuditLog } from './audit.js';
