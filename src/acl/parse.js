import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { isValidFilter } from './match.js';

const ACTIONS = new Set(['publish', 'subscribe']);
const EFFECTS = new Set(['allow', 'deny']);

export class AclConfigError extends Error {
  constructor(message, options) {
    super(message, options);
    this.name = 'AclConfigError';
  }
}

function fail(where, detail) {
  throw new AclConfigError(`${where}: ${detail}`);
}

/**
 * Check the syntax of a topic filter used inside a rule file.
 */
export function validateRuleFilter(filter, where = 'topic') {
  if (typeof filter !== 'string' || filter.length === 0) {
    fail(where, 'must be a non-empty string');
  }
  if (filter.split('/')[0] === '$share') {
    fail(where, 'shared subscription wrappers do not belong in rule files');
  }
  if (!isValidFilter(filter)) {
    fail(where, 'wildcards must occupy a whole level and `#` must be last');
  }
  return filter;
}

function normalizeRule(rule, where) {
  if (!rule || typeof rule !== 'object' || Array.isArray(rule)) {
    fail(where, 'rule must be an object');
  }
  if (!ACTIONS.has(rule.action)) {
    fail(where, 'action must be "publish" or "subscribe"');
  }
  if (!EFFECTS.has(rule.effect)) {
    fail(where, 'effect must be "allow" or "deny"');
  }
  validateRuleFilter(rule.topic, `${where}.topic`);
  return {
    action: rule.action,
    topic: rule.topic,
    effect: rule.effect,
  };
}

function validateRuleset(raw, source) {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    fail(source, 'document must be a JSON object');
  }
  if (!raw.principals || typeof raw.principals !== 'object' || Array.isArray(raw.principals)) {
    fail(source, 'document must contain a "principals" object');
  }

  const principals = {};
  for (const [name, entry] of Object.entries(raw.principals)) {
    if (name.length === 0) {
      fail(source, 'principal names must be non-empty');
    }
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
      fail(`${source}#${name}`, 'principal entry must be an object');
    }
    if (!Array.isArray(entry.rules)) {
      fail(`${source}#${name}`, 'principal entry must contain a "rules" array');
    }
    principals[name] = {
      rules: entry.rules.map((rule, index) =>
        normalizeRule(rule, `${source}#${name}.rules[${index}]`),
      ),
    };
  }

  return { principals };
}

function mergeInto(target, incoming) {
  for (const [name, entry] of Object.entries(incoming.principals)) {
    if (!Object.hasOwn(target.principals, name)) {
      target.principals[name] = { rules: [] };
    }
    target.principals[name].rules.push(...entry.rules);
  }
}

/**
 * Load and merge every `*.json` file in a rule directory.
 * Files are processed in filename order so the merged rule set is stable.
 */
export async function loadAclDirectory(directory) {
  const names = (await readdir(directory))
    .filter((name) => name.endsWith('.json'))
    .sort((left, right) => left.localeCompare(right));

  const merged = { principals: {} };

  for (const name of names) {
    const source = join(directory, name);
    const text = await readFile(source, 'utf8');
    let raw;
    try {
      raw = JSON.parse(text);
    } catch (cause) {
      throw new AclConfigError(`invalid JSON in ${source}`, { cause });
    }
    mergeInto(merged, validateRuleset(raw, source));
  }

  return merged;
}

export function parseRuleset(raw, source = '<memory>') {
  return validateRuleset(raw, source);
}
