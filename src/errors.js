/**
 * Error codes reported through the `errors` array of `load()` and
 * `applyPatch()`.
 */
export const CODES = Object.freeze({
  RESERVED_KEY: 'E_RESERVED_KEY',
  INVALID_KEY: 'E_INVALID_KEY',
  TYPE: 'E_TYPE',
  PATCH_OP: 'E_PATCH_OP',
  PATCH_PATH: 'E_PATCH_PATH',
  PATCH_NOT_FOUND: 'E_PATCH_NOT_FOUND',
  PATCH_TEST: 'E_PATCH_TEST',
  PATCH_TYPE: 'E_PATCH_TYPE',
});

/**
 * Build one entry of the `errors` array.
 *
 * @param {string} code one of CODES
 * @param {string} path dotted path or JSON Pointer identifying the input
 * @param {string} message human readable description
 */
export function makeError(code, path, message) {
  return { code, path, message };
}
