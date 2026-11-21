'use strict';

const fs = require('node:fs');
const path = require('node:path');

function cursorPathFor(stateDir, origin, epoch) {
  return path.join(stateDir, 'cursors', `${encodeURIComponent(origin)}.${epoch}.cursor.json`);
}

function writeCursor(cursorPath, cursor) {
  fs.mkdirSync(path.dirname(cursorPath), { recursive: true });
  fs.writeFileSync(cursorPath, `${JSON.stringify(cursor)}\n`, 'utf8');
}

function freshCursor(stream) {
  const files = stream && Array.isArray(stream.files) ? stream.files : [];
  const last = files.length > 0 ? files[files.length - 1] : null;
  return {
    version: 1,
    origin: stream ? stream.origin : null,
    epoch: stream ? stream.epoch : null,
    segment: last ? last.name : null,
    offset: last ? last.size : 0,
    nextSeq: 0,
    ackedSeq: 0,
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
