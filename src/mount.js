import fs from 'node:fs';
import path from 'node:path';

import { safeName } from './names.js';

export function prepareDirectory(dir) {
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

export function writeSecret(dir, name, value) {
  const target = path.join(dir, name);
  fs.writeFileSync(target, value);
  fs.chmodSync(target, 0o644);
  return target;
}

export function materialize(dir, secrets) {
  prepareDirectory(dir);

  const written = [];
  for (const [name, value] of Object.entries(secrets)) {
    if (!safeName(name)) continue;
    writeSecret(dir, name, value);
    written.push(name);
  }
  return written;
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
    if (entry.isDirectory()) continue;
    fs.rmSync(path.join(dir, entry.name), { force: true });
  }
}
