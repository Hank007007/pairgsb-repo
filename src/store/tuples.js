/**
 * Tuple storage.
 *
 * The store owns the mutation clock and the tuple rows. Rows are kept with
 * the version that created them and the version that deleted them so that a
 * read pinned to an older snapshot can ignore later mutations.
 */

import { AuthError, ErrorCodes } from '../errors.js';
import { parseTuple, formatObjectRef, formatTupleParts } from '../tuple.js';
import { bump, currentVersion } from './consistency.js';

export function createTupleStore() {
  return {
    clock: new Map(),
    issued: new Map(),
    rows: new Map(),
  };
}

function tupleKey(tenant, tuple) {
  return `${tenant}\u0000${formatTupleParts(tuple.object, tuple.relation, tuple.subject)}`;
}

/**
 * Insert a tuple and return the zookie of the mutation.
 *
 * @param {ReturnType<typeof createTupleStore>} store
 * @param {string} tenant
 * @param {string|{object: string, relation: string, subject: string}} input
 * @returns {{tuple: string, zookie: string}}
 */
export function write(store, tenant, input) {
  const tuple = parseTuple(input, ErrorCodes.INVALID_TUPLE);

  const zookie = bump(store, tenant);
  const version = currentVersion(store, tenant);

  const key = tupleKey(tenant, tuple);
  const history = store.rows.get(key) ?? [];
  history.push({
    object: tuple.object,
    relation: tuple.relation,
    subject: tuple.subject,
    createdVersion: version,
    deletedVersion: null,
  });
  store.rows.set(key, history);

  return { tuple: tuple.text, zookie };
}

/**
 * Delete a tuple and return the zookie of the deletion.
 *
 * @param {ReturnType<typeof createTupleStore>} store
 * @param {string} tenant
 * @param {string|{object: string, relation: string, subject: string}} input
 * @returns {{tuple: string, zookie: string, deleted: boolean}}
 */
export function deleteTuple(store, tenant, input) {
  const tuple = parseTuple(input, ErrorCodes.INVALID_TUPLE);
  const key = tupleKey(tenant, tuple);
  const history = store.rows.get(key) ?? [];

  const zookie = bump(store, tenant);
  const version = currentVersion(store, tenant);

  let deleted = false;
  for (let index = history.length - 1; index >= 0; index -= 1) {
    const row = history[index];
    if (row.deletedVersion === null && row.createdVersion <= version) {
      row.deletedVersion = version;
      deleted = true;
      break;
    }
  }

  return { tuple: tuple.text, zookie, deleted };
}

export { deleteTuple as delete };

/**
 * Read the subjects stored for `object#relation` at a snapshot.
 *
 * @param {ReturnType<typeof createTupleStore>} store
 * @param {string} tenant
 * @param {{type: string, id: string}} object
 * @param {string} relation
 * @param {{asOf?: number}} [options]
 * @returns {Array<object>} subject references
 */
export function read(store, tenant, object, relation, options = {}) {
  const asOf = options.asOf ?? currentVersion(store, tenant);
  const prefix = `${tenant}\u0000${formatObjectRef(object)}#${relation}@`;
  const subjects = [];

  for (const [key, history] of store.rows) {
    if (!key.startsWith(prefix)) continue;
    for (const row of history) {
      if (row.createdVersion > asOf) continue;
      if (row.deletedVersion !== null && row.deletedVersion <= asOf) continue;
      subjects.push(row.subject);
    }
  }

  return subjects;
}

/**
 * Parse a request's contextual tuple list.
 *
 * @param {Array<string|object>|undefined} values
 * @returns {Array<{object: object, relation: string, subject: object, text: string}>}
 */
export function parseContextualTuples(values) {
  if (values === undefined || values === null) return [];
  if (!Array.isArray(values)) {
    throw new AuthError(ErrorCodes.INVALID_TUPLE, 'contextualTuples must be an array');
  }
  return values.map((value) => parseTuple(value, ErrorCodes.INVALID_TUPLE));
}
