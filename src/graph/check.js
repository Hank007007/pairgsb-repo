/**
 * Evaluation of `check(object, relation, subject)`.
 *
 * The evaluator walks the compiled rewrite for the requested relation. Every
 * tuple read is performed by `this`; every other operator either rewrites to
 * another relation on the same object (`computedUserset`), follows tuples to
 * another object (`tupleToUserset`), or combines the results of its children.
 *
 * `check` is the entry point for a request. `checkUserset` evaluates one
 * `object#relation` frame together with its subject; nested userset subjects
 * and `computedUserset` hops recurse through it.
 */

import { AuthError, ErrorCodes } from '../errors.js';
import {
  parseObjectRef,
  parseSubject,
  formatObjectRef,
  isPlainSubject,
  isWildcard,
  subjectMatches,
} from '../tuple.js';
import { getCompiledNamespace } from '../model/compile.js';
import { read, parseContextualTuples } from '../store/tuples.js';
import { resolveZookie, currentVersion } from '../store/consistency.js';
import { decisionKey, getDecision, putDecision } from '../cache/decisions.js';

const MAX_DEPTH = 25;

function objectName(object) {
  return formatObjectRef(object);
}

function frameName(object, relation) {
  return `${objectName(object)}#${relation}`;
}

/**
 * Read the subjects stored for `object#relation`, together with the
 * contextual tuples that belong to this request.
 */
function subjectsFor(context, object, relation, depth) {
  const subjects = read(context.runtime.tuples, context.tenant, object, relation, {
    asOf: context.asOf,
  }).map((subject) => ({ ...subject }));

  if (depth > 0) return subjects;

  for (const tuple of context.contextual) {
    if (tuple.object.type === object.type && tuple.object.id === object.id && tuple.relation === relation) {
      subjects.push({ ...tuple.subject });
    }
  }

  return subjects;
}

function planFor(context, object, relation) {
  try {
    return getCompiledNamespace(context.runtime.namespaces, context.tenant, object.type);
  } catch (error) {
    if (error instanceof AuthError && error.code === ErrorCodes.NAMESPACE_NOT_FOUND) {
      throw new AuthError(ErrorCodes.NAMESPACE_NOT_FOUND, `namespace "${object.type}" is not registered`, {
        namespace: object.type,
        object: objectName(object),
        relation,
      });
    }
    throw error;
  }
}

function missingNamespace(error) {
  return error instanceof AuthError && error.code === ErrorCodes.NAMESPACE_NOT_FOUND;
}

/**
 * Check whether `subject` is a member of `object#relation`.
 *
 * @param {object} runtime
 * @param {string} tenant
 * @param {string} objectText `type:id`
 * @param {string} relation
 * @param {string} subjectText plain subject (`type:id` or `type:*`)
 * @param {{at_least?: string, contextualTuples?: Array}} [options]
 * @returns {boolean}
 */
export function check(runtime, tenant, objectText, relation, subjectText, options = {}) {
  const object = parseObjectRef(objectText);
  const subject = parseSubject(subjectText);

  if (subject.kind !== 'user') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'subject must be a plain subject; usersets belong inside tuples', {
      subject: subjectText,
    });
  }

  const context = {
    runtime,
    tenant,
    subject,
    asOf: resolveZookie(runtime.tuples, tenant, options.at_least) ?? currentVersion(runtime.tuples, tenant),
    contextual: parseContextualTuples(options.contextualTuples),
  };

  // Resolve the namespace up front so that a missing namespace or an unknown
  // relation is reported even when a cache entry exists for the triple.
  const plan = planFor(context, object, relation);
  if (!plan.relations.has(relation)) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, `namespace "${plan.namespace}" has no relation "${relation}"`, {
      namespace: plan.namespace,
      relation,
    });
  }

  const key = decisionKey({ tenant, object, relation, subject });
  const cached = getDecision(runtime.decisions, key);
  if (cached) {
    return cached.value;
  }

  const value = checkUserset(context, object, relation, 0, new Set());

  putDecision(runtime.decisions, key, {
    tenant,
    value,
    snapshotVersion: currentVersion(runtime.tuples, tenant),
    namespaceVersions: { [plan.namespace]: plan.version },
  });

  return value;
}

/**
 * Evaluate one `object#relation` frame for `context.subject`.
 *
 * @param {object} context evaluation context: runtime, tenant, subject, asOf, contextual
 * @param {{type: string, id: string}} object
 * @param {string} relation
 * @param {number} depth frames entered so far
 * @param {Set<string>} visited objects on the current branch
 * @returns {boolean}
 */
export function checkUserset(context, object, relation, depth = 0, visited = new Set()) {
  if (depth >= MAX_DEPTH) {
    return false;
  }

  const frame = frameName(object, relation);
  if (visited.has(objectName(object))) {
    throw new AuthError(ErrorCodes.CYCLE_DETECTED, `cycle detected at ${frame}`, { frame });
  }

  const plan = planFor(context, object, relation);
  const node = plan.relations.get(relation);
  if (!node) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, `namespace "${plan.namespace}" has no relation "${relation}"`, {
      namespace: plan.namespace,
      relation,
    });
  }

  visited.add(objectName(object));
  try {
    return evalNode(node, object, relation, plan, context, depth, visited);
  } finally {
    visited.delete(objectName(object));
  }
}

function evalNode(node, object, relation, plan, context, depth, visited) {
  switch (node.kind) {
    case 'this':
      return checkDirect(object, relation, context, depth, visited);
    case 'computedUserset':
      return checkUserset(context, object, node.relation, depth + 1, visited);
    case 'tupleToUserset':
      return checkTupleToUserset(node, object, relation, plan, context, depth, visited);
    case 'union':
      return checkUnion(node, object, relation, plan, context, depth, visited);
    case 'intersection':
      return checkIntersection(node, object, relation, plan, context, depth, visited);
    case 'exclusion':
      return checkExclusion(node, object, relation, plan, context, depth, visited);
    default:
      throw new AuthError(
        ErrorCodes.INVALID_NAMESPACE,
        `relation "${relation}" on namespace "${plan.namespace}" has an unsupported operator`,
        { namespace: plan.namespace, relation },
      );
  }
}

function checkDirect(object, relation, context, depth, visited) {
  for (const stored of subjectsFor(context, object, relation, depth)) {
    if (isWildcard(stored)) {
      if (stored.type === context.subject.type) return true;
      continue;
    }
    if (isPlainSubject(stored)) {
      if (subjectMatches(stored, context.subject)) return true;
      continue;
    }
    try {
      const member = checkUserset(context, { type: stored.type, id: stored.id }, stored.relation, depth + 1, visited);
      if (member) return true;
    } catch (error) {
      if (missingNamespace(error)) continue;
      throw error;
    }
  }
  return false;
}

function checkTupleToUserset(node, object, relation, plan, context, depth, visited) {
  const tupleset = plan.relations.get(node.tuplesetRelation);
  if (!tupleset || tupleset.kind !== 'this') {
    throw new AuthError(
      ErrorCodes.INVALID_NAMESPACE,
      `relation "${relation}" on namespace "${plan.namespace}": tupleToUserset reads "${node.tuplesetRelation}", ` +
        'which is not a direct tuple relation',
      { namespace: plan.namespace, relation },
    );
  }

  for (const target of subjectsFor(context, object, node.tuplesetRelation, depth)) {
    if (!isPlainSubject(target) || isWildcard(target)) continue;
    try {
      const member = checkUserset(context, { type: target.type, id: target.id }, node.computedRelation, depth + 1, visited);
      if (member) return true;
    } catch (error) {
      if (missingNamespace(error)) continue;
      throw error;
    }
  }
  return false;
}

function checkUnion(node, object, relation, plan, context, depth, visited) {
  for (const child of node.children) {
    if (evalNode(child, object, relation, plan, context, depth, visited)) return true;
  }
  return false;
}

function checkIntersection(node, object, relation, plan, context, depth, visited) {
  for (const child of node.children) {
    if (!evalNode(child, object, relation, plan, context, depth, visited)) return false;
  }
  return true;
}

function checkExclusion(node, object, relation, plan, context, depth, visited) {
  const base = evalNode(node.base, object, relation, plan, context, depth, visited);
  if (!base) return false;

  try {
    return !evalNode(node.subtract, object, relation, plan, context, depth, visited);
  } catch (error) {
    if (error instanceof AuthError) return true;
    throw error;
  }
}
