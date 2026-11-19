import { filterCovers } from './match.js';

/**
 * Map an authenticated client identity to the principal keys used in rule
 * files. More specific principals are listed first; the result is a union, so
 * the order only affects reporting, not the outcome.
 */
export function principalKeys(identity = {}) {
  const keys = [];
  if (identity.cn) {
    keys.push(`cn:${identity.cn}`);
  }
  if (identity.username) {
    keys.push(`user:${identity.username}`);
  }
  if (identity.clientId) {
    keys.push(`client:${identity.clientId}`);
  }
  return keys;
}

/**
 * Reduce one principal's rule list to a single effect for the given action and
 * subject. `subject` is a concrete topic for `publish` and a filter for
 * `subscribe`.
 */
export function decide(rules, action, subject) {
  const candidates = Array.isArray(rules) ? rules : [];
  let allowedBy = null;
  let deniedBy = null;

  for (const rule of candidates) {
    if (rule.action !== action) {
      continue;
    }
    if (!filterCovers(rule.topic, subject)) {
      continue;
    }

    if (rule.effect === 'deny') {
      deniedBy = rule;
    } else if (allowedBy === null) {
      allowedBy = rule;
    }
  }

  if (deniedBy) {
    return { effect: 'deny', rule: deniedBy };
  }
  if (allowedBy) {
    return { effect: 'allow', rule: allowedBy };
  }
  return { effect: 'deny', rule: null };
}

/**
 * Combine the decisions for every principal attached to an identity.
 */
export function evaluate({ rules, identity, action, subject, principals }) {
  const names = principals ?? principalKeys(identity);
  const table = rules?.principals ?? {};
  let allowedBy = null;

  for (const name of names) {
    const entry = table[name];
    if (!entry) {
      continue;
    }

    const result = decide(entry.rules, action, subject);

    if (result.effect === 'deny' && result.rule) {
      return {
        allowed: false,
        effect: 'deny',
        reason: 'matched-deny',
        principal: name,
        rule: result.rule,
      };
    }

    if (result.effect === 'allow' && allowedBy === null) {
      allowedBy = { principal: name, rule: result.rule };
    }
  }

  if (allowedBy !== null) {
    return {
      allowed: true,
      effect: 'allow',
      reason: 'matched-allow',
      principal: allowedBy.principal,
      rule: allowedBy.rule,
    };
  }

  return {
    allowed: false,
    effect: 'deny',
    reason: 'default-deny',
    principal: null,
    rule: null,
  };
}
