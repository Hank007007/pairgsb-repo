#!/usr/bin/env node
import { authenticate, findUser, describeBinding } from '../src/auth.js';
import { advancedSearch } from '../src/advanced.js';
import { config } from '../src/config.js';
import { createMockClient } from '../src/ldap/mock-client.js';
import { searchOrganization, searchServiceAccounts } from '../src/search.js';

const USAGE = `ldapfence — directory access layer demo

Usage:
  node bin/ldapfence.js login <uid> <password>
  node bin/ldapfence.js lookup <uid>
  node bin/ldapfence.js sync <ou> [pageSize]
  node bin/ldapfence.js services
  node bin/ldapfence.js advanced <filter> [base] [scope]
  node bin/ldapfence.js dn <uid>
`;

function printEntries(entries) {
  if (entries.length === 0) {
    console.log('(no entries)');
    return;
  }
  for (const entry of entries) {
    const uid = entry.attributes?.uid?.[0] ?? '-';
    const cn = entry.attributes?.cn?.[0] ?? '-';
    console.log(`${entry.dn}  uid=${uid} cn=${cn}`);
  }
}

async function main(argv) {
  const [command, ...rest] = argv;
  const client = createMockClient();

  switch (command) {
    case 'login': {
      const session = await authenticate(client, { username: rest[0], password: rest[1] }, config);
      console.log(`bound as ${session.dn} (${session.displayName})`);
      return 0;
    }
    case 'lookup': {
      printEntries(await findUser(client, rest[0], config));
      return 0;
    }
    case 'sync': {
      const pageSize = rest[1] ? Number.parseInt(rest[1], 10) : undefined;
      const result = await searchOrganization(client, rest[0], { pageSize }, config);
      printEntries(result.entries);
      console.log(`(${result.entries.length} entries, ${client.operations.length} operations)`);
      return 0;
    }
    case 'services': {
      const result = await searchServiceAccounts(client, {}, config);
      printEntries(result.entries);
      return 0;
    }
    case 'advanced': {
      const result = await advancedSearch(
        client,
        { filter: rest[0], base: rest[1], scope: rest[2] },
        config,
      );
      printEntries(result.entries);
      console.log(`(${result.entries.length} entries)`);
      return 0;
    }
    case 'dn': {
      console.log(JSON.stringify(describeBinding(rest[0], config), null, 2));
      return 0;
    }
    default:
      process.stdout.write(USAGE);
      return command ? 2 : 0;
  }
}

main(process.argv.slice(2)).then(
  (code) => {
    process.exitCode = code;
  },
  (error) => {
    console.error(`${error.code ?? 'ERROR'}: ${error.message}`);
    process.exitCode = 1;
  },
);
