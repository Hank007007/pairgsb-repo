/**
 * Expansion: enumerate the subjects that hold a relation on an object.
 *
 * Expansion is a structural walk over tuples. It answers "who?", not "may
 * this subject?". Operators that cannot be enumerated from tuples alone
 * (intersection, exclusion) are rejected.
 */

import { AuthError, ErrorCodes } from '../errors.js';
import {
  parseObjectRef,
  formatObjectRef,
  formatSubject,
  isPlainSubject,
  isWildcard,
} from '../tuple.js';
import { getCompiledNamespace } from '../model/compile.js';
import { read, parseContextualTuples } from '../store/tuples.js';
import { resolveZookie, currentVersion } from '../store/consistency.js';
import { decisionKey, getDecision, putDecision } from '../cache/decisions.js';

const MAX_DEPTH = 25;

function subjectsOf(store, tenant, object, relation, asOf, contextual, depth = 0) {
  const subjects = read(store, tenant, object, relation, { asOf }).map((subject) => ({ ...subject }));
  if (depth > 0) return subjects;
  for (const tuple of contextual) {
    if (tuple.object.type === object.type && tuple.object.id === object.id && tuple.relation === relation) {
      subjects.push({ ...tuple.subject });
    }
  }
  return subjects;
}

/**
 * Expand `object#relation`.
 *
 * @param {object} runtime
 * @param {string} tenant
 * @param {string} objectText
 * @param {string} relation
 * @param {{at_least?: string, contextualTuples?: Array}} [options]
 * @returns {{object: string, relation: string, subjects: string[]}}
 */
export function expand(runtime, tenant, objectText, relation, options = {}) {
  const object = parseObjectRef(objectText);
  const asOf = resolveZookie(runtime.tuples, tenant, options.at_least) ?? currentVersion(runtime.tuples, tenant);
  const contextual = parseContextualTuples(options.contextualTuples);

  const key = decisionKey({
    tenant,
    object,
    relation,
    subject: { type: '*', id: '*' },
  });
  const cached = getDecision(runtime.decisions, key);
  if (cached) {
    return cached.value;
  }
  const seen = new Set();
  const out = [];
  const addSubject = (subject) => {
    const text = formatSubject(subject);
    if (seen.has(text)) return;
    seen.add(text);
    out.push(text);
  };

  walk(runtime, tenant, object, relation, asOf, contextual, [], 0, addSubject);

  const result = { object: formatObjectRef(object), relation, subjects: out };
  putDecision(runtime.decisions, key, {
    tenant,
    value: result,
    snapshotVersion: currentVersion(runtime.tuples, tenant),
    namespaceVersions: {},
  });
  return result;
}

function walk(runtime, tenant, object, relation, asOf, contextual, stack, depth, addSubject) {
  if (depth >= MAX_DEPTH) {
    throw new AuthError(ErrorCodes.DEPTH_EXCEEDED, `expansion exceeded ${MAX_DEPTH} levels`, {
      object: formatObjectRef(object),
      relation,
    });
  }

  const plan = getCompiledNamespace(runtime.namespaces, tenant, object.type);
  const node = plan.relations.get(relation);
  if (!node) {
    throw new AuthError(
      ErrorCodes.INVALID_REQUEST,
      `namespace "${plan.namespace}" has no relation "${relation}"`,
      { namespace: plan.namespace, relation },
    );
  }

  const frame = `${formatObjectRef(object)}#${relation}`;
  if (stack.includes(frame)) {
    throw new AuthError(ErrorCodes.CYCLE_DETECTED, `cycle detected at ${frame}`, { frame });
  }
  stack.push(frame);

  if (node.kind === 'union') {
    for (const child of node.children) {
      walkNode(runtime, tenant, object, relation, child, asOf, contextual, stack, depth, addSubject);
    }
    stack.pop();
    return;
  }

  walkNode(runtime, tenant, object, relation, node, asOf, contextual, stack, depth, addSubject);
  stack.pop();
}

function walkNode(runtime, tenant, object, relation, node, asOf, contextual, stack, depth, addSubject) {
  const plan = getCompiledNamespace(runtime.namespaces, tenant, object.type);

  switch (node.kind) {
    case 'this':
      for (const subject of subjectsOf(runtime.tuples, tenant, object, relation, asOf, contextual, depth)) {
        if (isWildcard(subject)) continue;
        if (isPlainSubject(subject)) {
          addSubject(subject);
          continue;
        }
        const members = subjectsOf(
          runtime.tuples,
          tenant,
          { type: subject.type, id: subject.id },
          subject.relation,
          asOf,
          contextual,
          depth + 1,
        );
        if (members.length === 0) continue;
        for (const member of members) {
          if (isWildcard(member)) continue;
          addSubject(member);
        }
      }
      return;

    case 'computedUserset':
      walk(runtime, tenant, object, node.relation, asOf, contextual, stack, depth + 1, addSubject);
      return;

    case 'tupleToUserset':
      for (const subject of subjectsOf(runtime.tuples, tenant, object, node.tuplesetRelation, asOf, contextual, depth)) {
        if (!isPlainSubject(subject) || isWildcard(subject)) continue;
        walk(
          runtime,
          tenant,
          { type: subject.type, id: subject.id },
          node.computedRelation,
          asOf,
          contextual,
          stack,
          depth + 1,
          addSubject,
        );
      }
      return;

    case 'union':
      for (const child of node.children) {
        walkNode(runtime, tenant, object, relation, child, asOf, contextual, stack, depth, addSubject);
      }
      return;

    case 'intersection':
    case 'exclusion':
      throw new AuthError(
        ErrorCodes.INVALID_REQUEST,
        `relation "${relation}" on namespace "${plan.namespace}" cannot be expanded`,
        { namespace: plan.namespace, relation, operator: node.kind },
      );

    default:
      throw new AuthError(
        ErrorCodes.INVALID_NAMESPACE,
        `relation "${relation}" on namespace "${plan.namespace}" has an unsupported operator`,
        { namespace: plan.namespace, relation },
      );
  }
}
