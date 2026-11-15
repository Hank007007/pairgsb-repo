import { macEquals, recordMac } from './hmac.js';
import { listSegments, readSegment } from './journal.js';

/**
 * 逐个文件核对一条链。
 *
 * 返回 { ok, segments: [{ file, entries, ok, reason }] }。只读、不修、不写。
 */
export async function verifyChain(dir, key) {
  const files = await listSegments(dir);
  const segments = [];
  let prevMac = '0'.repeat(64);

  for (const file of files) {
    const entries = await readSegment(file);
    const result = verifySegment(file, entries, prevMac, key);
    segments.push(result);
    if (!result.ok) {
      return { ok: false, segments };
    }
    if (entries.length > 0) {
      prevMac = entries[entries.length - 1].mac;
    }
  }

  return { ok: true, segments };
}

function verifySegment(file, entries, prevMac, key) {
  let running = prevMac;
  for (const entry of entries) {
    if (!macEquals(entry.prevMac, running)) {
      return { file, entries: entries.length, ok: false, reason: 'chain-link' };
    }
    running = entry.mac;
  }
  return { file, entries: entries.length, ok: true, reason: null };
}
