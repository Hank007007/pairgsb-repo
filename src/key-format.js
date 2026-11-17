const KEY_PATTERN = /^kl_([a-z0-9-]+)_([0-9a-f]{2,64})_([0-9a-f]{16,128})$/;

export const ENVIRONMENTS = Object.freeze(['prod', 'staging', 'dev']);

export function isKnownEnvironment(value) {
  return ENVIRONMENTS.includes(value);
}

export function parseKey(value) {
  if (typeof value !== 'string') return null;
  const match = KEY_PATTERN.exec(value);
  if (!match) return null;
  return {
    env: match[1],
    id8: match[2],
    secret: match[3],
  };
}

export function formatKey({ env, id8, secret }) {
  return `kl_${env}_${id8}_${secret}`;
}
