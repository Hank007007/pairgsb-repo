/**
 * Parsing and formatting of the tuple syntax:
 *
 *   object#relation@subject
 *   doc:1#reader@user:alice
 *   doc:1#reader@group:eng#member
 *
 * Objects are `type:id`. Subjects are either a plain subject (`type:id`, with
 * `*` accepted as the id for a wildcard) or a userset (`type:id#relation`).
 */

import { AuthError, ErrorCodes } from './errors.js';

const TYPE_PATTERN = /^[a-z][a-z0-9_]*$/;
const RELATION_PATTERN = /^[a-z][a-z0-9_]*$/;
const ID_PATTERN = /^[A-Za-z0-9_.\-*]+$/;

function invalid(code, message, details) {
  throw new AuthError(code, message, details);
}

export function isValidType(type) {
  return typeof type === 'string' && TYPE_PATTERN.test(type);
}

export function isValidRelation(relation) {
  return typeof relation === 'string' && RELATION_PATTERN.test(relation);
}

/**
 * Parse `type:id` into an object reference.
 * @param {string} text
 * @param {string} [code] error code to use on failure
 * @returns {{ type: string, id: string }}
 */
export function parseObjectRef(text, code = ErrorCodes.INVALID_REQUEST) {
  if (typeof text !== 'string') {
    invalid(code, 'object must be a string of the form type:id');
  }
  const separator = text.indexOf(':');
  if (separator <= 0 || separator === text.length - 1) {
    invalid(code, `object "${text}" must be of the form type:id`, { object: text });
  }
  const type = text.slice(0, separator);
  const id = text.slice(separator + 1);
  if (!isValidType(type)) {
    invalid(code, `object "${text}" has an invalid type "${type}"`, { object: text, type });
  }
  if (!ID_PATTERN.test(id)) {
    invalid(code, `object "${text}" has an invalid id "${id}"`, { object: text, id });
  }
  return { type, id };
}

/**
 * Parse a subject. A subject is plain (`type:id`) or a userset
 * (`type:id#relation`).
 * @param {string} text
 * @param {string} [code]
 * @returns {{ kind: 'user'|'userset', type: string, id: string, relation?: string }}
 */
export function parseSubject(text, code = ErrorCodes.INVALID_REQUEST) {
  if (typeof text !== 'string' || text.length === 0) {
    invalid(code, 'subject must be a non-empty string');
  }
  const hash = text.indexOf('#');
  const objectPart = hash === -1 ? text : text.slice(0, hash);
  const object = parseObjectRef(objectPart, code);
  if (hash === -1) {
    return { kind: 'user', type: object.type, id: object.id };
  }
  const relation = text.slice(hash + 1);
  if (!isValidRelation(relation)) {
    invalid(code, `subject "${text}" has an invalid relation "${relation}"`, { subject: text });
  }
  return { kind: 'userset', type: object.type, id: object.id, relation };
}

/**
 * Parse a tuple given either as a string or as `{ object, relation, subject }`.
 * @param {string|{object: string, relation: string, subject: string}} value
 * @param {string} [code]
 */
export function parseTuple(value, code = ErrorCodes.INVALID_TUPLE) {
  let objectText;
  let relation;
  let subjectText;
  if (typeof value === 'string') {
    const at = value.indexOf('@');
    if (at === -1) {
      invalid(code, `tuple "${value}" is missing "@subject"`, { tuple: value });
    }
    const head = value.slice(0, at);
    subjectText = value.slice(at + 1);
    const hash = head.indexOf('#');
    if (hash === -1) {
      invalid(code, `tuple "${value}" is missing "#relation"`, { tuple: value });
    }
    objectText = head.slice(0, hash);
    relation = head.slice(hash + 1);
  } else if (value && typeof value === 'object') {
    objectText = value.object;
    relation = value.relation;
    subjectText = value.subject;
  } else {
    invalid(code, 'tuple must be a string or an object');
  }

  const object = parseObjectRef(objectText, code);
  if (!isValidRelation(relation)) {
    invalid(code, `tuple relation "${relation}" is invalid`, { relation });
  }
  const subject = parseSubject(subjectText, code);
  return {
    object,
    relation,
    subject,
    text: formatTupleParts(object, relation, subject),
  };
}

export function formatObjectRef(object) {
  return `${object.type}:${object.id}`;
}

export function formatSubject(subject) {
  const base = `${subject.type}:${subject.id}`;
  return subject.kind === 'userset' ? `${base}#${subject.relation}` : base;
}

export function formatTupleParts(object, relation, subject) {
  return `${formatObjectRef(object)}#${relation}@${formatSubject(subject)}`;
}

/** True when the tuple subject is a plain user (not a userset). */
export function isPlainSubject(subject) {
  return subject.kind === 'user';
}

/** True when the tuple subject is a wildcard (`type:*`). */
export function isWildcard(subject) {
  return subject.kind === 'user' && subject.id === '*';
}

/** Compare a stored subject with a queried subject, honouring wildcards. */
export function subjectMatches(stored, queried) {
  if (stored.kind !== 'user' || queried.kind !== 'user') return false;
  if (stored.type !== queried.type) return false;
  return stored.id === queried.id || stored.id === '*';
}
