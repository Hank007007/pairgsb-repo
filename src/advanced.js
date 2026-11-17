import { config as defaultConfig } from './config.js';
import { protocolError } from './ldap/errors.js';
import { parseFilter } from './ldap/filter-parser.js';
import {
  search,
  validateSearchBase,
  validateSearchCount,
  validateSearchScope,
} from './search.js';

const RESTRICTED_ATTRIBUTES = new Set(['userpassword', '2.5.4.35']);

function baseAttribute(attribute) {
  return String(attribute ?? '').split(';', 1)[0].trim().toLowerCase();
}

function isRestrictedAttribute(attribute) {
  return RESTRICTED_ATTRIBUTES.has(baseAttribute(attribute));
}

function forEachNode(node, visit) {
  visit(node);
  if (node.type === 'and' || node.type === 'or') {
    for (const child of node.children) forEachNode(child, visit);
  } else if (node.type === 'not') {
    forEachNode(node.child, visit);
  }
}

function hasEmptyLogicalNode(node) {
  if ((node.type === 'and' || node.type === 'or') && node.children.length === 0) {
    return true;
  }
  if (node.type === 'and' || node.type === 'or') {
    return node.children.some(hasEmptyLogicalNode);
  }
  if (node.type === 'not') return hasEmptyLogicalNode(node.child);
  return false;
}

function isTautology(node) {
  switch (node.type) {
    case 'present':
      return true;
    case 'substring':
      return !hasSubstringConstraint(node);
    case 'and':
      return node.children.every(isTautology);
    case 'or':
      return node.children.some(isTautology);
    case 'not':
      return isContradiction(node.child);
    default:
      return false;
  }
}

function isContradiction(node) {
  switch (node.type) {
    case 'and':
      return node.children.some(isContradiction);
    case 'or':
      return node.children.every(isContradiction);
    case 'not':
      return isTautology(node.child);
    default:
      return false;
  }
}

function hasValueConstraint(node) {
  if (node.type === 'substring') return hasSubstringConstraint(node);
  if (node.type === 'equal' || node.type === 'compare') {
    return true;
  }
  if (node.type === 'and' || node.type === 'or') {
    return node.children.some(hasValueConstraint);
  }
  if (node.type === 'not') return hasValueConstraint(node.child);
  return false;
}

function hasSubstringConstraint(node) {
  return [node.initial, ...(node.any ?? []), node.final].some((part) => part !== '');
}

function validateFilterTree(tree) {
  forEachNode(tree, (node) => {
    if (node.attribute && isRestrictedAttribute(node.attribute)) {
      throw protocolError('filter references a restricted attribute', {
        attribute: String(node.attribute),
      });
    }
  });
  if (hasEmptyLogicalNode(tree)) {
    throw protocolError('filter contains an empty logical group');
  }
  if (isTautology(tree)) {
    throw protocolError('filter is too broad');
  }
  if (!hasValueConstraint(tree)) {
    throw protocolError('filter must constrain an attribute value');
  }
}

function normalizeAttributes(attributes) {
  if (attributes === undefined || attributes === null) return undefined;
  if (!Array.isArray(attributes)) {
    throw protocolError('attributes must be an array');
  }
  const normalized = [...attributes];
  for (const attribute of normalized) {
    if (isRestrictedAttribute(attribute)) {
      throw protocolError('requested attributes include a restricted attribute', {
        attribute: String(attribute),
      });
    }
  }
  return normalized;
}

export function parseAdvanced(input = {}, config = defaultConfig) {
  const settings = config.advanced;
  const filter = String(input.filter ?? '').trim();
  if (filter === '') {
    throw protocolError('advanced search requires a filter');
  }
  const filterLength = Buffer.byteLength(filter, 'utf8');
  if (filterLength > settings.maxFilterLength) {
    throw protocolError('advanced filter exceeds the maximum length', {
      length: filterLength,
      maxFilterLength: settings.maxFilterLength,
    });
  }
  const tree = parseFilter(filter);
  validateFilterTree(tree);
  const base = input.base ?? settings.base;
  const scope = input.scope ?? settings.scope ?? 'sub';
  validateSearchBase(base, config);
  validateSearchScope(scope);
  const pageSize = input.pageSize ?? settings.pageSize;
  const sizeLimit = input.sizeLimit ?? settings.sizeLimit;
  if (pageSize !== undefined && pageSize !== null) validateSearchCount(pageSize, 'pageSize');
  if (sizeLimit !== undefined && sizeLimit !== null) validateSearchCount(sizeLimit, 'sizeLimit');
  return {
    base,
    scope,
    filter,
    attributes: normalizeAttributes(input.attributes),
    pageSize,
    sizeLimit,
  };
}

export async function advancedSearch(client, input = {}, config = defaultConfig) {
  const parsed = parseAdvanced(input, config);
  return search(
    client,
    {
      base: parsed.base,
      scope: parsed.scope,
      filter: parsed.filter,
      attributes: parsed.attributes,
      pageSize: parsed.pageSize,
      sizeLimit: parsed.sizeLimit,
    },
    config,
  );
}
