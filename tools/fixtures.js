import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const testdata = new URL('../testdata/', import.meta.url);

export function loadSnapshot(name) {
  return JSON.parse(readFileSync(new URL(`schemas/${name}`, testdata), 'utf8'));
}

export function loadMessage(name) {
  return readFileSync(new URL(`messages/${name}`, testdata));
}

export const rootDir = fileURLToPath(new URL('../', import.meta.url));
