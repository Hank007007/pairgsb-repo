export class QueryError extends Error {
  constructor(message, options = {}) {
    super(message, options.cause ? { cause: options.cause } : undefined);
    this.name = 'QueryError';
    this.code = options.code ?? 'invalid_query';
    this.field = options.field;
    this.status = options.status ?? 400;
  }
}

export function isQueryError(error) {
  return error instanceof QueryError;
}
