import { FIELD_MAP, TABLE_NAME } from './fields.mjs';
import { QueryError } from './errors.mjs';
import { applyPage } from './plan.mjs';

const COMPARISON_OPERATORS = Object.freeze({
  eq: '=',
  ne: '<>',
  lt: '<',
  lte: '<=',
  gt: '>',
  gte: '>=',
});

const SELECT_COLUMNS = Object.values(FIELD_MAP)
  .map((field) => field.column)
  .join(', ');

/**
 * Render a field name for use in SQL.
 *
 * The field map gives the physical column for an API field. Names that are
 * not in the map are passed through unchanged so callers can reach columns
 * that are not exposed to the API yet.
 */
export function quoteIdent(name) {
  const field = FIELD_MAP[name];
  const column = field === undefined ? name : field.column;
  if (column.includes(' ')) {
    return `"${column}"`;
  }
  return column;
}

/**
 * Build a contains-match predicate for a text field.
 *
 * `value` is the raw search text; the wildcard pattern is assembled here.
 */
export function buildLike(field, value) {
  const pattern = '%' + String(value) + '%';
  return `${quoteIdent(field)} LIKE ${JSON.stringify(pattern)}`;
}

/**
 * Build a WHERE clause from a filter expression.
 *
 * Bound values are appended to `params` in the order their placeholders
 * appear in the returned fragment.
 */
export function buildWhere(filter, params = []) {
  if (filter === undefined || filter === null) {
    return '';
  }
  if (typeof filter !== 'object' || Array.isArray(filter)) {
    throw new QueryError('filter must be an object', { code: 'invalid_query' });
  }
  if (filter.and !== undefined) {
    if (!Array.isArray(filter.and)) {
      throw new QueryError('filter.and must be an array', { code: 'invalid_query' });
    }
    const parts = filter.and.map((item) => buildWhere(item, params));
    return `(${parts.join(' AND ')})`;
  }
  if (filter.or !== undefined) {
    if (!Array.isArray(filter.or)) {
      throw new QueryError('filter.or must be an array', { code: 'invalid_query' });
    }
    const parts = filter.or.map((item) => buildWhere(item, params));
    return `(${parts.join(' OR ')})`;
  }

  const { field, op } = filter;
  const value = filter.value;
  if (typeof field !== 'string' || field.length === 0) {
    throw new QueryError('filter.field must be a non-empty string', {
      code: 'invalid_query',
    });
  }
  if (typeof op !== 'string' || op.length === 0) {
    throw new QueryError('filter.op must be a non-empty string', {
      code: 'invalid_query',
      field,
    });
  }

  const column = quoteIdent(field);

  if (op === 'is_null') {
    return `${column} IS NULL`;
  }
  if (op === 'not_null') {
    return `${column} IS NOT NULL`;
  }
  if (op === 'like') {
    return buildLike(field, value);
  }
  if (op === 'in') {
    if (!Array.isArray(value) || value.length === 0) {
      throw new QueryError(`operator "in" on field "${field}" needs a non-empty array`, {
        code: 'invalid_value',
        field,
      });
    }
    const items = value.map((item) => JSON.stringify(item));
    return `${column} IN (${items.join(', ')})`;
  }

  const comparison = COMPARISON_OPERATORS[op];
  if (comparison === undefined) {
    throw new QueryError(`unknown operator "${op}" for field "${field}"`, {
      code: 'unknown_operator',
      field,
    });
  }

  if (value === null) {
    params.push(null);
    return `${column} ${op === 'eq' ? 'IS' : 'IS NOT'} ?`;
  }

  return `${column} ${comparison} ${JSON.stringify(value)}`;
}

/**
 * Build an ORDER BY clause from one sort key or a list of them.
 */
export function buildOrderBy(sort) {
  if (sort === undefined || sort === null) {
    return '';
  }
  const keys = Array.isArray(sort) ? sort : [sort];
  if (keys.length === 0) {
    return '';
  }
  const clauses = keys.map((key) => {
    if (key === null || typeof key !== 'object' || typeof key.field !== 'string') {
      throw new QueryError('sort key must be an object with a field', {
        code: 'invalid_sort',
      });
    }
    const field = key.field;
    const direction = key.direction === undefined ? 'asc' : String(key.direction).toLowerCase();
    if (direction !== 'asc' && direction !== 'desc') {
      throw new QueryError(`sort direction must be "asc" or "desc", got "${key.direction}"`, {
        code: 'invalid_sort',
        field,
      });
    }
    return `${quoteIdent(field)} ${direction.toUpperCase()}`;
  });
  return `ORDER BY ${clauses.join(', ')}`;
}

/**
 * Compose one SELECT statement from a request body.
 */
export function buildQuery(request = {}) {
  const params = [];
  const filter = request === null ? undefined : request.filter;
  const sort = request === null ? undefined : request.sort;
  const limit = request === null ? undefined : request.limit;
  const offset = request === null ? undefined : request.offset;

  const where = buildWhere(filter, params);
  const orderBy = buildOrderBy(sort);

  let sql = `SELECT ${SELECT_COLUMNS} FROM ${TABLE_NAME}`;
  if (where.length > 0) {
    sql += ` WHERE ${where}`;
  }
  if (orderBy.length > 0) {
    sql += ` ${orderBy}`;
  }
  sql = applyPage(sql, { limit, offset });

  return { sql, params };
}
