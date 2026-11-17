import { createDirectory, matchesFilter } from './directory.js';
import { dnEquals, isDescendantDN } from './dn-parser.js';
import { ErrorCodes, LdapError } from './errors.js';
import { parseFilter } from './filter-parser.js';
import { SEED_ENTRIES, inScope } from './seed.js';

const VALID_SCOPES = new Set(['base', 'one', 'sub']);

function project(entry, attributes) {
  if (!attributes || attributes.length === 0) {
    return { dn: entry.dn, attributes: structuredClone(entry.attributes) };
  }
  const wanted = attributes.map((name) => String(name).toLowerCase());
  const selected = {};
  for (const name of wanted) {
    if (name === 'dn') continue;
    const values = entry.attributes[name];
    if (values) selected[name] = [...values];
  }
  return { dn: entry.dn, attributes: selected };
}

function namingContextMatches(base, namingContexts) {
  return namingContexts.some((context) => isDescendantDN(base, context));
}

/**
 * In-process stand-in for an LDAP client. It accepts the same call shape as
 * ldapjs (`client.search(base, options)`, `client.bind(dn, password)`) and
 * evaluates filters against an in-memory directory. Every outbound call is
 * recorded on `operations` so callers can observe what would have hit the wire.
 */
export function createMockClient(options = {}) {
  const entries = createDirectory(options.entries ?? SEED_ENTRIES);
  const namingContexts = options.namingContexts ?? ['dc=corp,dc=example'];
  const operations = [];
  const boundDns = [];

  return {
    operations,
    boundDns,

    async search(base, searchOptions = {}) {
      operations.push({ type: 'search', base, options: { ...searchOptions } });
      if (typeof base !== 'string' || base.trim() === '') {
        throw new LdapError('empty search base', ErrorCodes.NO_SUCH_OBJECT);
      }
      if (!namingContextMatches(base, namingContexts)) {
        throw new LdapError(`no such naming context: ${base}`, ErrorCodes.NO_SUCH_OBJECT, { base });
      }
      const scope = searchOptions.scope ?? 'sub';
      if (!VALID_SCOPES.has(scope)) {
        throw new LdapError(`invalid scope: ${scope}`, ErrorCodes.PROTOCOL_ERROR, { scope });
      }
      let node;
      try {
        node = parseFilter(searchOptions.filter ?? '(objectClass=*)');
      } catch (error) {
        throw new LdapError(`invalid filter: ${error.message}`, ErrorCodes.PROTOCOL_ERROR);
      }

      const matched = entries.filter((entry) => inScope(entry.dn, base, scope) && matchesFilter(node, entry));
      const pageSize = searchOptions.paged?.pageSize ?? searchOptions.pageSize ?? 0;
      const offset = Number.parseInt(searchOptions.pageCookie ?? '0', 10) || 0;
      const limit = searchOptions.sizeLimit ?? 0;
      const capped = limit > 0 ? matched.slice(0, limit) : matched;
      const pageEnd = pageSize > 0 ? Math.min(capped.length, offset + pageSize) : capped.length;
      const page = capped.slice(offset, pageEnd);
      const hasMore = pageSize > 0 && pageEnd < capped.length;

      return {
        entries: page.map((entry) => project(entry, searchOptions.attributes)),
        pageCookie: hasMore ? String(pageEnd) : undefined,
      };
    },

    async bind(dn, password) {
      operations.push({ type: 'bind', dn });
      const entry = entries.find((candidate) => dnEquals(candidate.dn, dn));
      if (!entry || entry.password !== password) {
        throw new LdapError('invalid credentials', ErrorCodes.INVALID_CREDENTIALS, { dn });
      }
      boundDns.push(entry.dn);
      return { dn: entry.dn };
    },

    async unbind() {
      operations.push({ type: 'unbind' });
    },
  };
}
