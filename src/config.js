const DEFAULT_CONFIG = Object.freeze({
  url: 'ldap://directory.internal:389',
  peopleSuffix: 'ou=people,dc=corp,dc=example',
  serviceSuffix: 'ou=service,dc=corp,dc=example',
  suffixes: Object.freeze([
    'ou=people,dc=corp,dc=example',
    'ou=service,dc=corp,dc=example',
  ]),
  search: Object.freeze({
    base: 'ou=people,dc=corp,dc=example',
    scope: 'sub',
    pageSize: 2,
    attributes: Object.freeze(['uid', 'cn', 'mail', 'ou', 'title']),
  }),
  advanced: Object.freeze({
    base: 'ou=people,dc=corp,dc=example',
    scope: 'sub',
    maxFilterLength: 1024,
  }),
});

function freezeList(value) {
  return Object.freeze([...value]);
}

function mergeSection(base, override) {
  if (!override) return base;
  return Object.freeze({ ...base, ...override });
}

export function createConfig(overrides = {}) {
  const suffixes = overrides.suffixes ? freezeList(overrides.suffixes) : DEFAULT_CONFIG.suffixes;
  return Object.freeze({
    ...DEFAULT_CONFIG,
    ...overrides,
    suffixes,
    search: mergeSection(DEFAULT_CONFIG.search, overrides.search),
    advanced: mergeSection(DEFAULT_CONFIG.advanced, overrides.advanced),
  });
}

export const config = createConfig();
