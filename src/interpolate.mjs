import {
  ExpressionError,
  assertNoSecretReferences,
  evaluate,
  toText,
  truthy,
  unwrapExpression,
} from './expr.mjs';
import { ConfigError } from './config.mjs';
import { SecretError } from './secrets.mjs';

const WRAPPER = /\$\{\{([\s\S]*?)\}\}/g;
const SECRET_PATH = /^secrets\s*(?:\.\s*([A-Za-z_][A-Za-z0-9_-]*)|\[\s*(['"])(.*?)\2\s*\])$/;

export function interpolateDataValue(value, contexts, options = {}) {
  const field = options.field ?? 'value';
  assertNoSecretReferences(value, options.secrets ?? null, field);
  return replaceWrappers(String(value), (source) => {
    const result = evaluate(source, contexts, { secrets: options.secrets ?? null, field });
    return toText(result);
  }, field);
}

export function renderScript(script, contexts, options = {}) {
  const field = options.field ?? 'run';
  const source = String(script);
  assertNoSecretReferences(source, options.secrets ?? null, field);
  return replaceWrappers(source, (expression) => {
    const result = evaluate(expression, contexts, { secrets: options.secrets ?? null, field });
    if (result !== null && typeof result === 'object') {
      throw new ExpressionError(`"${expression.trim()}" cannot be inserted into a script`);
    }
    return String(result);
  }, field);
}

export function interpolateEnvMapping(mapping, contexts, options = {}) {
  const result = {};
  for (const [key, value] of Object.entries(mapping)) {
    result[key] = interpolateEnvValue(value, contexts, options);
  }
  return result;
}

export function interpolateEnvValue(value, contexts, options = {}) {
  const field = options.field ?? 'env';
  const secrets = options.secrets ?? null;
  return replaceWrappers(String(value), (expression) => {
    const trimmed = unwrapExpression(expression);
    const secretName = secretNameOf(trimmed);
    if (secretName !== null) {
      if (!secrets) throw new SecretError(`unknown secret "${secretName.toUpperCase()}"`);
      return secrets.get(secretName);
    }
    const result = evaluate(trimmed, contexts, { secrets, field });
    return toText(result);
  }, field);
}

export function evaluateCondition(condition, contexts, options = {}) {
  const field = options.field ?? 'if';
  const text = String(condition);
  assertNoSecretReferences(text, options.secrets ?? null, field);

  const wrapped = text.trim().match(/^\$\{\{([\s\S]*)\}\}$/);
  const source = wrapped ? wrapped[1] : text;

  if (/^\s*$/.test(source)) {
    throw new ExpressionError('empty condition');
  }
  return truthy(evaluate(source, contexts, { secrets: options.secrets ?? null, field }));
}

export function findExpressions(text) {
  const found = [];
  const pattern = new RegExp(WRAPPER.source, 'g');
  let match;
  while ((match = pattern.exec(String(text))) !== null) {
    found.push({ source: match[1], index: match.index, raw: match[0] });
  }
  return found;
}

function replaceWrappers(text, render, field) {
  const pattern = new RegExp(WRAPPER.source, 'g');
  let output = '';
  let lastIndex = 0;
  let match;

  while ((match = pattern.exec(text)) !== null) {
    output += text.slice(lastIndex, match.index);
    output += render(match[1]);
    lastIndex = match.index + match[0].length;
  }

  const rest = text.slice(lastIndex);
  if (rest.includes('${{')) {
    throw new ConfigError(`unterminated expression in "${field}"`);
  }
  return output + rest;
}

function secretNameOf(trimmed) {
  const match = SECRET_PATH.exec(trimmed);
  if (!match) return null;
  return match[1] ?? match[3] ?? '';
}

export { ExpressionError, SecretError, ConfigError };
