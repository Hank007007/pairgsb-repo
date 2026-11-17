export function safeName(name) {
  if (typeof name !== 'string') return false;
  if (name.length === 0) return false;
  if (name.includes('/')) return false;
  if (name.includes('..')) return false;
  return true;
}
