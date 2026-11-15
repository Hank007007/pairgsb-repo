import { mkdir, open, readFile, readdir, stat } from 'node:fs/promises';
import path from 'node:path';

import { getKey, recordMac } from './hmac.js';

export const MANIFEST = 'manifest.json';

/**
 * 追加式审计日志。按 maxEntries 切段，每段一个 JSONL 文件。
 */
export class Journal {
  constructor({ dir, maxEntries = 500, key = null }) {
    this.dir = dir;
    this.maxEntries = maxEntries;
    this.key = key;
    this.segment = 0;
    this.seq = 0;
    this.prevMac = '0'.repeat(64);
    this.count = 0;
  }

  macKey() {
    return this.key ?? getKey();
  }

  segmentPath(index) {
    return path.join(this.dir, `segment-${String(index).padStart(4, '0')}.jsonl`);
  }

  manifestPath() {
    return path.join(this.dir, MANIFEST);
  }

  async open() {
    await mkdir(this.dir, { recursive: true });
    let manifest = null;
    try {
      manifest = JSON.parse(await readFile(this.manifestPath(), 'utf8'));
    } catch {
      manifest = null;
    }
    if (manifest) {
      this.segment = manifest.segment;
      this.seq = manifest.seq;
      this.prevMac = manifest.prevMac;
      this.count = manifest.count;
    }
    return this;
  }

  async append(record) {
    const last = await this.lastMac(this.segment);
    this.prevMac = last ?? this.prevMac;

    if (this.count >= this.maxEntries) {
      this.segment += 1;
      this.count = 0;
    }

    const seq = (this.seq += 1);
    const body = { ...record, seq, segment: this.segment };
    const entry = { ...body, prevMac: this.prevMac, mac: recordMac(body, this.macKey()) };

    const handle = await open(this.segmentPath(this.segment), 'a');
    try {
      await handle.write(`${JSON.stringify(entry)}\n`);
    } finally {
      await handle.close();
    }

    this.prevMac = entry.mac;
    this.count += 1;
    await this.writeManifest();
    return entry;
  }

  async lastMac(segment) {
    let text;
    try {
      text = await readFile(this.segmentPath(segment), 'utf8');
    } catch {
      return null;
    }
    const lines = text.split('\n').filter((line) => line.trim() !== '');
    if (lines.length === 0) return null;
    try {
      return JSON.parse(lines[lines.length - 1]).mac ?? null;
    } catch {
      return null;
    }
  }

  async writeManifest() {
    const body = JSON.stringify({
      segment: this.segment,
      seq: this.seq,
      prevMac: this.prevMac,
      count: this.count,
    });
    const handle = await open(this.manifestPath(), 'w');
    try {
      await handle.write(body);
    } finally {
      await handle.close();
    }
  }
}

export async function listSegments(dir) {
  let names = [];
  try {
    names = await readdir(dir);
  } catch {
    return [];
  }
  return names
    .filter((name) => /^segment-\d{4}\.jsonl$/.test(name))
    .sort()
    .map((name) => path.join(dir, name));
}

export async function readSegment(file) {
  const text = await readFile(file, 'utf8');
  return text
    .split('\n')
    .filter((line) => line.trim() !== '')
    .map((line) => JSON.parse(line));
}

export async function segmentExists(file) {
  try {
    await stat(file);
    return true;
  } catch {
    return false;
  }
}
