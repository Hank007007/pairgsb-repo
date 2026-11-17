import { config as defaultConfig } from './config.js';
import { protocolError } from './ldap/errors.js';
import { search } from './search.js';

export function parseAdvanced(input = {}, config = defaultConfig) {
  const settings = config.advanced;
  const filter = String(input.filter ?? '').trim();
  if (filter === '') {
    throw protocolError('advanced search requires a filter');
  }
  if (filter.length > settings.maxFilterLength) {
    throw protocolError('advanced filter exceeds the maximum length', {
      length: filter.length,
      maxFilterLength: settings.maxFilterLength,
    });
  }
  return {
    base: input.base ?? settings.base,
    scope: input.scope ?? settings.scope ?? 'sub',
    filter,
    attributes: input.attributes ? [...input.attributes] : undefined,
    pageSize: input.pageSize ?? settings.pageSize,
    sizeLimit: input.sizeLimit ?? settings.sizeLimit,
  };
}

export async function advancedSearch(client, input = {}, config = defaultConfig) {
  const parsed = parseAdvanced(input, config);
  return search(
    client,
    {
      base: parsed.base,
      scope: parsed.scope,
      filter: parsed.filter,
      attributes: parsed.attributes,
      pageSize: parsed.pageSize,
      sizeLimit: parsed.sizeLimit,
    },
    config,
  );
}
