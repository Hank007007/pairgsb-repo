'use strict';

function seed(app) {
  const acmeUser = app.users.create('acme', {
    userName: 'ada@acme.example',
    externalId: 'hr-1001',
    name: { givenName: 'Ada', familyName: 'Lovelace' },
    emails: [{ value: 'ada@acme.example', type: 'work', primary: true }],
    active: true,
    groups: [{ value: 'acme-g0001', display: 'Engineering' }],
    roles: [{ value: 'engineer', display: 'Engineer' }],
  });
  const grace = app.users.create('acme', {
    userName: 'grace@acme.example',
    externalId: 'hr-1002',
    name: { givenName: 'Grace', familyName: 'Hopper' },
    emails: [{ value: 'grace@acme.example', type: 'work', primary: true }],
    active: true,
    groups: [{ value: 'acme-g0001', display: 'Engineering' }],
    roles: [{ value: 'engineer', display: 'Engineer' }],
  });

  const acmeGroup = app.groups.create('acme', {
    displayName: 'Engineering',
    members: [
      { value: acmeUser.id, display: 'ada@acme.example', type: 'User' },
      { value: grace.id, display: 'grace@acme.example', type: 'User' },
    ],
  });

  app.groups.create('globex', {
    displayName: 'Contractors',
    members: [],
  });
  app.users.create('globex', {
    userName: 'hank@globex.example',
    externalId: 'hr-2001',
    name: { givenName: 'Hank', familyName: 'Scorpio' },
    emails: [{ value: 'hank@globex.example', type: 'work', primary: true }],
    active: true,
    groups: [],
    roles: [],
  });

  return { acmeGroup };
}

module.exports = { seed };
