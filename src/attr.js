// Attribute allowlisting and per-type validation.

import {
  ALLOWED_ATTRIBUTES,
  DENIED_ATTRIBUTES,
  ENUM_ATTRIBUTES,
  NUMERIC_ATTRIBUTES,
  PAINT_ATTRIBUTES,
  URL_REFERENCE_ATTRIBUTES,
} from './policy.js';
import { isSafeUrl } from './url.js';
import { sanitizeStyle } from './css.js';
import { decodeEntities } from './entities.js';

const LENGTH = /^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?(?:px|em|ex|pt|pc|cm|mm|in|%)?$/;
const VIEWBOX = /^[+-]?(?:\d+\.?\d*|\.\d+)(?:[,\s]+[+-]?(?:\d+\.?\d*|\.\d+)){3}$/;
const URL_WRAPPER = /^url\(\s*(['"]?)(.*?)\1\s*\)$/i;
const LOCAL_FRAGMENT = /^#[\w.:-]+$/;

const ENUM_VALUES = {
  'fill-rule': new Set(['nonzero', 'evenodd', 'inherit']),
  'clip-rule': new Set(['nonzero', 'evenodd', 'inherit']),
  'stroke-linecap': new Set(['butt', 'round', 'square', 'inherit']),
  'stroke-linejoin': new Set(['miter', 'round', 'bevel', 'inherit']),
  'font-weight': new Set(['normal', 'bold', 'bolder', 'lighter', 'inherit']),
  'font-style': new Set(['normal', 'italic', 'oblique', 'inherit']),
  'text-anchor': new Set(['start', 'middle', 'end', 'inherit']),
  'dominant-baseline': new Set([
    'auto',
    'alphabetic',
    'middle',
    'central',
    'hanging',
    'mathematical',
    'ideographic',
    'text-top',
    'text-bottom',
  ]),
  preserveaspectratio: new Set(['none', 'xminymin', 'xmidymin', 'xmaxymin', 'xminymid', 'xmidymid', 'xmaxymid', 'xminymax', 'xmidymax', 'xmaxymax']),
  display: new Set(['inline', 'block', 'none', 'inherit']),
  visibility: new Set(['visible', 'hidden', 'collapse', 'inherit']),
};

function normalizeName(name) {
  return name.toLowerCase();
}

function isSafePaint(value) {
  const trimmed = value.trim();
  if (trimmed === 'none' || trimmed === 'currentColor' || trimmed === 'inherit') return true;
  if (/^#[0-9a-f]{3,8}$/i.test(trimmed)) return true;
  if (/^rgba?\(/i.test(trimmed) || /^hsla?\(/i.test(trimmed)) return true;
  if (/^[a-z]+$/i.test(trimmed)) return true;
  const wrapped = trimmed.match(URL_WRAPPER);
  if (wrapped) return isSafeUrl(wrapped[2]);
  return false;
}

function isSafeLocalReference(value) {
  const trimmed = value.trim();
  if (trimmed === 'none') return true;
  const wrapped = trimmed.match(URL_WRAPPER);
  if (wrapped) return LOCAL_FRAGMENT.test(wrapped[2]);
  return false;
}

function isTransform(value) {
  return /^(?:\s*(?:matrix|translate|scale|rotate|skewX|skewY)\s*\([^()]*\)\s*)*$/i.test(value);
}

function isPointList(value) {
  return /^\s*[+-]?(?:\d+\.?\d*|\.\d+)(?:[,\s]+[+-]?(?:\d+\.?\d*|\.\d+))*\s*$/.test(value);
}

// Returns the filtered attribute, or null when the attribute must be dropped.
// `elementName` is the normalized name of the owning element.
export function filterAttr(name, value, elementName) {
  const normalized = normalizeName(name);
  const decoded = value === null ? null : decodeEntities(value);

  if (DENIED_ATTRIBUTES.has(name)) {
    return null;
  }

  if (ALLOWED_ATTRIBUTES.has(normalized)) {
    const result = checkType(normalized, decoded);
    return result === null ? null : { name, value: result.value };
  }

  // Attributes we do not recognise do not carry executable content of their own.
  return { name, value: decoded };
}

function checkType(name, value) {
  if (value === null) {
    // Boolean-ish allowlisted attributes are not used by SVG; drop them.
    return null;
  }

  if (name === 'style') {
    const css = sanitizeStyle(value);
    return css === '' ? null : { name, value: css };
  }

  if (name === 'xmlns') {
    return value === 'http://www.w3.org/2000/svg' ? { name, value } : null;
  }

  if (NUMERIC_ATTRIBUTES.has(name)) {
    return LENGTH.test(value.trim()) ? { name, value } : null;
  }

  if (name === 'viewbox') {
    return VIEWBOX.test(value.trim()) ? { name, value } : null;
  }

  if (name === 'transform') {
    return isTransform(value) ? { name, value } : null;
  }

  if (name === 'points') {
    return isPointList(value) ? { name, value } : null;
  }

  if (name === 'd' || name === 'path') {
    if (/[<]/.test(value)) return null;
    return { name, value };
  }
  if (PAINT_ATTRIBUTES.has(name)) {
    return isSafePaint(value) ? { name, value } : null;
  }

  if (URL_REFERENCE_ATTRIBUTES.has(name)) {
    if (name === 'href' || name === 'xlink:href') {
      return isSafeUrl(value) ? { name, value } : null;
    }
    return isSafeLocalReference(value) ? { name, value } : null;
  }

  if (ENUM_ATTRIBUTES.has(name)) {
    const allowed = ENUM_VALUES[name];
    if (!allowed) return { name, value };
    return allowed.has(value.trim().toLowerCase()) ? { name, value } : null;
  }

  if (name === 'id') {
    return /^[A-Za-z_][\w.:-]*$/.test(value) ? { name, value } : null;
  }

  if (name === 'class') {
    return /^[\w\s-]+$/.test(value) ? { name, value } : null;
  }

  if (name === 'title') {
    return /[<>]/.test(value) ? null : { name, value };
  }

  return { name, value };
}
