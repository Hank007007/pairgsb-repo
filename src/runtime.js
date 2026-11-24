/**
 * Runtime composition.
 *
 * A runtime bundles the three pieces of state the evaluator needs:
 *
 *   namespaces  namespace documents, versions, compiled plans
 *   tuples      tuple storage and the mutation clock
 *   decisions   decision cache
 *
 * Tests build isolated runtimes; the HTTP server owns exactly one.
 */

import { createRegistry } from './model/registry.js';
import { createTupleStore } from './store/tuples.js';
import { createDecisionCache } from './cache/decisions.js';

export function createRuntime() {
  return {
    namespaces: createRegistry(),
    tuples: createTupleStore(),
    decisions: createDecisionCache(),
  };
}
