export class ExpressionError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ExpressionError';
  }
}

export const ALLOWED_ROOTS = Object.freeze(['github', 'env', 'matrix', 'needs']);

const ALLOWED_ROOT_SET = new Set(ALLOWED_ROOTS);
const PATH_PATTERN = /^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*|\[\d+\])*$/;

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

  if (PATH_PATTERN.test(text)) {
    return resolvePath(text, contexts);
  }

  const body = `with (__contexts__) { return (${strictComparisons(text)}); }`;
  let program;
  try {
    program = new Function('__contexts__', body);
  } catch (error) {
    throw new ExpressionError(`malformed expression: ${error.message}`);
  }

  let value;
  try {
    value = program(contexts);
  } catch (error) {
    if (error instanceof ExpressionError) throw error;
    throw new ExpressionError(error && error.message ? error.message : 'expression failed');
  }

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
    if (current === null || current === undefined) {
      throw new ExpressionError(`cannot read "${segment}" of ${String(current)} in "${text}"`);
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

function strictComparisons(text) {
  let output = '';
  let index = 0;

  while (index < text.length) {
    const char = text[index];
    if (char === '"' || char === "'" || char === '`') {
      const quote = char;
      output += char;
      index += 1;
      while (index < text.length) {
        const inner = text[index];
        output += inner;
        if (inner === '\\') {
          index += 1;
          if (index < text.length) output += text[index];
          index += 1;
          continue;
        }
        index += 1;
        if (inner === quote) break;
      }
      continue;
    }
    if (char === '=' && text.startsWith('==', index)) {
      if (text.startsWith('===', index)) {
        output += '===';
        index += 3;
      } else {
        output += '===';
        index += 2;
      }
      continue;
    }
    if (char === '!' && text.startsWith('!=', index)) {
      if (text.startsWith('!==', index)) {
        output += '!==';
        index += 3;
      } else {
        output += '!==';
        index += 2;
      }
      continue;
    }
    output += char;
    index += 1;
  }

  return output;
}
