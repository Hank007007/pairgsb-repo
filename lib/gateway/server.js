/**
 * Gateway request handling.
 *
 * `handleRequest` is the single entry point used by the daemon (and by tests):
 * it admits a body, resolves the schema for the request, hands the work to
 * `verifyRequest`, and dispatches the decoded message to the router.
 */
import { ProtolensError, CODES } from '../errors.js';
import { resolveType } from '../schema/resolve.js';
import { verifyRequest } from './verify.js';

/** Largest body the gateway will process. */
export const MAX_BODY_BYTES = 4 * 1024 * 1024;

const DEFAULT_LOGGER = {
  info() {},
  error(message, fields) {
    console.error(message, fields ?? '');
  },
};

function bodyOf(request) {
  const body = request?.body;
  if (Buffer.isBuffer(body)) return body;
  if (body instanceof Uint8Array) return Buffer.from(body);
  if (typeof body === 'string') return Buffer.from(body);
  return Buffer.alloc(0);
}

function headerValue(headers, name) {
  if (!headers) return undefined;
  const direct = headers[name];
  if (direct !== undefined) return direct;
  const lower = name.toLowerCase();
  for (const key of Object.keys(headers)) {
    if (key.toLowerCase() === lower) return headers[key];
  }
  return undefined;
}

/**
 * Handle one gateway request.
 *
 * @param {object} request `{ body, type, version?, signature, headers? }`
 * @param {object} ctx `{ registry, secret, router?, logger? }`
 * @returns {{ status: number, body?: object, error?: object }}
 */
export function handleRequest(request, ctx) {
  const logger = ctx.logger ?? DEFAULT_LOGGER;
  const body = bodyOf(request);
  const headers = request.headers ?? {};
  const declaredLength = Number(headerValue(headers, 'content-length'));

  if (declaredLength > MAX_BODY_BYTES) {
    return {
      status: 413,
      error: {
        code: CODES.BODY_TOO_LARGE,
        message: `request body exceeds ${MAX_BODY_BYTES} bytes`,
      },
    };
  }

  let version;
  try {
    const schema = resolveType(ctx.registry, request.type, request.version);
    version = schema.version;

    const requestContext = {
      ...ctx,
      resolve() {
        return resolveType(ctx.registry, request.type, version);
      },
    };

    const { message } = verifyRequest(request, requestContext);

    const routed =
      typeof ctx.router === 'function'
        ? ctx.router(message, { type: request.type, version, headers })
        : undefined;

    return { status: 200, body: { type: request.type, version, message, routed } };
  } catch (err) {
    logger.error('request failed', {
      type: request.type,
      version,
      code: err && err.code,
      body: body.subarray(0, 64).toString('hex'),
    });

    if (err instanceof ProtolensError) {
      return {
        status: 500,
        error: { code: err.code, message: err.message },
      };
    }
    throw err;
  }
}
