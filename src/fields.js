'use strict';

const { RuleError } = require('./errors');

const COMMON_FIELDS = [
  'event.action',
  'event.id',
  'event.time',
  'severity',
  'message',
  'source.ip',
  'source.port',
  'source.host',
  'source.geo.country',
  'actor.user',
  'actor.role',
];

const SCHEMAS = {
  default: {
    fields: [...COMMON_FIELDS],
  },
  acme: {
    fields: [
      ...COMMON_FIELDS,
      'cloud.region',
      'cloud.account',
    ],
  },
  globex: {
    fields: [
      ...COMMON_FIELDS,
      'network.srcip',
      'network.dstip',
      'network.protocol',
    ],
  },
};

function buildSchema(tenantId, definition) {
  const fields = new Set(definition.fields);
  const roots = new Set();
  for (const path of fields) {
    roots.add(path.split('.')[0]);
  }
  return Object.freeze({
    tenantId,
    fields,
    roots,
  });
}

const BUILT = new Map();
for (const [tenantId, definition] of Object.entries(SCHEMAS)) {
  BUILT.set(tenantId, buildSchema(tenantId, definition));
}

function getFieldSchema(tenantId) {
  const schema = BUILT.get(tenantId);
  if (!schema) {
    throw new RuleError('E_UNKNOWN_TENANT', `Unknown tenant ${JSON.stringify(tenantId)}`);
  }
  return schema;
}

/**
 * Resolve a field path against an event document. Returns an object with
 * `found` and `value` so callers can distinguish a missing field from a field
 * whose value happens to be undefined.
 */
function readField(event, segments, schema) {
  const root = segments[0];

  if (!schema.roots.has(root)) {
    return { found: false, value: undefined };
  }

  let current = event[root];
  for (let i = 1; i < segments.length; i += 1) {
    if (current === null || current === undefined) {
      return { found: false, value: undefined };
    }
    current = current[segments[i]];
  }

  if (current === undefined) {
    return { found: false, value: undefined };
  }
  return { found: true, value: current };
}

module.exports = {
  getFieldSchema,
  readField,
};
