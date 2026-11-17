import { config as defaultConfig } from './config.js';
import { buildServiceDN } from './dn.js';
import { buildEqualityFilter } from './filter.js';
import { protocolError } from './ldap/errors.js';

function resolveSettings(options, config) {
  return options.settings ?? config.search;
}

export function normalizeOptions(options = {}, config = defaultConfig) {
  const settings = resolveSettings(options, config);
  const base = options.base ?? settings.base;
  const scope = options.scope ?? settings.scope ?? 'sub';
  const attributes = options.attributes ?? settings.attributes;
  const pageSize = options.pageSize ?? settings.pageSize ?? 0;
  const sizeLimit = options.sizeLimit ?? settings.sizeLimit ?? 0;
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
