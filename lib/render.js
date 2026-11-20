"use strict";

const { sanitize } = require("./sanitize");

/**
 * Wraps occurrences of a query in <mark> elements.
 *
 * @param {string} fragment sanitized HTML fragment
 * @param {string} query text to highlight
 * @returns {string}
 */
function withHighlight(fragment, query) {
  const html = String(fragment == null ? "" : fragment);
  if (typeof query !== "string" || query === "") return html;

  const pattern = new RegExp(escapeRegExp(query), "gi");
  return html.replace(pattern, (match) => "<mark>" + match + "</mark>");
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Sanitizes a fragment and applies highlighting in one step.
 *
 * @param {string} html
 * @param {object} [opts]
 * @param {string} [opts.highlight]
 * @returns {string}
 */
function render(html, opts) {
  const options = opts || {};
  const safe = sanitize(html, options);
  return withHighlight(safe, options.highlight);
}

module.exports = {
  render,
  withHighlight,
};
