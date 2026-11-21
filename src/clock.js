'use strict';

/**
 * Attach shipping metadata to a record.
 */
function stamp(record) {
  return {
    ...record,
    ts: new Date().toISOString(),
  };
}

function stampAll(records) {
  return records.map(stamp);
}

module.exports = {
  stamp,
  stampAll,
};
