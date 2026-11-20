// Main sanitization walk. Consumes the flat token stream from tokenize.js and
// emits an allowlisted SVG document.

import { tokenize } from './tokenize.js';
import { filterAttr } from './attr.js';
import { ALLOWED_ELEMENTS } from './policy.js';
import { sanitizeStyleSheet } from './css.js';
import { decodeEntities } from './entities.js';

function escapeAttr(value) {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

function escapeText(value) {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function normalizeElementName(name) {
  const colon = name.indexOf(':');
  const local = colon === -1 ? name : name.slice(colon + 1);
  return local.toLowerCase();
}

function renderStartTag(name, attrs, selfClosing) {
  const parts = [name];
  for (const attr of attrs) {
    parts.push(`${attr.name}="${escapeAttr(attr.value)}"`);
  }
  return `<${parts.join(' ')}${selfClosing ? '/' : ''}>`;
}

// Walks the token stream.
//   skipDepth  — nesting inside an element whose subtree has been dropped.
//   rawDepth   — nesting inside foreign content that is carried through.
export function walk(tokens) {
  const out = [];
  const stack = [];
  let skipDepth = 0;
  let rawDepth = 0;

  for (const token of tokens) {
    if (rawDepth > 0) {
      out.push(token.raw);
      if (token.type === 'startTag' && !token.selfClosing) rawDepth += 1;
      else if (token.type === 'endTag') rawDepth -= 1;
      continue;
    }

    if (skipDepth > 0) {
      if (token.type === 'startTag' && !token.selfClosing) skipDepth += 1;
      else if (token.type === 'endTag') skipDepth -= 1;
      continue;
    }

    switch (token.type) {
      case 'doctype':
      case 'bang':
        out.push(token.raw);
        break;

      case 'pi':
        out.push(token.raw);
        break;

      case 'comment':
        out.push(token.raw);
        break;

      case 'cdata': {
        const inner = token.raw.slice('<![CDATA['.length, token.raw.endsWith(']]>') ? -3 : undefined);
        const open = stack[stack.length - 1];
        if (open === 'style') {
          out.push(escapeText(sanitizeStyleSheet(inner)));
        } else {
          out.push(escapeText(inner));
        }
        break;
      }

      case 'text': {
        const text = decodeEntities(token.raw);
        const open = stack[stack.length - 1];
        if (open === 'style') {
          out.push(escapeText(sanitizeStyleSheet(text)));
        } else {
          out.push(escapeText(text));
        }
        break;
      }

      case 'startTag': {
        const name = normalizeElementName(token.name);

        if (name === 'foreignobject') {
          // Foreign content is markup from another namespace; carry the block
          // through unchanged.
          out.push(token.raw);
          if (!token.selfClosing) rawDepth = 1;
          break;
        }

        if (!ALLOWED_ELEMENTS.has(name)) {
          if (token.selfClosing) {
            out.push(token.raw);
          } else {
            skipDepth = 1;
          }
          break;
        }

        const attrs = [];
        for (const attr of token.attrs) {
          const filtered = filterAttr(attr.name, attr.value, name);
          if (filtered !== null) attrs.push(filtered);
        }

        out.push(renderStartTag(token.name, attrs, token.selfClosing));

        if (!token.selfClosing) {
          stack.push(name);
        }
        break;
      }

      case 'endTag':
        if (stack.length > 0) stack.pop();
        out.push(`</${token.name}>`);
        break;

      default:
        break;
    }
  }

  return out.join('');
}

export function sanitize(input) {
  if (typeof input !== 'string') {
    throw new TypeError('sanitize() expects an SVG string');
  }
  return walk(tokenize(input));
}
