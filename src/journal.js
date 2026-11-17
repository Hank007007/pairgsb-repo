import { mkdir, open, readFile, readdir, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';

import { chainDigest, getKey, recordMac } from './hmac.js';

export const MANIFEST = 'manifest.json';
const ZERO_MAC = '0'.repeat(64);

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
    this.prevMac = ZERO_MAC;
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
    const recovered = await this.recoverState();
    let manifest = null;
    try {
      manifest = JSON.parse(await readFile(this.manifestPath(), 'utf8'));
    } catch {
      manifest = null;
    }
    const state =
      manifest && this.manifestMatches(manifest, recovered) ? manifest : recovered;
    this.segment = state.segment;
    this.seq = state.seq;
    this.prevMac = state.prevMac;
    this.count = state.count;
    return this;
  }

  async append(record) {
    const last = await this.lastMac(this.segment);
    this.prevMac = last ?? this.prevMac;

    if (this.count >= this.maxEntries) {
      this.segment += 1;
      this.count = 0;
    } else if (this.count > 0) {
      await this.clearLastDigest(this.segment);
    }

    const seq = (this.seq += 1);
    const body = { ...record, seq, segment: this.segment };
    const macBody = { ...body, prevMac: this.prevMac };
    const mac = recordMac(macBody, this.macKey());
    const segmentDigest = chainDigest(this.segment, this.count + 1, mac, this.macKey());
    const entry = { ...macBody, segmentDigest, mac };

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

  async recoverState() {
    const files = await listSegments(this.dir);
    if (files.length === 0) {
      return { segment: 0, seq: 0, prevMac: ZERO_MAC, count: 0 };
    }

    const lastFile = files[files.length - 1];
    const lastSegment = segmentIndex(lastFile);
    const lastEntries = await readSegment(lastFile);
    let lastNonEmpty = lastEntries;
    for (let i = files.length - 2; i >= 0 && lastNonEmpty.length === 0; i -= 1) {
      lastNonEmpty = await readSegment(files[i]);
    }
    const lastEntry = lastNonEmpty[lastNonEmpty.length - 1];
    return {
      segment: lastSegment,
      seq: lastEntry ? lastEntry.seq : 0,
      prevMac: lastEntry ? lastEntry.mac : ZERO_MAC,
      count: lastEntries.length,
    };
  }

  manifestMatches(manifest, recovered) {
    return (
      manifest.segment === recovered.segment &&
      manifest.seq === recovered.seq &&
      manifest.prevMac === recovered.prevMac &&
      manifest.count === recovered.count
    );
  }

  async clearLastDigest(segment) {
    const file = this.segmentPath(segment);
    let entries;
    try {
      entries = await readSegment(file);
    } catch {
      return;
    }
    if (entries.length === 0) return;
    const last = entries[entries.length - 1];
    if (!('segmentDigest' in last)) return;
    delete last.segmentDigest;
    await writeFile(file, `${entries.map((entry) => JSON.stringify(entry)).join('\n')}\n`, 'utf8');
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

function segmentIndex(file) {
  const match = /^segment-(\d{4})\.jsonl$/.exec(path.basename(file));
  return match ? Number(match[1]) : -1;
}
