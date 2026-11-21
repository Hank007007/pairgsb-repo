'use strict';

const { USER_ATTRIBUTES, USER_SCHEMA } = require('../scim/schema');
const { matches } = require('./query');

function clone(value) {
  return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
}

class UserRepo {
  constructor() {
    this.rows = new Map();
    this.sequence = 0;
  }

  resourceType() {
    return 'User';
  }

  schema() {
    return USER_SCHEMA;
  }

  attributes() {
    return USER_ATTRIBUTES;
  }

  nextId(tenant) {
    this.sequence += 1;
    return `${tenant}-u${String(this.sequence).padStart(4, '0')}`;
  }

  list(tenant) {
    return Array.from(this.rows.values()).filter((row) => row.tenant === tenant);
  }

  search(condition) {
    return Array.from(this.rows.values())
      .map((row) => ({ ...row.attributes, ...row }))
      .filter((row) => matches(row, condition));
  }

  find(tenant, id) {
    const row = this.rows.get(id);
    if (!row || row.tenant !== tenant) return null;
    return row;
  }

  findByUserName(tenant, userName) {
    const wanted = String(userName).toLowerCase();
    return this.list(tenant).find(
      (row) => String(row.attributes.userName || '').toLowerCase() === wanted
    ) || null;
  }

  findByExternalId(tenant, externalId) {
    return this.list(tenant).find(
      (row) => row.attributes.externalId === externalId
    ) || null;
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

module.exports = { UserRepo };
