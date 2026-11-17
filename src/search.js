import { config as defaultConfig } from './config.js';
import { buildServiceDN } from './dn.js';
import { buildEqualityFilter } from './filter.js';
import { isDescendantDN, parseDN } from './ldap/dn-parser.js';
import { ErrorCodes, LdapError, protocolError } from './ldap/errors.js';

const VALID_SCOPES = new Set(['base', 'one', 'sub']);

function resolveSettings(options, config) {
  return options.settings ?? config.search;
}

export function validateSearchBase(base, config = defaultConfig) {
  if (typeof base !== 'string') {
    throw new LdapError('search base must be a DN string', ErrorCodes.INVALID_DN_SYNTAX, {
      baseType: typeof base,
    });
  }
  parseDN(base);
  const suffixes = Array.isArray(config.suffixes) ? config.suffixes : [];
  const allowed = suffixes.some((suffix) => {
    try {
      return isDescendantDN(base, suffix);
    } catch {
      return false;
    }
  });
  if (!allowed) {
    throw new LdapError('search base is not permitted', ErrorCodes.NO_SUCH_OBJECT);
  }
  return base;
}

export function validateSearchScope(scope) {
  if (!VALID_SCOPES.has(scope)) {
    throw protocolError('invalid search scope', { scope: String(scope) });
  }
  return scope;
}

export function validateSearchCount(value, name) {
  if (!Number.isInteger(value) || value < 0) {
    throw protocolError(`${name} must be a non-negative integer`, { [name]: value });
  }
  return value;
}

function normalizeCount(value, fallback, name) {
  const resolved = value ?? fallback ?? 0;
  validateSearchCount(resolved, name);
  return resolved;
}

export function normalizeOptions(options = {}, config = defaultConfig) {
  const settings = resolveSettings(options, config);
  const base = options.base ?? settings.base;
  const scope = options.scope ?? settings.scope ?? 'sub';
  const attributes = options.attributes ?? settings.attributes;
  const pageSize = normalizeCount(options.pageSize, settings.pageSize, 'pageSize');
  const sizeLimit = normalizeCount(options.sizeLimit, settings.sizeLimit, 'sizeLimit');
  validateSearchBase(base, config);
  validateSearchScope(scope);
  return {
    base,
    scope,
    filter: options.filter ?? '(objectClass=*)',
    attributes: attributes ? [...attributes] : undefined,
    sizeLimit,
    paged: pageSize > 0 ? { pageSize, pagePause: false } : undefined,
  };
}

function toSearchResult(result) {
  if (!result) return { entries: [], pageCookie: undefined };
  if (Array.isArray(result)) return { entries: result, pageCookie: undefined };
  return {
    entries: result.entries ?? [],
    pageCookie: result.pageCookie ?? result.controls?.pageCookie,
  };
}

export async function search(client, options = {}, config = defaultConfig) {
  const normalized = normalizeOptions(options, config);
  const entries = [];
  let pageCookie;
  do {
    const pageOptions = {
      scope: normalized.scope,
      filter: normalized.filter,
      attributes: normalized.attributes,
      sizeLimit: normalized.sizeLimit,
    };
    if (normalized.paged) {
      pageOptions.paged = normalized.paged;
      if (pageCookie !== undefined) pageOptions.pageCookie = pageCookie;
    }
    const page = toSearchResult(await client.search(normalized.base, pageOptions));
    entries.push(...page.entries);
    pageCookie = normalized.paged ? page.pageCookie : undefined;
    if (!normalized.paged) break;
  } while (pageCookie);

  return { entries, base: normalized.base, scope: normalized.scope };
}

export async function searchOrganization(client, ou, options = {}, config = defaultConfig) {
  return search(
    client,
    {
      ...options,
      base: options.base ?? config.peopleSuffix,
      scope: options.scope ?? 'sub',
      filter: options.filter ?? buildEqualityFilter('ou', ou),
    },
    config,
  );
}

export async function searchServiceAccounts(client, options = {}, config = defaultConfig) {
  return search(
    client,
    {
      ...options,
      base: options.base ?? config.serviceSuffix,
      scope: options.scope ?? 'one',
      filter: options.filter ?? '(objectClass=serviceAccount)',
    },
    config,
  );
}

export function serviceDNFor(accountName, config = defaultConfig) {
  const dn = buildServiceDN(accountName, config.serviceSuffix);
  if (dn.trim() === '') {
    throw protocolError('service account name must not be empty');
  }
  return dn;
}
