'use strict';

class ScimError extends Error {
  constructor(status, scimType, detail) {
    super(detail || scimType || 'SCIM error');
    this.name = 'ScimError';
    this.status = status;
    this.scimType = scimType;
    this.detail = detail || null;
  }

  toBody() {
    const body = {
      schemas: ['urn:ietf:params:scim:api:messages:2.0:Error'],
      status: String(this.status),
    };
    if (this.scimType) body.scimType = this.scimType;
    if (this.detail) body.detail = this.detail;
    return body;
  }
}

function badRequest(scimType, detail) {
  return new ScimError(400, scimType, detail);
}

function unauthorized(detail) {
  return new ScimError(401, 'invalidToken', detail);
}

function forbidden(scimType, detail) {
  return new ScimError(403, scimType, detail);
}

function preconditionFailed(detail) {
  return new ScimError(412, 'preconditionFailed', detail);
}

function notFound(detail) {
  return new ScimError(404, null, detail);
}

module.exports = {
  ScimError,
  badRequest,
  unauthorized,
  forbidden,
  preconditionFailed,
  notFound,
};
