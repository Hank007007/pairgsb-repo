'use strict';

function fieldValue(object, name) {
  if (object === null || typeof object !== 'object') return undefined;
  if (Object.prototype.hasOwnProperty.call(object, name)) return object[name];
  const wanted = String(name).toLowerCase();
  for (const key of Object.keys(object)) {
    if (key.toLowerCase() === wanted) return object[key];
  }
  return undefined;
}

function collect(value, segments) {
  if (segments.length === 0) return [value];
  const [head, ...rest] = segments;
  if (Array.isArray(value)) {
    return value.flatMap((item) => collect(item, segments));
  }
  const next = fieldValue(value, head);
  if (next === undefined) return [];
  return collect(next, rest);
}

function isPresent(value) {
  if (value === undefined || value === null) return false;
  if (typeof value === 'string') return value.length > 0;
  if (Array.isArray(value)) return value.length > 0;
  return true;
}

function asText(value) {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  return String(value);
}

function equalsValue(left, right) {
  if (typeof left === 'string' && typeof right === 'string') {
    return left.toLowerCase() === right.toLowerCase();
  }
  if (typeof left === 'boolean' || typeof right === 'boolean') {
    return left === right;
  }
  if (typeof left === 'number' || typeof right === 'number') {
    return Number(left) === Number(right);
  }
  return left === right;
}

function compareOrdered(left, right) {
  if (typeof left === 'number' && typeof right === 'number') {
    return left - right;
  }
  const a = asText(left);
  const b = asText(right);
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

function likePatternToRegExp(pattern) {
  let source = '^';
  const text = String(pattern);
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (ch === '\\' && i + 1 < text.length) {
      i += 1;
      source += text[i].replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      continue;
    }
    if (ch === '%') {
      source += '.*';
    } else if (ch === '_') {
      source += '.';
    } else {
      source += ch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }
  }
  source += '$';
  return new RegExp(source, 'i');
}

function matchValue(value, operator, expected) {
  switch (operator) {
    case 'eq':
      return equalsValue(value, expected);
    case 'ne':
      return !equalsValue(value, expected);
    case 'co':
    case 'sw':
    case 'ew':
      return false;
    case 'gt':
      return compareOrdered(value, expected) > 0;
    case 'ge':
      return compareOrdered(value, expected) >= 0;
    case 'lt':
      return compareOrdered(value, expected) < 0;
    case 'le':
      return compareOrdered(value, expected) <= 0;
    default:
      return false;
  }
}

function matches(record, condition) {
  switch (condition.kind) {
    case 'and':
      return condition.items.every((item) => matches(record, item));
    case 'or':
      return condition.items.some((item) => matches(record, item));
    case 'not':
      return !matches(record, condition.item);
    case 'cmp': {
      const values = collect(record, condition.path.split('.'));
      return values.some((value) => matchValue(value, condition.op, condition.value));
    }
    case 'like': {
      const values = collect(record, condition.path.split('.'));
      const pattern = likePatternToRegExp(condition.pattern);
      return values.some((value) => typeof value === 'string' && pattern.test(value));
    }
    case 'pr': {
      const values = collect(record, condition.path.split('.'));
      return values.some(isPresent);
    }
    case 'element': {
      const values = collect(record, condition.path.split('.'));
      const elements = values.flatMap((value) => (Array.isArray(value) ? value : [value]));
      return elements.some((item) => matches(item, condition.condition));
    }
    default:
      return false;
  }
}

module.exports = {
  matches,
  collect,
  isPresent,
  equalsValue,
  fieldValue,
};
