"use strict";

/**
 * Decides whether a URL value may be emitted in an attribute.
 *
 * @param {string} value attribute value as carried on the node
 * @returns {boolean}
 */
function isSafeUrl(value) {
  const candidate = String(value == null ? "" : value).trim().toLowerCase();
  if (candidate === "") return false;
  if (candidate.startsWith("javascript:")) return false;
  return true;
}

module.exports = {
  isSafeUrl,
};
