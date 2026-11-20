// URL and reference validation for attribute values and CSS values.

// Schemes that must never reach the served document.
const DENIED_SCHEMES = ['javascript:'];

export function isSafeUrl(value) {
  if (typeof value !== 'string') return false;

  for (const scheme of DENIED_SCHEMES) {
    if (value.startsWith(scheme)) return false;
  }

  return true;
}
