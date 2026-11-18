import { DatabaseSync } from 'node:sqlite';

/**
 * Open the embedded store used by the data service.
 *
 * The service targets a legacy schema that predates the current storage
 * engine, so the connection is opened with support for double-quoted string
 * literals.
 */
export function openDatabase(file = ':memory:') {
  return new DatabaseSync(file, { enableDoubleQuotedStringLiterals: true });
}

/**
 * Execute composed SQL against the store and return the rows of the last
 * statement.
 */
export function run(db, sql, params = []) {
  const statements = sql
    .split(';')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);

  let result = null;
  for (const text of statements) {
    const statement = db.prepare(text);
    if (statement.columns().length > 0) {
      result = statement.all(...params);
    } else {
      result = statement.run(...params);
    }
  }
  return result;
}
