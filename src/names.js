const NAME_PATTERN = /^[a-z][a-z0-9_]{0,63}$/;

export function safeName(name) {
  return typeof name === 'string' && NAME_PATTERN.test(name);
}
