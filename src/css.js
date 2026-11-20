// CSS sanitization for `style=` attributes and `<style>` element text.

import { ALLOWED_CSS_PROPERTIES } from './policy.js';
import { isSafeUrl } from './url.js';

function splitDeclarations(style) {
  return style.split(';');
}

function splitProperty(declaration) {
  const colon = declaration.indexOf(':');
  if (colon === -1) return null;
  return {
    property: declaration.slice(0, colon).trim().toLowerCase(),
    value: declaration.slice(colon + 1).trim(),
  };
}

function checkUrls(value) {
  const urlPattern = /url\(([^)]*)\)/gi;
  let match;
  while ((match = urlPattern.exec(value)) !== null) {
    if (!isSafeUrl(match[1])) return false;
  }
  return true;
}

function checkValue(value) {
  return checkUrls(value);
}

export function sanitizeStyle(style) {
  if (typeof style !== 'string') return '';

  const kept = [];
  for (const declaration of splitDeclarations(style)) {
    if (declaration.trim() === '') continue;
    const parsed = splitProperty(declaration);
    if (!parsed) continue;

    const { property, value } = parsed;
    if (!ALLOWED_CSS_PROPERTIES.has(property)) continue;
    if (value === '') continue;
    if (!checkValue(value)) continue;

    kept.push(`${property}: ${value}`);
  }

  return kept.join('; ');
}

// Sanitizes the text content of a <style> element. Rule blocks are split on
// `{`/`}`, and every declaration inside a kept block is passed through
// sanitizeStyle. Content outside a block is carried through as written.
export function sanitizeStyleSheet(css) {
  if (typeof css !== 'string') return '';

  let text = css;
  const cdata = text.match(/^\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*$/);
  if (cdata) text = cdata[1];

  const blocks = [];
  let i = 0;
  while (i < text.length) {
    const open = text.indexOf('{', i);
    const close = open === -1 ? -1 : text.indexOf('}', open + 1);

    if (open === -1 || close === -1) {
      const rest = text.slice(i).trim();
      if (rest !== '') blocks.push(rest);
      break;
    }

    const selector = text.slice(i, open).trim();
    const body = text.slice(open + 1, close);
    i = close + 1;

    if (selector.startsWith('@')) {
      blocks.push(`${selector} { ${body.trim()} }`);
      continue;
    }

    const declarations = sanitizeStyle(body);
    if (declarations !== '') {
      blocks.push(`${selector} { ${declarations} }`);
    }
  }

  return blocks.join('\n');
}
