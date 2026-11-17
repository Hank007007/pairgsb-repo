import path from 'node:path';

import { chainDigest, macEquals, recordMac } from './hmac.js';
import { listSegments, readSegment } from './journal.js';

const ZERO_MAC = '0'.repeat(64);

/**
 * 逐个文件核对一条链。
 *
 * 返回 { ok, segments: [{ file, entries, ok, reason }], broken }。只读、不修、不写。
 */
export async function verifyChain(dir, key) {
  const files = await listSegments(dir);
  const segments = [];
  let broken = null;
  let prevMac = ZERO_MAC;
  let prevSeq = 0;
  let expectedIndex = 0;
  let seenEntry = false;

  for (const file of files) {
    const index = segmentIndex(file);
    const entries = await readSegment(file);
    const gap = index !== expectedIndex;
    const result = verifySegment({
      file,
      index,
      entries,
      prevMac,
      expectedSeq: seenEntry ? prevSeq + 1 : 1,
      checkCrossLink: !gap,
      checkFirstSeq: !gap,
      gap,
      key,
    });

    segments.push({
      file: result.file,
      entries: result.entries,
      ok: result.ok,
      reason: result.reason,
    });
    if (!result.ok && broken === null) {
      broken = { file: result.file, seq: result.brokenSeq, reason: result.reason };
    }

    if (entries.length > 0) {
      const last = entries[entries.length - 1];
      prevMac = last.mac;
      prevSeq = last.seq;
      seenEntry = true;
    }
    expectedIndex = index + 1;
  }

  return { ok: broken === null, segments, broken };
}

function verifySegment({
  file,
  index,
  entries,
  prevMac,
  expectedSeq,
  checkCrossLink,
  checkFirstSeq,
  gap,
  key,
}) {
  const fail = (reason, brokenSeq) => ({
    file,
    entries: entries.length,
    ok: false,
    reason,
    brokenSeq,
  });

  if (gap) {
    return fail('segment-link', entries.length > 0 ? entries[0].seq : null);
  }
  if (entries.length === 0) {
    return fail('segment-digest', null);
  }

  const last = entries[entries.length - 1];
  for (let i = 0; i < entries.length - 1; i += 1) {
    if (Object.prototype.hasOwnProperty.call(entries[i], 'segmentDigest')) {
      return fail('segment-digest', entries[i].seq);
    }
  }
  if (typeof last.segmentDigest !== 'string') {
    return fail('segment-digest', last.seq);
  }

  for (const entry of entries) {
    if (!macEquals(entry.mac, recordMac(entry, key))) {
      return fail('record-mac', entry.seq);
    }
  }

  for (let i = 0; i < entries.length; i += 1) {
    const entry = entries[i];
    if (i === 0) {
      if (checkCrossLink && !macEquals(entry.prevMac, prevMac)) {
        return fail('chain-link', entry.seq);
      }
    } else if (!macEquals(entry.prevMac, entries[i - 1].mac)) {
      return fail('chain-link', entry.seq);
    }
  }

  let seq = expectedSeq;
  for (let i = 0; i < entries.length; i += 1) {
    const entry = entries[i];
    if (i === 0 && !checkFirstSeq) {
      seq = entry.seq;
    }
    if (entry.seq !== seq || entry.segment !== index) {
      return fail('chain-link', entry.seq);
    }
    seq += 1;
  }

  const expectedDigest = chainDigest(index, entries.length, last.mac, key);
  if (!macEquals(last.segmentDigest, expectedDigest)) {
    return fail('segment-digest', last.seq);
  }

  return { file, entries: entries.length, ok: true, reason: null, brokenSeq: null };
}

function segmentIndex(file) {
  const match = /^segment-(\d{4})\.jsonl$/.exec(path.basename(file));
  return match ? Number(match[1]) : -1;
}
