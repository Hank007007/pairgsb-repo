'use strict';

// Policy evaluation for secret injection.
//
// A policy document has the shape:
//   {
//     scopes: [{ name, priority }],
//     rules:  [{ effect, pipeline, branch, scope }]
//   }
//
// `allows` decides whether a single scope may be injected for a build.
// `resolveScopes` folds several granted scopes into one variable map.

function escapeRegExp(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function globToRegExp(pattern) {
  let source = '';
  for (const ch of String(pattern)) {
    if (ch === '*') source += '.*';
    else source += escapeRegExp(ch);
  }
  return new RegExp('^' + source + '$');
}

function ruleMatches(rule, pipeline, branch, scope) {
  if (rule.scope !== scope) return false;
  if (!globToRegExp(rule.pipeline).test(pipeline)) return false;
  if (!globToRegExp(rule.branch).test(branch)) return false;
  return true;
}

function allows(policy, context) {
  const pipeline = context.pipeline;
  const branch = context.branch;
  const scope = context.scope;
  let result = false;

  for (const rule of policy.rules || []) {
    if (!ruleMatches(rule, pipeline, branch, scope)) continue;
    if (rule.effect === 'allow') result = true;
    else if (rule.effect === 'deny') result = false;
  }

  return result;
}

// grants: [{ scope: 'name', values: { KEY: 'value' } }, ...]
// scopeDeclarations: the policy's `scopes` array.
function resolveScopes(grants, scopeDeclarations) {
  const merged = {};

  for (const grant of grants || []) {
    const values = grant.values || {};
    for (const name of Object.keys(values)) {
      if (!Object.prototype.hasOwnProperty.call(merged, name)) {
        merged[name] = values[name];
      }
    }
  }

  return merged;
}

module.exports = {
  allows,
  resolveScopes,
  globToRegExp,
};
