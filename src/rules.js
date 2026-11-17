'use strict';

const fs = require('node:fs');
const path = require('node:path');

const DEFAULT_ROUTES_FILE = path.join(__dirname, '..', 'config', 'routes.json');

function escapeRegExp(value) {
  return value.replace(/[.+?^${}()|[\]\\]/g, '\\$&');
}

function compilePattern(pattern) {
  if (typeof pattern !== 'string' || pattern.length === 0 || pattern[0] !== '/') {
    throw new TypeError('route pattern must be an absolute path');
  }
  const source = '^' + escapeRegExp(pattern).replace(/\*/g, '.*') + '$';
  return new RegExp(source);
}

function normalizeRule(entry, index) {
  if (!entry || typeof entry !== 'object') {
    throw new TypeError(`route rule at index ${index} must be an object`);
  }
  if (typeof entry.id !== 'string' || entry.id.length === 0) {
    throw new TypeError(`route rule at index ${index} is missing an id`);
  }
  const methods = Array.isArray(entry.methods)
    ? entry.methods.map((method) => String(method).toUpperCase())
    : [];
  const scopes = Array.isArray(entry.scopes) ? entry.scopes.map(String) : [];
  const tenants = Array.isArray(entry.tenants) ? entry.tenants.map(String) : null;

  return Object.freeze({
    id: entry.id,
    pattern: entry.pattern,
    methods: Object.freeze(methods),
    scopes: Object.freeze(scopes),
    tenants: tenants === null ? null : Object.freeze(tenants),
    matcher: compilePattern(entry.pattern),
  });
}

function parseRoutes(document) {
  if (!document || typeof document !== 'object' || !Array.isArray(document.rules)) {
    throw new TypeError('route table must contain a "rules" array');
  }
  return Object.freeze(document.rules.map(normalizeRule));
}

function loadRules(filePath = DEFAULT_ROUTES_FILE) {
  const document = JSON.parse(fs.readFileSync(filePath, 'utf8'));
  return parseRoutes(document);
}

function ruleAppliesToTenant(rule, tenant) {
  if (rule.tenants === null) {
    return true;
  }
  return rule.tenants.includes(tenant);
}

function selectRule(rules, session, method, pathname) {
  const tenant = session && typeof session.tenant === 'string' ? session.tenant : '';
  for (const rule of rules) {
    if (!ruleAppliesToTenant(rule, tenant)) {
      continue;
    }
    if (!rule.methods.includes(method)) {
      continue;
    }
    if (!rule.matcher.test(pathname)) {
      continue;
    }
    return rule;
  }
  return null;
}

function missingScopes(rule, session) {
  const granted = new Set(Array.isArray(session && session.scopes) ? session.scopes : []);
  return rule.scopes.filter((scope) => !granted.has(scope));
}

module.exports = {
  DEFAULT_ROUTES_FILE,
  compilePattern,
  parseRoutes,
  loadRules,
  selectRule,
  missingScopes,
};
