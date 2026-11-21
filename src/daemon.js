'use strict';

const path = require('node:path');

const { SourceReader } = require('./reader');
const { Spool } = require('./spool');
const { BatchBuilder } = require('./batch');
const { scrubBatch } = require('./redact');
const { stampAll } = require('./clock');
const { deliverOnce, httpTransport } = require('./deliver');

const DEFAULT_BATCH_SIZE = 100;

/**
 * The relay: read the source file into the spool, cut a batch, redact it,
 * stamp it, send it, and move the cursor once it is accepted.
 */
class Relay {
  constructor(options = {}) {
    this.source = options.source === undefined ? 'node' : options.source;
    this.sourcePath = options.sourcePath;
    this.spoolDir = options.spoolDir;
    this.collectorUrl = options.collectorUrl;
    this.transport = options.transport || httpTransport;
    this.batchSize = options.batchSize === undefined ? DEFAULT_BATCH_SIZE : options.batchSize;
    this.deliveryOptions = options.deliveryOptions || {};
    this.maxRecords = options.maxRecords;

    this.reader = new SourceReader(this.sourcePath);
    this.spool = new Spool(this.spoolDir, { maxRecords: this.maxRecords });
    this.builder = new BatchBuilder({ source: this.source });
    this.offset = 0;
  }

  start() {
    this.spool.load();
    this.offset = this.spool.getCursor();
    return this;
  }

  /**
   * Read everything currently available into the spool.
   */
  ingest() {
    const result = this.reader.readFrom(this.offset);
    if (result.events.length === 0) {
      return { ingested: 0, offset: this.offset };
    }
    const records = result.events.map((event) => ({
      seq: event.seq,
      ts: event.ts,
      prev_hash: event.prev_hash,
      payload: event.payload,
      meta: event.meta,
    }));
    this.spool.saveCursor(result.offset, records);
    this.offset = result.offset;
    return { ingested: records.length, offset: this.offset };
  }

  /**
   * Send one batch of pending records. Returns the delivery result plus how
   * many records were committed.
   */
  async flush() {
    const pending = this.spool.pending();
    if (pending.length === 0) {
      return { ok: true, sent: 0 };
    }

    const records = pending.slice(0, this.batchSize);
    const prepared = scrubBatch({ records: stampAll(records) }).records;
    const batch = this.builder.buildBatch(prepared);

    const result = await deliverOnce(batch, this.transport, {
      url: this.collectorUrl,
      ...this.deliveryOptions,
    });

    if (!result.ok) {
      return { ...result, sent: 0 };
    }

    this.spool.markDelivered(records);
    return { ...result, sent: records.length };
  }
}

async function main() {
  const root = process.cwd();
  const relay = new Relay({
    source: process.env.SEALFERRY_SOURCE || 'node',
    sourcePath: process.env.SEALFERRY_SOURCE_FILE || path.join(root, 'data', 'events.log'),
    spoolDir: process.env.SEALFERRY_SPOOL_DIR || path.join(root, 'spool'),
    collectorUrl: process.env.SEALFERRY_COLLECTOR_URL || 'http://127.0.0.1:8787/ingest',
    maxRecords: process.env.SEALFERRY_MAX_RECORDS
      ? Number(process.env.SEALFERRY_MAX_RECORDS)
      : undefined,
  });

  relay.start();
  relay.ingest();
  const result = await relay.flush();
  process.stdout.write(`${JSON.stringify(result)}\n`);
}

if (require.main === module) {
  main().catch((err) => {
    process.stderr.write(`${err.stack || err}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  Relay,
  DEFAULT_BATCH_SIZE,
};
