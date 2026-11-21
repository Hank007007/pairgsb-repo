'use strict';

const fs = require('node:fs');
const path = require('node:path');

const QUEUE_FILE = 'queue.log';
const CURSOR_FILE = 'cursor.json';

/**
 * The durable local queue. Events read from the source file are appended here
 * before any delivery attempt; records are removed only once their batch has
 * been acknowledged.
 */
class Spool {
  constructor(dir, options = {}) {
    this.dir = dir;
    this.maxRecords = options.maxRecords === undefined ? 1000 : options.maxRecords;
    this.fs = options.fs || fs;
    this.logPath = path.join(dir, QUEUE_FILE);
    this.cursorPath = path.join(dir, CURSOR_FILE);
    this.records = [];
    this.cursor = 0;
  }

  /**
   * Load the persisted cursor and queue. Returns this spool.
   */
  load() {
    this.fs.mkdirSync(this.dir, { recursive: true });

    try {
      const raw = this.fs.readFileSync(this.cursorPath, 'utf8');
      const parsed = JSON.parse(raw);
      this.cursor = Number.isFinite(parsed.offset) ? parsed.offset : 0;
    } catch (err) {
      if (err.code !== 'ENOENT' && !(err instanceof SyntaxError)) {
        throw err;
      }
      this.cursor = 0;
    }

    let text = '';
    try {
      text = this.fs.readFileSync(this.logPath, 'utf8');
    } catch (err) {
      if (err.code !== 'ENOENT') {
        throw err;
      }
      this.records = [];
      return this;
    }

    this.records = text
      .split('\n')
      .filter(Boolean)
      .map((line) => {
        try {
          return JSON.parse(line);
        } catch (err) {
          return null;
        }
      })
      .filter(Boolean);

    return this;
  }

  /**
   * Append one record to the queue and enforce the capacity limit.
   */
  append(record) {
    this.fs.appendFileSync(this.logPath, `${JSON.stringify(record)}\n`, 'utf8');
    this.records.push(record);
    this.trim();
    return record;
  }

  /**
   * Persist the read position, then append the events that position covers.
   */
  saveCursor(offset, records = []) {
    this.fs.writeFileSync(this.cursorPath, JSON.stringify({ offset }), 'utf8');
    this.cursor = offset;
    for (const record of records) {
      this.append(record);
    }
  }

  /**
   * Keep the newest `maxRecords` entries.
   */
  trim() {
    while (this.records.length > this.maxRecords) {
      this.records.shift();
    }
  }

  pending() {
    return this.records.slice();
  }

  /**
   * Drop acknowledged records from the queue and rewrite the queue file with
   * whatever remains.
   */
  markDelivered(delivered) {
    const done = new Set(delivered);
    this.records = this.records.filter((record) => !done.has(record));
    const body = this.records.map((record) => JSON.stringify(record)).join('\n');
    this.fs.writeFileSync(this.logPath, this.records.length ? `${body}\n` : '', 'utf8');
  }

  getCursor() {
    return this.cursor;
  }
}

module.exports = {
  Spool,
  QUEUE_FILE,
  CURSOR_FILE,
};
