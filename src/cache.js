function snapshot(record) {
  return record ? structuredClone(record) : null;
}

export function createCache({ max = 256 } = {}) {
  const entries = new Map();

  function get(id8) {
    if (!entries.has(id8)) return null;
    const record = entries.get(id8);
    entries.delete(id8);
    entries.set(id8, record);
    return snapshot(record);
  }

  function peek(id8) {
    return entries.has(id8) ? entries.get(id8) : null;
  }

  function set(id8, record) {
    if (entries.has(id8)) entries.delete(id8);
    entries.set(id8, snapshot(record));
    while (entries.size > max) {
      const oldest = entries.keys().next().value;
      entries.delete(oldest);
    }
  }

  function remove(id8) {
    return entries.delete(id8);
  }

  function clear() {
    entries.clear();
  }

  function has(id8) {
    return entries.has(id8);
  }

  return {
    get,
    peek,
    set,
    delete: remove,
    clear,
    has,
    get size() {
      return entries.size;
    },
  };
}
