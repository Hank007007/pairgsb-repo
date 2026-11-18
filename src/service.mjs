import { buildQuery } from './builder.mjs';
import { run } from './pool.mjs';
import { QueryError, isQueryError } from './errors.mjs';

function isPlainObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function errorResponse(error) {
  const body = {
    error: {
      code: error.code,
      message: error.message,
    },
  };
  if (error.field !== undefined) {
    body.error.field = error.field;
  }
  return { status: error.status ?? 400, body };
}

/**
 * Answer one query request.
 *
 * Returns `{ status, body }`: `200` with `{ rows }` when the statement ran,
 * otherwise an error envelope as described in `docs/query-rules.md` §7.
 */
export function handleQuery(db, request) {
  try {
    if (!isPlainObject(request)) {
      throw new QueryError('request body must be a JSON object', {
        code: 'invalid_query',
      });
    }
    const { sql, params } = buildQuery(request);
    const result = run(db, sql, params);
    const rows = Array.isArray(result) ? result.map((row) => ({ ...row })) : [];
    return { status: 200, body: { rows } };
  } catch (error) {
    if (isQueryError(error)) {
      return errorResponse(error);
    }
    return {
      status: 500,
      body: {
        error: {
          code: 'internal_error',
          message: 'query failed',
        },
      },
    };
  }
}
