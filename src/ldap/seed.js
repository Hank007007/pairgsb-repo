import { isDescendantDN, isDirectChildDN, parseDN } from './dn-parser.js';

export const SEED_ENTRIES = [
  {
    dn: 'uid=alice,ou=people,dc=corp,dc=example',
    password: 'alice-secret',
    attributes: {
      objectClass: ['inetOrgPerson', 'organizationalPerson'],
      uid: ['alice'],
      cn: ['Alice Zhang'],
      mail: ['alice.zhang@corp.example'],
      ou: ['engineering'],
      title: ['Staff Engineer'],
    },
  },
  {
    dn: 'uid=bob,ou=people,dc=corp,dc=example',
    password: 'bob-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['bob'],
      cn: ['Bob Li'],
      mail: ['bob.li@corp.example'],
      ou: ['engineering'],
      title: ['Engineer'],
    },
  },
  {
    dn: 'uid=carol,ou=people,dc=corp,dc=example',
    password: 'carol-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['carol'],
      cn: ['Carol Wu'],
      mail: ['carol.wu@corp.example'],
      ou: ['engineering'],
      title: ['Engineer'],
    },
  },
  {
    dn: 'uid=dave,ou=people,dc=corp,dc=example',
    password: 'dave-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['dave'],
      cn: ['Dave Chen'],
      mail: ['dave.chen@corp.example'],
      ou: ['engineering'],
      title: ['Engineering Manager'],
    },
  },
  {
    dn: 'uid=erin,ou=people,dc=corp,dc=example',
    password: 'erin-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['erin'],
      cn: ['Erin Sun'],
      mail: ['erin.sun@corp.example'],
      ou: ['sales'],
      title: ['Account Executive'],
    },
  },
  {
    dn: 'uid=frank,ou=people,dc=corp,dc=example',
    password: 'frank-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['frank'],
      cn: ['Frank Xu'],
      mail: ['frank.xu@corp.example'],
      ou: ['sales'],
      title: ['Account Executive'],
    },
  },
  {
    dn: 'uid=eve,ou=partners,ou=people,dc=corp,dc=example',
    password: 'eve-partner-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['eve'],
      cn: ['Eve Partner'],
      mail: ['eve@partner.example'],
      ou: ['partners'],
      title: ['Partner Liaison'],
    },
  },
  {
    dn: 'uid=diaz\\, ana,ou=people,dc=corp,dc=example',
    password: 'diaz-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['diaz, ana'],
      cn: ['Ana Diaz'],
      mail: ['ana.diaz@corp.example'],
      ou: ['lab'],
      title: ['Researcher'],
    },
  },
  {
    dn: 'uid=o\\=connor,ou=people,dc=corp,dc=example',
    password: 'connor-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['o=connor'],
      cn: ["Riley O'Connor"],
      mail: ['riley.oconnor@corp.example'],
      ou: ['lab'],
      title: ['Researcher'],
    },
  },
  {
    dn: 'uid=back\\\\slash,ou=people,dc=corp,dc=example',
    password: 'slash-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['back\\slash'],
      cn: ['Back Slash'],
      mail: ['back.slash@corp.example'],
      ou: ['lab'],
      title: ['Researcher'],
    },
  },
  {
    dn: 'cn=svc-sync,ou=service,dc=corp,dc=example',
    password: 'sync-secret',
    attributes: {
      objectClass: ['serviceAccount'],
      cn: ['svc-sync'],
      uid: ['svc-sync'],
      description: ['org chart synchroniser'],
    },
  },
  {
    dn: 'cn=svc-reports,ou=service,dc=corp,dc=example',
    password: 'reports-secret',
    attributes: {
      objectClass: ['serviceAccount'],
      cn: ['svc-reports'],
      uid: ['svc-reports'],
      description: ['reporting bridge'],
    },
  },
  {
    dn: 'uid=mallory,ou=contractors,dc=corp,dc=example',
    password: 'contractor-secret',
    attributes: {
      objectClass: ['inetOrgPerson'],
      uid: ['mallory'],
      cn: ['Mallory Doe'],
      mail: ['mallory@vendor.example'],
      ou: ['contractors'],
      title: ['Vendor'],
    },
  },
];

export function inScope(entryDn, base, scope) {
  if (scope === 'base') return isDescendantDN(entryDn, base) && parseDN(entryDn).length === parseDN(base).length;
  if (scope === 'one') return isDirectChildDN(entryDn, base);
  return isDescendantDN(entryDn, base);
}
