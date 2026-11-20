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

  if (Array.isArray(submission.fields) && submission.fields.length > LIMITS.maxFieldsPerRecord) {
    return {
      code: 'fatal_too_many_fields',
      message:
        `record ${submission.rowNumber} has ${submission.fields.length} fields; ` +
        `the limit is ${LIMITS.maxFieldsPerRecord}`,
      row: submission.rowNumber,
      column: LIMITS.maxFieldsPerRecord + 1,
    };
  }

  if (Array.isArray(submission.fields)) {
    for (let i = 0; i < submission.fields.length; i += 1) {
      const value = submission.fields[i];
      const size =
        typeof value === 'string'
          ? Buffer.byteLength(value, 'utf8')
          : 0;
      if (size > LIMITS.maxCellBytes) {
        return {
          code: 'fatal_cell_too_large',
          message:
            `record ${submission.rowNumber}, column ${i + 1} is ${size} bytes; ` +
            `the limit is ${LIMITS.maxCellBytes}`,
          row: submission.rowNumber,
          column: i + 1,
        };
      }
    }
  }

  if (
    typeof submission.recordCount === 'number' &&
    submission.recordCount > LIMITS.maxRecords
  ) {
    return {
      code: 'fatal_too_many_records',
      message:
        `record ${submission.rowNumber} would make ${submission.recordCount} data records; ` +
        `the limit is ${LIMITS.maxRecords}`,
      row: submission.rowNumber,
      column: 0,
    };
  }

  return null;
}

module.exports = {
  LIMITS,
  check,
};
