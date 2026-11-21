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
  return Array.from(streams.values());
}

function truncateLine(line, maxRecordBytes) {
  if (Buffer.byteLength(line, 'utf8') <= maxRecordBytes) return line;
  return Buffer.from(line, 'utf8').subarray(0, maxRecordBytes).toString('utf8');
}

function lineParts(buffer, startOffset) {
  const text = buffer.subarray(startOffset).toString('utf8');
  const parts = text.split('\n');
  parts.pop();
  return parts;
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

  for (let fileIndex = startIndex; fileIndex < stream.files.length; fileIndex += 1) {
    if (records.length >= limit) break;
    const file = stream.files[fileIndex];
    const filePath = path.join(spoolDir, file.name);
    const buffer = fs.readFileSync(filePath);
    let offset = 0;
    if (file.name === position.segment) {
      offset = Math.min(position.offset, buffer.length);
    }
    const parts = lineParts(buffer, offset);

    if (parts.length === 0) {
      if (fileIndex + 1 < stream.files.length) {
        position = {
          segment: stream.files[fileIndex + 1].name,
          offset: 0,
          nextSeq: firstSeq + records.length,
        };
      }
      continue;
    }

    for (const part of parts) {
      if (records.length >= limit) break;
      records.push(truncateLine(part, maxRecordBytes));
      offset += Buffer.byteLength(part, 'utf8') + 1;
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
