'use strict';

const { GROUP_ATTRIBUTES, GROUP_SCHEMA } = require('../scim/schema');
const { matches } = require('./query');

function clone(value) {
  return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
}

class GroupRepo {
  constructor() {
    this.rows = new Map();
    this.sequence = 0;
  }

  resourceType() {
    return 'Group';
  }

  schema() {
    return GROUP_SCHEMA;
  }

  attributes() {
    return GROUP_ATTRIBUTES;
  }

  nextId(tenant) {
    this.sequence += 1;
    return `${tenant}-g${String(this.sequence).padStart(4, '0')}`;
  }

  list(tenant) {
    return Array.from(this.rows.values()).filter((row) => row.tenant === tenant);
  }

  search(condition) {
    return Array.from(this.rows.values())
      .map((row) => ({ ...row, ...row.attributes }))
      .filter((row) => matches(row, condition));
  }

  find(tenant, id) {
    const row = this.rows.get(id);
    if (!row || row.tenant !== tenant) return null;
    return row;
  }

  create(tenant, attributes) {
    const id = this.nextId(tenant);
    const now = new Date().toISOString();
    const row = {
      id,
      tenant,
      version: 1,
      created: now,
      lastModified: now,
      attributes: clone(attributes),
    };
    this.rows.set(id, row);
    return row;
  }

  save(row) {
    row.lastModified = new Date().toISOString();
    row.version += 1;
    this.rows.set(row.id, row);
    return row;
  }

  delete(tenant, id) {
    const row = this.find(tenant, id);
    if (!row) return false;
    this.rows.delete(id);
    return true;
  }
}

module.exports = { GroupRepo };
