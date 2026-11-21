'use strict';

const fs = require('node:fs');
const path = require('node:path');

const { SpoolError } = require('./errors');
const { digestPayload } = require('../protocol/manifest');

const DEFAULT_LIMIT = 100;
const DEFAULT_MAX_RECORD_BYTES = 8192;
const SEGMENT_EXTENSION = '.ndjson';

function segmentFileName(origin, epoch, index) {
  const encoded = encodeURIComponent(origin);
  return `${encoded}.${String(epoch)}.${String(index).padStart(6, '0')}${SEGMENT_EXTENSION}`;
}

function parseSegmentName(name) {
  if (!name.endsWith(SEGMENT_EXTENSION)) return null;
  const base = name.slice(0, -SEGMENT_EXTENSION.length);
  const parts = base.split('.');
  if (parts.length < 3) return null;
  const indexText = parts.pop();
  const epochText = parts.pop();
  if (!/^\d+$/.test(indexText) || !/^\d+$/.test(epochText)) return null;
  let origin;
  try {
    origin = decodeURIComponent(parts.join('.'));
  } catch {
    return null;
  }
  if (!origin) return null;
  return { name, origin, epoch: Number(epochText), index: Number(indexText) };
}

function listStreams(spoolDir) {
  let entries;
  try {
    entries = fs.readdirSync(spoolDir, { withFileTypes: true });
  } catch (err) {
    if (err.code === 'ENOENT') return [];
    throw err;
  }

  const streams = new Map();
  for (const entry of entries) {
    if (!entry.isFile()) continue;
    const parsed = parseSegmentName(entry.name);
    if (!parsed) continue;
    const key = `${parsed.origin}\u0000${parsed.epoch}`;
    let stream = streams.get(key);
    if (!stream) {
      stream = { origin: parsed.origin, epoch: parsed.epoch, files: [] };
      streams.set(key, stream);
    }
    const stat = fs.statSync(path.join(spoolDir, entry.name));
    stream.files.push({ name: parsed.name, index: parsed.index, size: stat.size });
  }
  const ordered = Array.from(streams.values());
  for (const stream of ordered) {
    stream.files.sort((a, b) => a.index - b.index);
  }
  ordered.sort((a, b) => (a.origin === b.origin ? a.epoch - b.epoch : a.origin < b.origin ? -1 : 1));
  return ordered;
}

/**
 * Reads at most `limit` records for one stream, starting at the cursor
 * position. Returns null when the cursor is already at the end of the stream.
 */
function readBatch(options) {
  const {
    spoolDir,
    stream,
    cursor,
    limit = DEFAULT_LIMIT,
    maxRecordBytes = DEFAULT_MAX_RECORD_BYTES,
  } = options;

  if (!stream || !Array.isArray(stream.files) || stream.files.length === 0) return null;
  if (!Number.isInteger(limit) || limit <= 0) {
    throw new SpoolError('batch limit must be a positive integer', { code: 'invalid_limit' });
  }

  const firstSeq = Number.isInteger(cursor.nextSeq) ? cursor.nextSeq : 0;
  const startName = cursor.segment;
  let startIndex = 0;
  if (startName) {
    const found = stream.files.findIndex((file) => file.name === startName);
    if (found === -1) {
      return null;
    }
    startIndex = found;
  }

  const records = [];
  let position = {
    segment: startName || stream.files[startIndex].name,
    offset: startName ? cursor.offset || 0 : 0,
    nextSeq: firstSeq,
  };

  for (let fileIndex = startIndex; fileIndex < stream.files.length && records.length < limit; fileIndex += 1) {
    const file = stream.files[fileIndex];
    const filePath = path.join(spoolDir, file.name);
    const buffer = fs.readFileSync(filePath);
    let offset = file.name === position.segment ? Math.min(position.offset, buffer.length) : 0;

    while (records.length < limit && offset < buffer.length) {
      const newline = buffer.indexOf(0x0a, offset);
      const end = newline === -1 ? buffer.length : newline;
      const size = end - offset;
      if (size > maxRecordBytes) {
        throw new SpoolError(
          `record in ${file.name} at byte offset ${offset} is ${size} bytes, over the ${maxRecordBytes} byte limit`,
          { code: 'record_too_large', file: file.name, offset },
        );
      }
      records.push(buffer.subarray(offset, end).toString('utf8'));
      offset = newline === -1 ? buffer.length : newline + 1;
      position = {
        segment: file.name,
        offset,
        nextSeq: firstSeq + records.length,
      };
    }
  }

  if (records.length === 0) return null;

  const body = records.map((line) => `${line}\n`).join('');
  return {
    origin: stream.origin,
    epoch: stream.epoch,
    firstSeq,
    count: records.length,
    digest: digestPayload(body),
    records,
    body,
    position,
  };
}

module.exports = {
  DEFAULT_LIMIT,
  DEFAULT_MAX_RECORD_BYTES,
  listStreams,
  parseSegmentName,
  readBatch,
  segmentFileName,
};
