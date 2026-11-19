'use strict';

const fs = require('node:fs');
const path = require('node:path');

function redactQuery(pathAndQuery) {
  if (typeof pathAndQuery !== 'string' || pathAndQuery.length === 0) return '/';
  const hashIndex = pathAndQuery.indexOf('#');
  const withoutFragment = hashIndex === -1 ? pathAndQuery : pathAndQuery.slice(0, hashIndex);
  const questionIndex = withoutFragment.indexOf('?');
  if (questionIndex === -1) return withoutFragment;

  const pathname = withoutFragment.slice(0, questionIndex);
  const query = withoutFragment.slice(questionIndex + 1);
  if (query.length === 0) return `${pathname}?`;

  const firstPair = query.split('&')[0];
  const firstName = firstPair.split('=')[0];
  return `${pathname}?${firstName}=…`;
}

function errorCode(err) {
  if (!err) return null;
  if (typeof err.code === 'string') return err.code;
  return 'EUNKNOWN';
}

/**
 * Appends one record per hop to the audit file. The file is opened lazily and
 * kept for the lifetime of the process; each record is written with a single
 * append call.
 */
function createAuditLog(options) {
  const file = options && options.file ? path.resolve(options.file) : null;
  let handle = null;

  function open() {
    if (handle !== null) return handle;
    if (file === null) return null;
    fs.mkdirSync(path.dirname(file), { recursive: true });
    handle = fs.openSync(file, 'a', 0o600);
    return handle;
  }

  function logRequest(record) {
    const target = open();
    if (target === null) return;

    const entry = {
      ts: record.ts instanceof Date ? record.ts.toISOString() : new Date().toISOString(),
      host: record.host || null,
      path: redactQuery(record.path),
      status: Number.isInteger(record.status) ? record.status : null,
      durationMs: typeof record.durationMs === 'number' ? Math.round(record.durationMs * 1000) / 1000 : null,
      keyId: record.keyId || null,
      error: errorCode(record.error),
    };

    fs.writeSync(target, `${JSON.stringify(entry)}\n`);
  }

  function close() {
    if (handle !== null) {
      fs.closeSync(handle);
      handle = null;
    }
  }

  return { logRequest, close };
}

module.exports = { createAuditLog, redactQuery };
