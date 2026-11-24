/**
 * tupleresolve — relation-based authorization evaluation.
 *
 * The public surface is the service object from `service.js` plus the
 * low-level pieces the fixtures and tests use directly.
 */

export { createRuntime } from './runtime.js';
export { createService } from './service.js';
export { AuthError, ErrorCodes, isAuthError } from './errors.js';
export { check } from './graph/check.js';
export { expand } from './graph/expand.js';
export { write, deleteTuple as delete, read } from './store/tuples.js';
export { compileRewrite, compileNamespace, getCompiledNamespace } from './model/compile.js';
export { isFresh, bump, resolveZookie, currentVersion, currentZookie } from './store/consistency.js';
export { registerNamespace, listNamespaces, getNamespaceDocument } from './model/registry.js';
export { createDecisionCache, decisionKey, getDecision, putDecision, clearDecisions } from './cache/decisions.js';
export { parseTuple, parseObjectRef, parseSubject, formatTupleParts } from './tuple.js';
