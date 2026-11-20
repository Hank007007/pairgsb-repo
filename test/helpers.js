/**
 * Shared test helpers: a scriptable in-memory transport and small builders.
 */

import { compileBundle } from '../src/bundle.js';

let revisionCounter = 0;

/**
 * Build a compiled bundle from shorthand entries.
 *
 * @param {Array<object>} secrets
 */
export function makeBundle(secrets) {
  revisionCounter += 1;
  return compileBundle({ version: 1, secrets }, revisionCounter, '<test>');
}

/**
 * A transport that replays a queue of responses (or calls a handler) and keeps
 * a transcript of every outbound request.
 *
 * @param {Array<object | ((request: object) => object | Promise<object>)> | ((request: object) => object | Promise<object>)} plan
 */
export function createFakeTransport(plan) {
  const script = Array.isArray(plan) ? plan.slice() : [plan];
  const sent = [];

  async function transport(request) {
    sent.push({
      url: request.url,
      method: request.method,
      headers: { ...request.headers },
      body: request.body ?? null,
    });

    const next = script.length > 1 ? script.shift() : script[0];
    if (next === undefined) {
      throw new Error('fake transport ran out of responses');
    }
    const response = typeof next === 'function' ? await next(request) : next;
    return {
      status: 200,
      headers: {},
      body: '',
      url: request.url,
      ...response,
    };
  }

  return {
    transport,
    sent,
    get callCount() {
      return sent.length;
    },
    last() {
      return sent[sent.length - 1];
    },
  };
}

/**
 * A response object shaped like the transport contract.
 */
export function reply(status, { headers = {}, body = '' } = {}) {
  return { status, headers, body };
}
