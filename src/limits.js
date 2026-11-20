'use strict';

/**
 * Upload limits for the batch import endpoint.
 *
 * The values here are part of the public contract; see docs/csv-intake.md §6.
 */

const LIMITS = Object.freeze({
  maxBodyBytes: 8 * 1024 * 1024,
  maxCellBytes: 32 * 1024,
  maxFieldsPerRecord: 64,
  maxRecords: 50000,
});

/**
 * Validate one upload against the service limits.
 *
 * @param {{
 *   bytes?: number,
 *   header?: string[] | null,
 *   fields?: string[] | null,
 *   rowNumber?: number,
 *   recordCount?: number,
 * }} submission
 * @returns {null | { code: string, message: string, row: number, column: number }}
 */
function check(submission = {}) {
  const bytes = typeof submission.bytes === 'number' ? submission.bytes : 0;

  if (bytes > LIMITS.maxBodyBytes) {
    return {
      code: 'fatal_request_too_large',
      message: `request body is ${bytes} bytes; the limit is ${LIMITS.maxBodyBytes}`,
      row: 0,
      column: 0,
    };
  }

  return null;
}

module.exports = {
  LIMITS,
  check,
};
