'use strict';

// Rendering and lifecycle of the per-job env file.

const fs = require('node:fs');
const path = require('node:path');

function render(values) {
  const lines = [];

  for (const name of Object.keys(values)) {
    const value = values[name];
    lines.push(name + '=' + String(value));
  }

  return lines.join('\n') + '\n';
}

function ensureDirectory(directory) {
  fs.mkdirSync(directory, { recursive: true });
}

function write(values, targetPath) {
  const directory = path.dirname(targetPath);
  ensureDirectory(directory);

  const contents = render(values);
  fs.writeFileSync(targetPath, contents, { encoding: 'utf8' });
  fs.chmodSync(targetPath, 0o600);

  return targetPath;
}

function remove(targetPath) {
  try {
    fs.unlinkSync(targetPath);
  } catch (error) {
    if (error && error.code === 'ENOENT') return false;
    throw error;
  }
  return true;
}

module.exports = { render, write, remove };
