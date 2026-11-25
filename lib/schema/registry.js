/**
 * Schema registry.
 *
 * Holds the published versions of every message type and supports hot reload:
 * a fresh snapshot of the schema set is swapped in while traffic is flowing.
 *
 * A type definition is `{ name, version, fields }`. Fields are normalised on
 * the way in so that every definition the registry hands out has the same
 * shape regardless of how the snapshot was written.
 */
import { ProtolensError, CODES } from '../errors.js';

/**
 * @typedef {object} FieldDef
 * @property {number} number
 * @property {string} name
 * @property {string} type
 * @property {boolean} repeated
 * @property {boolean} packed
 * @property {string} [messageType]
 */

function normalizeField(field) {
  return {
    number: field.number,
    name: field.name,
    type: field.type,
    repeated: Boolean(field.repeated),
    packed: Boolean(field.packed),
    messageType: field.messageType,
  };
}

function normalizeFields(fields) {
  return (fields ?? []).map(normalizeField);
}

function sameField(a, b) {
  return (
    a.number === b.number &&
    a.name === b.name &&
    a.type === b.type &&
    a.repeated === b.repeated &&
    a.packed === b.packed &&
    a.messageType === b.messageType
  );
}

function sameFields(a, b) {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (!sameField(a[i], b[i])) return false;
  }
  return true;
}

export class SchemaRegistry {
  /**
   * @param {{ types?: Record<string, { fields: FieldDef[] }> }} [snapshot]
   */
  constructor(snapshot = { types: {} }) {
    /** @type {Map<string, { current: number, versions: Map<number, object> }>} */
    this._types = new Map();
    this._install(snapshot);
  }

  _install(snapshot) {
    for (const [name, definition] of Object.entries(snapshot.types ?? {})) {
      const fields = normalizeFields(definition.fields);
      this._types.set(name, {
        current: 1,
        versions: new Map([[1, { name, version: 1, fields }]]),
      });
    }
  }

  /**
   * Version currently published for `name`.
   *
   * @param {string} name
   * @returns {number}
   */
  currentVersion(name) {
    const entry = this._types.get(name);
    if (!entry) {
      throw new ProtolensError(CODES.SCHEMA_NOT_FOUND, `unknown message type: ${name}`, {
        type: name,
      });
    }
    return entry.current;
  }

  /**
   * Look up a published type definition.
   *
   * @param {string} name
   * @param {number} [version] defaults to the current version
   * @returns {{ name: string, version: number, fields: FieldDef[] }}
   */
  get(name, version) {
    const entry = this._types.get(name);
    if (!entry) {
      throw new ProtolensError(CODES.SCHEMA_NOT_FOUND, `unknown message type: ${name}`, {
        type: name,
      });
    }
    const wanted = version ?? entry.current;
    const definition = entry.versions.get(wanted);
    if (!definition) {
      throw new ProtolensError(
        CODES.SCHEMA_NOT_FOUND,
        `message type ${name} has no version ${wanted}`,
        { type: name, version: wanted },
      );
    }
    return definition;
  }

  /**
   * Whether `name` is known to the registry.
   *
   * @param {string} name
   * @returns {boolean}
   */
  has(name) {
    return this._types.has(name);
  }

  /**
   * Publish a new snapshot.
   *
   * Types whose definition is unchanged keep their version; changed types move
   * to the next version number. The registry's current pointer is swapped once
   * the whole snapshot has been processed.
   *
   * @param {{ types?: Record<string, { fields: FieldDef[] }> }} snapshot
   */
  reload(snapshot) {
    for (const [name, definition] of Object.entries(snapshot.types ?? {})) {
      const fields = normalizeFields(definition.fields);
      const entry = this._types.get(name);

      if (!entry) {
        this._types.set(name, {
          current: 1,
          versions: new Map([[1, { name, version: 1, fields }]]),
        });
        continue;
      }

      const active = entry.versions.get(entry.current);
      if (sameFields(active.fields, fields)) {
        continue;
      }

      const next = entry.current + 1;
      active.fields.splice(0, active.fields.length, ...fields);
      active.version = next;
      entry.versions.set(next, active);
      entry.current = next;
    }
  }

  /**
   * Names of every type the registry knows.
   *
   * @returns {string[]}
   */
  typeNames() {
    return [...this._types.keys()];
  }
}
