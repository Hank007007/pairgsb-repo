export class ExpressionError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ExpressionError';
  }
}

export const ALLOWED_ROOTS = Object.freeze(['github', 'env', 'matrix', 'needs']);

const ALLOWED_ROOT_SET = new Set(ALLOWED_ROOTS);
const FORBIDDEN_SEGMENTS = new Set(['__proto__', 'prototype', 'constructor']);

export function isExpressionWrapper(text) {
  return /\$\{\{/.test(String(text));
}

export function unwrapExpression(text) {
  const source = String(text).trim();
  const match = /^\$\{\{([\s\S]*)\}\}$/.exec(source);
  if (match) return match[1].trim();
  return source;
}

export function evaluate(source, contexts = {}, options = {}) {
  if (options.secrets) {
    assertNoSecretReferences(source, options.secrets, options.field ?? 'expression');
  }

  const text = unwrapExpression(source);
  if (text === '') {
    throw new ExpressionError('empty expression');
  }

  const node = parseExpression(text);
  const value = evaluateNode(node, contexts);

  if (typeof value === 'function') {
    throw new ExpressionError('expression produced a function');
  }
  if (typeof value === 'symbol') {
    throw new ExpressionError('expression produced a symbol');
  }
  if (typeof value === 'bigint') {
    throw new ExpressionError('expression produced an unsupported number');
  }
  return value;
}

export function resolvePath(path, contexts = {}) {
  const text = String(path).trim();
  const segments = tokenizePath(text);
  return resolveSegments(segments, contexts, text);
}

function resolveSegments(segments, contexts, text) {
  const root = segments[0];

  if (!ALLOWED_ROOT_SET.has(root)) {
    throw new ExpressionError(`unknown context "${root}"`);
  }
  if (!Object.prototype.hasOwnProperty.call(contexts, root)) {
    throw new ExpressionError(`context "${root}" is not available`);
  }

  let current = contexts[root];
  for (let index = 1; index < segments.length; index += 1) {
    const segment = segments[index];
    if (typeof segment === 'string' && FORBIDDEN_SEGMENTS.has(segment)) {
      throw new ExpressionError(`"${text}" may not read "${segment}"`);
    }
    if (current === null || current === undefined) {
      throw new ExpressionError(`cannot read "${segment}" of ${String(current)} in "${text}"`);
    }
    if (typeof segment === 'number') {
      if (!Array.isArray(current)) {
        throw new ExpressionError(`cannot index a non-array in "${text}"`);
      }
      if (segment >= current.length) {
        throw new ExpressionError(`index ${segment} is out of range in "${text}"`);
      }
      current = current[segment];
      continue;
    }
    if (typeof current !== 'object') {
      throw new ExpressionError(`cannot read "${segment}" of ${String(current)} in "${text}"`);
    }
    if (!Object.prototype.hasOwnProperty.call(current, segment)) {
      throw new ExpressionError(`"${text}" is not defined`);
    }
    current = current[segment];
  }

  if (current === undefined) {
    throw new ExpressionError(`"${text}" is not defined`);
  }
  return current;
}

function tokenizePath(text) {
  const segments = [];
  let index = 0;

  const readIdentifier = () => {
    const match = /^[A-Za-z_$][\w$]*/.exec(text.slice(index));
    if (!match) throw new ExpressionError(`malformed path "${text}"`);
    index += match[0].length;
    return match[0];
  };

  segments.push(readIdentifier());
  while (index < text.length) {
    const char = text[index];
    if (char === '.') {
      index += 1;
      segments.push(readIdentifier());
      continue;
    }
    if (char === '[') {
      const close = text.indexOf(']', index);
      if (close === -1) throw new ExpressionError(`malformed path "${text}"`);
      const inner = text.slice(index + 1, close).trim();
      if (!/^\d+$/.test(inner)) {
        throw new ExpressionError(`only numeric indexes are supported in "${text}"`);
      }
      segments.push(Number(inner));
      index = close + 1;
      continue;
    }
    throw new ExpressionError(`malformed path "${text}"`);
  }
  return segments;
}

export function truthy(value) {
  if (value === null || value === undefined || value === false) return false;
  if (typeof value === 'number' && (value === 0 || Number.isNaN(value))) return false;
  if (typeof value === 'string' && value === '') return false;
  return true;
}

export function toText(value) {
  if (value === null || value === undefined) return 'null';
  if (typeof value === 'string') return value;
  if (typeof value === 'number') {
    if (Number.isNaN(value)) throw new ExpressionError('NaN cannot be rendered as text');
    return String(value);
  }
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  throw new ExpressionError('objects and arrays cannot be rendered as text');
}

export function assertNoSecretReferences(source, secrets = null, field = 'expression') {
  const text = String(source);
  const pattern = /\bsecrets\b\s*(?:\.\s*([A-Za-z_][A-Za-z0-9_-]*)|\[\s*(['"])(.*?)\2\s*\])/g;
  const match = pattern.exec(text);
  if (!match) return;
  const name = match[1] ?? match[3] ?? '';
  throw new ExpressionError(
    `"secrets.${name}" may only be used as an env or with value (found in ${field})`,
  );
}

const STRING_ESCAPES = {
  n: '\n',
  t: '\t',
  r: '\r',
  '\\': '\\',
  "'": "'",
  '"': '"',
};

function parseExpression(text) {
  const state = { text, index: 0 };
  const node = parseOr(state);
  skipSpace(state);
  if (state.index < text.length) {
    throw new ExpressionError(`malformed expression: unexpected "${text.slice(state.index)}"`);
  }
  return node;
}

function skipSpace(state) {
  while (state.index < state.text.length && /\s/.test(state.text[state.index])) {
    state.index += 1;
  }
}

function matchOperator(state, operator) {
  skipSpace(state);
  if (state.text.startsWith(operator, state.index)) {
    state.index += operator.length;
    return true;
  }
  return false;
}

function parseOr(state) {
  let left = parseAnd(state);
  while (matchOperator(state, '||')) {
    left = { type: 'or', left, right: parseAnd(state) };
  }
  return left;
}

function parseAnd(state) {
  let left = parseEquality(state);
  while (matchOperator(state, '&&')) {
    left = { type: 'and', left, right: parseEquality(state) };
  }
  return left;
}

function parseEquality(state) {
  let left = parseRelational(state);
  for (;;) {
    if (matchOperator(state, '==')) {
      left = { type: 'compare', op: '==', left, right: parseRelational(state) };
    } else if (matchOperator(state, '!=')) {
      left = { type: 'compare', op: '!=', left, right: parseRelational(state) };
    } else {
      return left;
    }
  }
}

function parseRelational(state) {
  let left = parseUnary(state);
  for (;;) {
    if (matchOperator(state, '<=')) {
      left = { type: 'compare', op: '<=', left, right: parseUnary(state) };
    } else if (matchOperator(state, '>=')) {
      left = { type: 'compare', op: '>=', left, right: parseUnary(state) };
    } else if (matchOperator(state, '<')) {
      left = { type: 'compare', op: '<', left, right: parseUnary(state) };
    } else if (matchOperator(state, '>')) {
      left = { type: 'compare', op: '>', left, right: parseUnary(state) };
    } else {
      return left;
    }
  }
}

function parseUnary(state) {
  skipSpace(state);
  if (state.text[state.index] === '!' && state.text[state.index + 1] !== '=') {
    state.index += 1;
    return { type: 'not', operand: parseUnary(state) };
  }
  return parsePrimary(state);
}

function parsePrimary(state) {
  skipSpace(state);
  const char = state.text[state.index];
  if (char === undefined) {
    throw new ExpressionError('malformed expression: unexpected end of input');
  }
  if (char === '(') {
    state.index += 1;
    const node = parseOr(state);
    skipSpace(state);
    if (state.text[state.index] !== ')') {
      throw new ExpressionError('malformed expression: missing ")"');
    }
    state.index += 1;
    return node;
  }
  if (char === "'" || char === '"') {
    return { type: 'literal', value: readString(state) };
  }
  if (/[0-9]/.test(char)) {
    return { type: 'literal', value: readNumber(state) };
  }
  if (/[A-Za-z_$]/.test(char)) {
    return readIdentifierOrPath(state);
  }
  throw new ExpressionError(`malformed expression: unexpected "${char}"`);
}

function readString(state) {
  const quote = state.text[state.index];
  state.index += 1;
  let value = '';
  while (state.index < state.text.length) {
    const char = state.text[state.index];
    if (char === '\\') {
      state.index += 1;
      if (state.index >= state.text.length) break;
      const escaped = state.text[state.index];
      value += Object.prototype.hasOwnProperty.call(STRING_ESCAPES, escaped)
        ? STRING_ESCAPES[escaped]
        : escaped;
      state.index += 1;
      continue;
    }
    if (char === quote) {
      state.index += 1;
      return value;
    }
    value += char;
    state.index += 1;
  }
  throw new ExpressionError('malformed expression: unterminated string literal');
}

function readNumber(state) {
  const match = /^\d+(?:\.\d+)?/.exec(state.text.slice(state.index));
  state.index += match[0].length;
  return Number(match[0]);
}

function readIdentifier(state) {
  const match = /^[A-Za-z_$][A-Za-z0-9_$]*/.exec(state.text.slice(state.index));
  if (!match) {
    throw new ExpressionError('malformed expression: expected an identifier');
  }
  state.index += match[0].length;
  return match[0];
}

function readIdentifierOrPath(state) {
  const first = readIdentifier(state);
  const segments = [first];
  for (;;) {
    const char = state.text[state.index];
    if (char === '.') {
      state.index += 1;
      segments.push(readIdentifier(state));
      continue;
    }
    if (char === '[') {
      const close = state.text.indexOf(']', state.index);
      if (close === -1) {
        throw new ExpressionError('malformed expression: unterminated "["');
      }
      const inner = state.text.slice(state.index + 1, close).trim();
      if (!/^\d+$/.test(inner)) {
        throw new ExpressionError(`only numeric indexes are supported in "${state.text}"`);
      }
      segments.push(Number(inner));
      state.index = close + 1;
      continue;
    }
    break;
  }
  if (segments.length === 1) {
    if (first === 'true') return { type: 'literal', value: true };
    if (first === 'false') return { type: 'literal', value: false };
    if (first === 'null') return { type: 'literal', value: null };
  }
  return { type: 'path', segments, text: formatPath(segments) };
}

function formatPath(segments) {
  let text = String(segments[0]);
  for (let index = 1; index < segments.length; index += 1) {
    const segment = segments[index];
    text += typeof segment === 'number' ? `[${segment}]` : `.${segment}`;
  }
  return text;
}

function evaluateNode(node, contexts) {
  switch (node.type) {
    case 'literal':
      return node.value;
    case 'path':
      return resolveSegments(node.segments, contexts, node.text);
    case 'not':
      return !truthy(evaluateNode(node.operand, contexts));
    case 'and': {
      if (!truthy(evaluateNode(node.left, contexts))) return false;
      return truthy(evaluateNode(node.right, contexts));
    }
    case 'or': {
      if (truthy(evaluateNode(node.left, contexts))) return true;
      return truthy(evaluateNode(node.right, contexts));
    }
    case 'compare':
      return compareValues(
        node.op,
        evaluateNode(node.left, contexts),
        evaluateNode(node.right, contexts),
      );
    default:
      throw new ExpressionError('malformed expression');
  }
}

function compareValues(operator, left, right) {
  if (operator === '==') return left === right;
  if (operator === '!=') return left !== right;
  const bothNumbers = typeof left === 'number' && typeof right === 'number';
  const bothStrings = typeof left === 'string' && typeof right === 'string';
  if (!bothNumbers && !bothStrings) {
    throw new ExpressionError(
      `cannot order ${describeType(left)} against ${describeType(right)}`,
    );
  }
  switch (operator) {
    case '<': return left < right;
    case '<=': return left <= right;
    case '>': return left > right;
    case '>=': return left >= right;
    default: throw new ExpressionError(`unknown operator "${operator}"`);
  }
}

function describeType(value) {
  if (value === null) return 'null';
  if (Array.isArray(value)) return 'array';
  return typeof value;
}
