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

const MAX_DEPTH = 25;

function subjectsOf(store, tenant, object, relation, asOf, contextual) {
  const subjects = read(store, tenant, object, relation, { asOf }).map((subject) => ({ ...subject }));
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

  const seen = new Set();
  const out = [];
  const addSubject = (subject) => {
    const text = formatSubject(subject);
    if (seen.has(text)) return;
    seen.add(text);
    out.push(text);
  };

  walk(runtime, tenant, object, relation, asOf, contextual, [], 0, addSubject);

  return { object: formatObjectRef(object), relation, subjects: out };
}

function missingNamespace(error) {
  return error instanceof AuthError && error.code === ErrorCodes.NAMESPACE_NOT_FOUND;
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
      for (const subject of subjectsOf(runtime.tuples, tenant, object, relation, asOf, contextual)) {
        if (isPlainSubject(subject)) {
          addSubject(subject);
          continue;
        }
        try {
          walk(
            runtime,
            tenant,
            { type: subject.type, id: subject.id },
            subject.relation,
            asOf,
            contextual,
            stack,
            depth + 1,
            addSubject,
          );
        } catch (error) {
          if (missingNamespace(error)) continue;
          throw error;
        }
      }
      return;

    case 'computedUserset':
      walk(runtime, tenant, object, node.relation, asOf, contextual, stack, depth + 1, addSubject);
      return;

    case 'tupleToUserset':
      if (plan.relations.get(node.tuplesetRelation)?.kind !== 'this') {
        throw new AuthError(
          ErrorCodes.INVALID_NAMESPACE,
          `relation "${relation}" on namespace "${plan.namespace}": tupleToUserset reads ` +
            `"${node.tuplesetRelation}", which is not a direct tuple relation`,
          { namespace: plan.namespace, relation },
        );
      }
      for (const subject of subjectsOf(runtime.tuples, tenant, object, node.tuplesetRelation, asOf, contextual)) {
        if (!isPlainSubject(subject) || isWildcard(subject)) continue;
        try {
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
        } catch (error) {
          if (missingNamespace(error)) continue;
          throw error;
        }
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
