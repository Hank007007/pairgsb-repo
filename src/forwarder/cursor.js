'use strict';

const fs = require('node:fs');
const path = require('node:path');

function cursorPathFor(stateDir, origin, epoch) {
  return path.join(stateDir, 'cursors', `${encodeURIComponent(origin)}.${epoch}.cursor.json`);
}

function writeCursor(cursorPath, cursor) {
  fs.mkdirSync(path.dirname(cursorPath), { recursive: true });
  const existed = fs.existsSync(cursorPath);
  const fd = fs.openSync(cursorPath, 'w');
  try {
    fs.writeFileSync(fd, `${JSON.stringify(cursor)}\n`, 'utf8');
    fs.fsyncSync(fd);
  } finally {
    fs.closeSync(fd);
  }
  if (!existed) {
    try {
      const dirFd = fs.openSync(path.dirname(cursorPath), 'r');
      try {
        fs.fsyncSync(dirFd);
      } finally {
        fs.closeSync(dirFd);
      }
    } catch {
      // Directory fsync is not supported on every platform (e.g. Windows).
    }
  }
}

function freshCursor(stream) {
  const files = stream && Array.isArray(stream.files) ? stream.files : [];
  const first = files.length > 0 ? files[0] : null;
  return {
    version: 1,
    origin: stream ? stream.origin : null,
    epoch: stream ? stream.epoch : null,
    segment: first ? first.name : null,
    offset: 0,
    nextSeq: 0,
    ackedSeq: -1,
    updatedAt: new Date().toISOString(),
  };
}

function openCursor(cursorPath, stream) {
  let text;
  try {
    text = fs.readFileSync(cursorPath, 'utf8');
  } catch (err) {
    if (err.code !== 'ENOENT') throw err;
    const cursor = freshCursor(stream);
    writeCursor(cursorPath, cursor);
    return cursor;
  }
  return JSON.parse(text);
}

function advance(cursorPath, cursor, position) {
  const next = {
    ...cursor,
    ...position,
    ackedSeq: position.nextSeq - 1,
    updatedAt: new Date().toISOString(),
  };
  writeCursor(cursorPath, next);
  return next;
}

module.exports = {
  advance,
  cursorPathFor,
  freshCursor,
  openCursor,
  writeCursor,
};
