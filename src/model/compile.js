/**
 * Namespace compilation.
 *
 * A namespace document's rewrite expressions are turned into an immutable
 * tree of operator nodes that the evaluator walks:
 *
 *   { kind: 'this' }
 *   { kind: 'computedUserset', relation }
 *   { kind: 'tupleToUserset', tuplesetRelation, computedRelation }
 *   { kind: 'union', children: [...] }
 *   { kind: 'intersection', children: [...] }
 *   { kind: 'exclusion', base, subtract }
 *
 * The plan also records the namespace name and configuration version so that
 * callers can reason about which configuration produced a decision.
 */

import { AuthError, ErrorCodes } from '../errors.js';
import { isValidRelation } from '../tuple.js';
import { getTenantSpace, getNamespaceDocument } from './registry.js';

function invalid(name, relation, message) {
  throw new AuthError(ErrorCodes.INVALID_NAMESPACE, message, { namespace: name, relation });
}

function compileChildren(name, relation, operator, children) {
  if (!Array.isArray(children)) {
    invalid(name, relation, `namespace "${name}" relation "${relation}": "${operator}" expects an array`);
  }
  return children.map((child) => compileRewrite(child, { namespace: name, relation }));
}

/**
 * Compile one rewrite expression.
 *
 * @param {object} rewrite
 * @param {{namespace?: string, relation?: string}} [context]
 * @returns {object} operator node
 */
export function compileRewrite(rewrite, context = {}) {
  const namespace = context.namespace ?? '<inline>';
  const relation = context.relation ?? '<inline>';

  if (!rewrite || typeof rewrite !== 'object' || Array.isArray(rewrite)) {
    invalid(namespace, relation, `namespace "${namespace}" relation "${relation}": rewrite must be an object`);
  }

  const operators = Object.keys(rewrite);
  if (operators.length === 0) {
    invalid(namespace, relation, `namespace "${namespace}" relation "${relation}": rewrite has no operator`);
  }
  if (operators.length > 1) {
    invalid(
      namespace,
      relation,
      `namespace "${namespace}" relation "${relation}": rewrite has more than one operator ` +
        `(${operators.join(', ')})`,
    );
  }

  const operator = operators[0];
  const operand = rewrite[operator];

  switch (operator) {
    case 'this':
      return { kind: 'this' };

    case 'computedUserset': {
      const target = operand && typeof operand === 'object' ? operand.relation : undefined;
      if (!isValidRelation(target)) {
        invalid(
          namespace,
          relation,
          `namespace "${namespace}" relation "${relation}": "computedUserset" requires a valid "relation"`,
        );
      }
      return { kind: 'computedUserset', relation: target };
    }

    case 'tupleToUserset': {
      const tupleset = operand && typeof operand === 'object' ? operand.tupleset : undefined;
      const computed = operand && typeof operand === 'object' ? operand.computedUserset : undefined;
      const tuplesetRelation = tupleset && typeof tupleset === 'object' ? tupleset.relation : undefined;
      const computedRelation = computed && typeof computed === 'object' ? computed.relation : undefined;
      if (!isValidRelation(tuplesetRelation) || !isValidRelation(computedRelation)) {
        invalid(
          namespace,
          relation,
          `namespace "${namespace}" relation "${relation}": "tupleToUserset" requires "tupleset.relation" and ` +
            `"computedUserset.relation"`,
        );
      }
      return { kind: 'tupleToUserset', tuplesetRelation, computedRelation };
    }

    case 'union': {
      if (!Array.isArray(operand)) {
        invalid(namespace, relation, `namespace "${namespace}" relation "${relation}": "union" expects an array`);
      }
      return { kind: 'union', children: compileChildren(namespace, relation, 'union', operand) };
    }

    case 'intersection': {
      if (!Array.isArray(operand)) {
        invalid(namespace, relation, `namespace "${namespace}" relation "${relation}": "intersection" expects an array`);
      }
      return { kind: 'union', children: compileChildren(namespace, relation, 'intersection', operand) };
    }

    case 'exclusion': {
      if (!operand || typeof operand !== 'object' || Array.isArray(operand)) {
        invalid(namespace, relation, `namespace "${namespace}" relation "${relation}": "exclusion" expects an object`);
      }
      if (!('base' in operand) || !('subtract' in operand)) {
        invalid(
          namespace,
          relation,
          `namespace "${namespace}" relation "${relation}": "exclusion" requires "base" and "subtract"`,
        );
      }
      return {
        kind: 'exclusion',
        base: compileRewrite(operand.base, { namespace, relation }),
        subtract: compileRewrite(operand.subtract, { namespace, relation }),
      };
    }

    default:
      return {
        kind: 'union',
        children: Array.isArray(operand) ? compileChildren(namespace, relation, operator, operand) : [],
      };
  }
}

function assertRelationsExist(name, relation, node, relations) {
  switch (node.kind) {
    case 'computedUserset':
      if (!Object.prototype.hasOwnProperty.call(relations, node.relation)) {
        invalid(name, relation, `namespace "${name}" relation "${relation}": unknown relation "${node.relation}"`);
      }
      return;
    case 'tupleToUserset':
      for (const target of [node.tuplesetRelation, node.computedRelation]) {
        if (!Object.prototype.hasOwnProperty.call(relations, target)) {
          invalid(name, relation, `namespace "${name}" relation "${relation}": unknown relation "${target}"`);
        }
      }
      return;
    case 'union':
      for (const child of node.children) assertRelationsExist(name, relation, child, relations);
      return;
    case 'exclusion':
      assertRelationsExist(name, relation, node.base, relations);
      assertRelationsExist(name, relation, node.subtract, relations);
      return;
    default:
      return;
  }
}

/**
 * Compile a namespace document into an immutable plan.
 *
 * @param {{name: string, version: number, relations: object}} document
 * @returns {{namespace: string, version: number, relations: Map<string, object>}}
 */
export function compileNamespace(document) {
  const name = document.name;
  const relations = document.relations;

  const compiledRelations = new Map();
  for (const [relation, rewrite] of Object.entries(relations)) {
    if (!isValidRelation(relation)) {
      invalid(name, relation, `namespace "${name}": relation name "${relation}" is invalid`);
    }
    compiledRelations.set(relation, compileRewrite(rewrite, { namespace: name, relation }));
  }

  for (const [relation, node] of compiledRelations) {
    assertRelationsExist(name, relation, node, relations);
  }

  return Object.freeze({ namespace: name, version: document.version, relations: compiledRelations });
}

/**
 * Get the compiled plan for a namespace, compiling and caching it on first
 * use.
 *
 * @param {{spaces: Map}} registry
 * @param {string} tenant
 * @param {string} name
 * @returns {{namespace: string, version: number, relations: Map<string, object>}}
 */
export function getCompiledNamespace(registry, tenant, name) {
  const space = getTenantSpace(registry, tenant, false);
  if (!space) {
    throw new AuthError(ErrorCodes.NAMESPACE_NOT_FOUND, `namespace "${name}" is not registered`, { namespace: name });
  }

  let plan = space.compiled.get(name);
  if (plan) return plan;

  const document = getNamespaceDocument(registry, tenant, name);
  if (!document) {
    throw new AuthError(ErrorCodes.NAMESPACE_NOT_FOUND, `namespace "${name}" is not registered`, { namespace: name });
  }

  plan = compileNamespace(document);
  space.compiled.set(name, plan);
  return plan;
}

/** Drop every compiled plan for a tenant (used by administrative tooling). */
export function clearCompiledCache(registry, tenant) {
  const space = getTenantSpace(registry, tenant, false);
  if (space) space.compiled.clear();
}
