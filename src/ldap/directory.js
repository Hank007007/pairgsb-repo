function asArray(value) {
  if (value === undefined || value === null) return [];
  return Array.isArray(value) ? value : [value];
}

function normalizeEntry(entry) {
  const attributes = {};
  for (const [key, value] of Object.entries(entry.attributes ?? {})) {
    attributes[key.toLowerCase()] = asArray(value).map(String);
  }
  const password = entry.password ?? attributes.userpassword?.[0];
  if (password !== undefined) {
    attributes.userpassword = [String(password)];
  }
  return {
    dn: entry.dn,
    attributes,
    password: password === undefined ? undefined : String(password),
  };
}

function regexEscape(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function valueEquals(attribute, actual, expected) {
  if (attribute === 'userpassword') return actual === expected;
  return actual.toLowerCase() === expected.toLowerCase();
}

function matchesLeaf(node, entry) {
  const values = entry.attributes[node.attribute.toLowerCase()] ?? [];
  switch (node.type) {
    case 'present':
      return values.length > 0;
    case 'equal':
      return values.some((value) => valueEquals(node.attribute.toLowerCase(), value, node.value));
    case 'substring': {
      let pattern = `^${regexEscape(node.initial ?? '')}`;
      for (const part of node.any ?? []) {
        pattern += `.*${regexEscape(part)}`;
      }
      pattern += node.final ? `.*${regexEscape(node.final)}` : '.*';
      pattern += '$';
      const expression = new RegExp(pattern, 'i');
      return values.some((value) => expression.test(value));
    }
    case 'compare':
      return values.some((value) => {
        const left = value.toLowerCase();
        const right = String(node.value).toLowerCase();
        if (node.operator === '>=') return left >= right;
        if (node.operator === '<=') return left <= right;
        return left === right;
      });
    default:
      return false;
  }
}

export function matchesFilter(node, entry) {
  if (!node) return false;
  switch (node.type) {
    case 'and':
      return node.children.every((child) => matchesFilter(child, entry));
    case 'or':
      return node.children.some((child) => matchesFilter(child, entry));
    case 'not':
      return !matchesFilter(node.child, entry);
    default:
      return matchesLeaf(node, entry);
  }
}

export function createDirectory(entries) {
  return entries.map(normalizeEntry);
}
