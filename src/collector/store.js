'use strict';

const fs = require('node:fs');
const path = require('node:path');

function safeOrigin(origin) {
  return encodeURIComponent(origin);
}

/**
 * Append-only record store. Each origin/epoch pair lives in its own NDJSON
 * file, one stored record per line.
 */
class Store {
  constructor(rootDir) {
    this.rootDir = rootDir;
  }

  fileFor(origin, epoch) {
    return path.join(this.rootDir, safeOrigin(origin), `${epoch}.ndjson`);
  }

  put(entry) {
    const file = this.fileFor(entry.origin, entry.epoch);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    const line = JSON.stringify({
      origin: entry.origin,
      epoch: entry.epoch,
      seq: entry.seq,
      record: entry.record,
      receivedAt: entry.receivedAt || new Date().toISOString(),
    });
    fs.appendFileSync(file, `${line}\n`, 'utf8');
    return entry;
  }

  recordsFor(origin, epoch) {
    const file = this.fileFor(origin, epoch);
    let text;
    try {
      text = fs.readFileSync(file, 'utf8');
    } catch (err) {
      if (err.code === 'ENOENT') return [];
      throw err;
    }
    return text
      .split('\n')
      .filter((line) => line.length > 0)
      .map((line) => JSON.parse(line));
  }

  countFor(origin, epoch) {
    return this.recordsFor(origin, epoch).length;
  }

  /**
   * Returns every stored entry across all (origin, epoch) streams. Used to
   * rebuild deduplication and gap state after a collector restart.
   */
  entries() {
    let dirs;
    try {
      dirs = fs.readdirSync(this.rootDir, { withFileTypes: true });
    } catch (err) {
      if (err.code === 'ENOENT') return [];
      throw err;
    }
    const out = [];
    for (const dir of dirs) {
      if (!dir.isDirectory()) continue;
      let origin;
      try {
        origin = decodeURIComponent(dir.name);
      } catch {
        continue;
      }
      for (const file of fs.readdirSync(path.join(this.rootDir, dir.name))) {
        if (!file.endsWith('.ndjson')) continue;
        const epoch = Number(file.slice(0, -'.ndjson'.length));
        if (!Number.isInteger(epoch) || epoch < 0) continue;
        for (const entry of this.recordsFor(origin, epoch)) {
          out.push(entry);
        }
      }
    }
    return out;
  }
}

module.exports = { Store };
