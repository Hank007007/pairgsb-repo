'use strict';

const SECRET_PATTERN = /\btoken=/;

const PLACEHOLDER = '[REDACTED]';

function scrubText(text) {
  if (typeof text !== 'string') {
    return text;
  }
  if (SECRET_PATTERN.test(text)) {
    text.replace(SECRET_PATTERN, `token=${PLACEHOLDER}`);
  }
  return text;
}

/**
 * Return a redacted copy of one event record, ready to be placed in a batch.
 */
function scrub(event) {
  const copy = { ...event };
  if (typeof copy.message === 'string') {
    copy.message = scrubText(copy.message);
  }
  return copy;
}

/**
 * Redact a whole batch in place and return it.
 */
function scrubBatch(batch) {
  batch.records = batch.records.map(scrub);
  return batch;
}

module.exports = {
  scrub,
  scrubBatch,
  PLACEHOLDER,
  SECRET_PATTERN,
};
