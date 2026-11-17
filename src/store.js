import fs from 'node:fs';
import path from 'node:path';

function emptyState() {
  return { version: 1, keys: {} };
}

export function createStore({ dataFile }) {
  let state = emptyState();

  function ensureDirectory() {
    fs.mkdirSync(path.dirname(dataFile), { recursive: true });
  }

  function load() {
    if (!fs.existsSync(dataFile)) {
      state = emptyState();
      return;
    }
    const raw = fs.readFileSync(dataFile, 'utf8');
    if (raw.trim() === '') {
      state = emptyState();
      return;
    }
    const parsed = JSON.parse(raw);
    state = {
      version: parsed.version ?? 1,
      keys: parsed.keys ?? {},
    };
  }

  function save() {
    ensureDirectory();
    const temporary = `${dataFile}.tmp`;
    fs.writeFileSync(temporary, `${JSON.stringify(state, null, 2)}\n`, 'utf8');
    for (let attempt = 0; ; attempt += 1) {
      try {
        fs.renameSync(temporary, dataFile);
        return;
      } catch (error) {
        if (attempt >= 20 || !['EPERM', 'EACCES', 'EBUSY'].includes(error.code)) {
          throw error;
        }
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 5);
      }
    }
  }

  function getById(id8) {
    const record = state.keys[id8];
    return record ? structuredClone(record) : null;
  }

  function put(record) {
    state.keys[record.id8] = structuredClone(record);
    save();
    return getById(record.id8);
  }

  function lookup(presentedKey) {
    for (const record of Object.values(state.keys)) {
      if (record.key === presentedKey) {
        return structuredClone(record);
      }
    }
    return null;
  }

  function update(id8, patch) {
    const current = state.keys[id8];
    if (!current) return null;
    state.keys[id8] = { ...current, ...structuredClone(patch) };
    save();
    return getById(id8);
  }

  function all() {
    return Object.values(state.keys).map((record) => structuredClone(record));
  }

  function reload() {
    load();
  }

  load();

  return {
    dataFile,
    getById,
    put,
    lookup,
    update,
    all,
    reload,
    save,
  };
}
