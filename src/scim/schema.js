'use strict';

const USER_ATTRIBUTES = {
  userName: { type: 'string', required: true },
  externalId: { type: 'string' },
  name: {
    type: 'complex',
    sub: { givenName: 'string', familyName: 'string' },
  },
  emails: {
    type: 'multi',
    element: { value: 'string', type: 'string', primary: 'boolean' },
  },
  active: { type: 'boolean', sensitive: 'active' },
  groups: {
    type: 'multi',
    sensitive: 'groups',
    element: { value: 'string', display: 'string' },
  },
  roles: {
    type: 'multi',
    sensitive: 'roles',
    element: { value: 'string', display: 'string' },
  },
};

const GROUP_ATTRIBUTES = {
  displayName: { type: 'string', required: true },
  members: {
    type: 'multi',
    sensitive: 'groups',
    element: { value: 'string', display: 'string', type: 'string' },
  },
};

const USER_SCHEMA = 'urn:ietf:params:scim:schemas:core:2.0:User';
const GROUP_SCHEMA = 'urn:ietf:params:scim:schemas:core:2.0:Group';
const LIST_SCHEMA = 'urn:ietf:params:scim:api:messages:2.0:ListResponse';

function schemaFor(resourceType) {
  return resourceType === 'Group' ? GROUP_ATTRIBUTES : USER_ATTRIBUTES;
}

function sensitiveAttributeOf(resourceType, attribute) {
  const schema = schemaFor(resourceType);
  const definition = schema[attribute];
  return definition && definition.sensitive ? definition.sensitive : null;
}

function isKnownAttribute(resourceType, attribute) {
  return Object.prototype.hasOwnProperty.call(schemaFor(resourceType), attribute);
}

module.exports = {
  USER_ATTRIBUTES,
  GROUP_ATTRIBUTES,
  USER_SCHEMA,
  GROUP_SCHEMA,
  LIST_SCHEMA,
  schemaFor,
  sensitiveAttributeOf,
  isKnownAttribute,
};
