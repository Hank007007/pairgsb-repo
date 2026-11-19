'use strict';

const { normalizeHost } = require('./vault');

/**
 * Canonical form used for every comparison: lower case, no trailing dot, no
 * brackets around an IPv6 literal. Returns null when the value cannot be a host.
 */
function canonicalHost(value) {
  if (typeof value !== 'string') return null;
  let host = value.trim().toLowerCase();
  if (host.startsWith('[') && host.endsWith(']')) host = host.slice(1, -1);
  if (host.endsWith('.')) host = host.slice(0, -1);
  return host.length === 0 ? null : host;
}

/**
 * Maps a request host to the rule that governs it, or null when the host is
 * outside the configured scope.
 */
function selectCredential(rules, host) {
  const target = canonicalHost(host);
  if (target === null) return null;

  let selected = null;
  for (const rule of rules) {
    if (!target.endsWith(rule.host)) continue;
    if (selected === null || rule.host.length > selected.host.length) selected = rule;
  }
  return selected;
}

function parseNoProxy(env) {
  const raw = env.NO_PROXY !== undefined ? env.NO_PROXY : env.no_proxy;
  if (typeof raw !== 'string' || raw.trim().length === 0) return [];
  return raw
    .split(/[\s,]+/)
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

function matchesNoProxy(host, port, env) {
  const target = canonicalHost(host);
  if (target === null) return false;
  for (const entry of parseNoProxy(env)) {
    if (matchesNoProxyEntry(target, port, entry)) return true;
  }
  return false;
}

function matchesNoProxyEntry(target, port, entry) {
  let pattern = entry.trim().toLowerCase();
  if (pattern === '*') return true;

  let entryPort = null;
  const withPort = /^(.*):(\d+)$/.exec(pattern);
  if (withPort !== null && !withPort[1].includes(':') && !withPort[1].startsWith('[')) {
    pattern = withPort[1];
    entryPort = Number(withPort[2]);
  }
  if (entryPort !== null && entryPort !== port) return false;

  if (pattern.startsWith('*.')) pattern = pattern.slice(2);
  else if (pattern.startsWith('.')) pattern = pattern.slice(1);
  if (pattern.length === 0) return false;
  if (pattern.startsWith('[') && pattern.endsWith(']')) pattern = pattern.slice(1, -1);

  return target === pattern || target.endsWith(`.${pattern}`);
}

/**
 * Validates a host taken from configuration. Throws when it is not a plain
 * lower-case DNS name.
 */
function ruleHost(value) {
  return normalizeHost(value);
}

module.exports = {
  canonicalHost,
  selectCredential,
  matchesNoProxy,
  parseNoProxy,
  ruleHost,
};
