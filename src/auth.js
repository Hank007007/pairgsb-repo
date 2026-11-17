import { config as defaultConfig } from './config.js';
import { buildUserDN, escapeDN } from './dn.js';
import { buildUserFilter } from './filter.js';
import { ErrorCodes, LdapError } from './ldap/errors.js';
import { search } from './search.js';

export function userBindDN(username, config = defaultConfig) {
  return buildUserDN(username, config.peopleSuffix);
}

export async function findUser(client, username, config = defaultConfig) {
  const filter = buildUserFilter(username);
  const result = await search(
    client,
    {
      base: config.peopleSuffix,
      scope: 'sub',
      filter,
      attributes: config.search.attributes,
    },
    config,
  );
  return result.entries;
}

export async function authenticate(client, credentials = {}, config = defaultConfig) {
  const username = credentials.username;
  const password = credentials.password;
  if (typeof username !== 'string' || username === '') {
    throw new LdapError('username is required', ErrorCodes.INVALID_CREDENTIALS);
  }
  if (typeof password !== 'string' || password === '') {
    throw new LdapError('password is required', ErrorCodes.INVALID_CREDENTIALS);
  }

  const dn = userBindDN(username, config);
  let entries;
  try {
    entries = await findUser(client, username, config);
  } catch (error) {
    throw new LdapError('unable to look up account', ErrorCodes.PROTOCOL_ERROR, {
      cause: error?.code,
    });
  }

  if (entries.length !== 1) {
    throw new LdapError('account not found', ErrorCodes.INVALID_CREDENTIALS, {
      matches: entries.length,
    });
  }

  try {
    await client.bind(dn, password);
  } catch (error) {
    throw new LdapError('invalid credentials', ErrorCodes.INVALID_CREDENTIALS, {
      cause: error?.code,
    });
  }

  return {
    dn,
    entry: entries[0],
    displayName: entries[0]?.attributes?.cn?.[0] ?? entries[0]?.attributes?.uid?.[0] ?? username,
  };
}

export function describeBinding(username, config = defaultConfig) {
  return {
    dn: buildUserDN(username, config.peopleSuffix),
    escapedValue: escapeDN(username),
  };
}
