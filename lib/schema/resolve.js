/**
 * Schema resolution.
 *
 * Turns a registry entry into the immutable object the wire layer consumes: a
 * field list, a field-number lookup and a way to reach nested message types.
 * Resolved schemas are cached per registry.
 */
import { ProtolensError, CODES } from '../errors.js';

/** @type {WeakMap<object, Map<string, object>>} */
const resolvedCaches = new WeakMap();

/** @type {WeakMap<object, Map<string, Map<number, object>>>} */
const fieldCaches = new WeakMap();

function resolvedCacheFor(registry) {
  let cache = resolvedCaches.get(registry);
  if (cache === undefined) {
    cache = new Map();
    resolvedCaches.set(registry, cache);
  }
  return cache;
}

function fieldCacheFor(registry) {
  let cache = fieldCaches.get(registry);
  if (cache === undefined) {
    cache = new Map();
    fieldCaches.set(registry, cache);
  }
  return cache;
}

/**
 * Build the composite cache key for a resolved schema.
 *
 * @param {string} type message type name
 * @param {number} version schema version
 * @returns {string}
 */
export function cacheKey(type, version) {
  return `${type}#${version}`;
}

/**
 * Resolve a message type to the schema object used by the wire layer.
 *
 * @param {object} registry a `SchemaRegistry`
 * @param {string} typeName message type name
 * @param {number} [version] version to resolve; defaults to the current one
 * @returns {object} resolved schema
 */
export function resolveType(registry, typeName, version) {
  const cache = resolvedCacheFor(registry);
  const key = typeName;

  const cached = cache.get(key);
  if (cached !== undefined) {
    return cached;
  }

  const wanted = version ?? registry.currentVersion(typeName);
  const definition = registry.get(typeName, wanted);

  /** @type {any} */
  const schema = {
    type: definition.name,
    version: definition.version,
    registry,
    fields: definition.fields,
    getType(name) {
      return resolveType(registry, name);
    },
    fieldByNumber(number) {
      return fieldByNumber(schema, number);
    },
  };

  cache.set(key, schema);
  return schema;
}

/**
 * Look up a field by its wire field number within a resolved schema.
 *
 * @param {object} schema resolved schema
 * @param {number} fieldNumber
 * @returns {object|undefined}
 */
export function fieldByNumber(schema, fieldNumber) {
  const registry = schema.registry;
  const cache = fieldCacheFor(registry);
  const key = schema.type;

  let table = cache.get(key);
  if (table === undefined) {
    table = new Map();
    for (const field of schema.fields) {
      table.set(field.number, field);
    }
    cache.set(key, table);
  }
  return table.get(fieldNumber);
}

/**
 * Drop every cached resolution for a registry. Intended for tests and for
 * operators who need to reclaim memory.
 *
 * @param {object} registry
 */
export function clearResolveCache(registry) {
  resolvedCaches.delete(registry);
  fieldCaches.delete(registry);
}
