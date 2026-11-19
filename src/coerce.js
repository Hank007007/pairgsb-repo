/**
 * Coerce text coming from an environment variable or `--set` entry to the
 * type of the value already present at the target path.
 *
 * @param {*} raw
 * @param {*} existing
 * @param {boolean} found
 * @returns {{ ok: true, value: *, append?: boolean } | { ok: false }}
 */
export function coerceText(raw, existing, found) {
  const text = String(raw);

  if (!found) return { ok: true, value: text };

  if (typeof existing === 'boolean') {
    const lowered = text.trim().toLowerCase();
    if (lowered === 'true') return { ok: true, value: true };
    if (lowered === 'false') return { ok: true, value: false };
    return { ok: false };
  }

  if (typeof existing === 'number') {
    if (text.trim() === '') return { ok: false };
    const parsed = Number(text);
    if (!Number.isFinite(parsed)) return { ok: false };
    return { ok: true, value: parsed };
  }

  if (Array.isArray(existing)) {
    const first = existing.length > 0 ? existing[0] : undefined;
    if (typeof first === 'boolean') {
      const lowered = text.trim().toLowerCase();
      if (lowered === 'true') return { ok: true, value: true, append: true };
      if (lowered === 'false') return { ok: true, value: false, append: true };
      return { ok: false };
    }
    if (typeof first === 'number') {
      if (text.trim() === '') return { ok: false };
      const parsed = Number(text);
      if (!Number.isFinite(parsed)) return { ok: false };
      return { ok: true, value: parsed, append: true };
    }
    return { ok: true, value: text, append: true };
  }

  return { ok: true, value: text };
}
