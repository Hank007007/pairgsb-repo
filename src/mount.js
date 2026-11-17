import fs from 'node:fs';
import crypto from 'node:crypto';
import path from 'node:path';

import { safeName } from './names.js';

const DIR_MODE = 0o700;
const FILE_MODE = 0o400;

export function prepareDirectory(dir) {
  fs.mkdirSync(dir, { recursive: true });
  fs.chmodSync(dir, DIR_MODE);
  return dir;
}

function tempPath(dir) {
  const suffix = crypto.randomBytes(8).toString('hex');
  return path.join(dir, `.sidemount-${process.pid}-${suffix}.tmp`);
}

export function writeSecret(dir, name, value) {
  const target = path.join(dir, name);
  const temp = tempPath(dir);
  try {
    fs.writeFileSync(temp, value, { mode: FILE_MODE });
    fs.chmodSync(temp, FILE_MODE);
    if (process.platform === 'win32') {
      // Windows refuses to rename over a read-only file; the final mode is
      // still enforced by the fresh 0400 temp file moved into place.
      const existing = fs.lstatSync(target, { throwIfNoEntry: false });
      if (existing && existing.isFile()) fs.chmodSync(target, 0o600);
    }
    fs.renameSync(temp, target);
  } catch (err) {
    fs.rmSync(temp, { force: true });
    throw err;
  }
  return target;
}

function assertRegularOrAbsent(dir, name) {
  const target = path.join(dir, name);
  const stat = fs.lstatSync(target, { throwIfNoEntry: false });
  if (stat && !stat.isFile()) {
    throw new Error(`target path ${JSON.stringify(name)} exists and is not a regular file`);
  }
}

export function materialize(dir, secrets) {
  prepareDirectory(dir);

  const admissible = Object.entries(secrets).filter(([name]) => safeName(name));

  for (const [name] of admissible) {
    assertRegularOrAbsent(dir, name);
  }

  const written = [];
  for (const [name, value] of admissible) {
    writeSecret(dir, name, value);
    written.push(name);
  }
  return written;
}

export function removeMounted(dir, name) {
  const target = path.join(dir, name);
  const stat = fs.lstatSync(target, { throwIfNoEntry: false });
  if (stat && (stat.isFile() || stat.isSymbolicLink())) {
    fs.unlinkSync(target);
  }
}

export function cleanup(dir) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (err) {
    if (err.code === 'ENOENT') return;
    throw err;
  }

  for (const entry of entries) {
    if (!entry.isFile()) continue;
    fs.rmSync(path.join(dir, entry.name), { force: true });
  }
}
