import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

import { isPlainObject, PolicyFormatError, normalizeRules } from './rules.js';

function extractRules(document, source) {
  if (Array.isArray(document)) {
    return document;
  }
  if (isPlainObject(document) && Array.isArray(document.rules)) {
    return document.rules;
  }
  throw new PolicyFormatError(`${source}: expected an array of rules or an object with a "rules" array`);
}

/**
 * Turn a parsed policy document into a frozen rule list.
 *
 * @param {unknown} document
 * @param {{ source?: string }} [options]
 * @returns {ReadonlyArray<Readonly<object>>}
 */
export function parsePolicyDocument(document, { source = 'policy' } = {}) {
  return normalizeRules(extractRules(document, source), { source });
}

/**
 * Read every `.json` document in a directory and return the combined rule
 * list ordered by file name and then by declaration order.
 *
 * @param {string} directory
 * @returns {Promise<ReadonlyArray<Readonly<object>>>}
 */
export async function loadPolicies(directory) {
  let entries;
  try {
    entries = await readdir(directory, { withFileTypes: true });
  } catch (error) {
    if (error && error.code === 'ENOENT') {
      throw new PolicyFormatError(`policy directory not found: ${directory}`);
    }
    throw error;
  }

  const files = entries
    .filter((entry) => entry.isFile() && entry.name.endsWith('.json'))
    .map((entry) => entry.name)
    .sort();

  const seen = new Set();
  const rules = [];

  for (const file of files) {
    const path = join(directory, file);
    const text = await readFile(path, 'utf8');

    let document;
    try {
      document = JSON.parse(text);
    } catch (error) {
      throw new PolicyFormatError(`${file}: invalid JSON`, { cause: error });
    }

    let documentRules;
    try {
      documentRules = parsePolicyDocument(document, { source: file });
    } catch (error) {
      if (error instanceof PolicyFormatError) {
        throw error;
      }
      throw new PolicyFormatError(`${file}: could not parse policy document`, { cause: error });
    }

    for (const rule of documentRules) {
      if (seen.has(rule.id)) {
        throw new PolicyFormatError(`${file}: duplicate rule id "${rule.id}"`);
      }
      seen.add(rule.id);
      rules.push(rule);
    }
  }

  return Object.freeze(rules);
}
