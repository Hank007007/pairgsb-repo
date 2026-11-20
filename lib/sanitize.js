"use strict";

const { parseFragment, walk } = require("./tree");
const { filterAttrs, TAG_WHITELIST } = require("./attrs");
const { decodeEntities } = require("./tokenizer");
const { toHtml } = require("./serialize");

const DROP_WITH_CONTENT = new Set([
  "script",
  "style",
  "iframe",
  "object",
  "embed",
  "applet",
  "svg",
  "math",
  "form",
  "input",
  "button",
  "select",
  "textarea",
  "link",
  "meta",
  "base",
  "frame",
  "frameset",
  "noembed",
  "noframes",
  "xmp",
  "plaintext",
]);

/**
 * Parses, sanitizes, and serializes an HTML fragment.
 *
 * @param {string} html
 * @param {object} [opts]
 * @returns {string}
 */
function sanitize(html, opts) {
  const options = opts || {};
  const roots = parseFragment(html);
  decodeTextNodes(roots);

  const sanitized = [];
  for (const root of roots) {
    sanitized.push(...walk(root, createVisitor(options)));
  }

  return toHtml(sanitized);
}

function decodeTextNodes(nodes) {
  for (const node of nodes) {
    if (!node) continue;
    if (node.type === "text") {
      node.value = decodeEntities(node.value);
    } else if (node.children) {
      decodeTextNodes(node.children);
    }
  }
}

function createVisitor(options) {
  return {
    enter(node) {
      if (DROP_WITH_CONTENT.has(node.name)) return "drop";

      if (!TAG_WHITELIST.has(node.name)) return "unwrap";

      for (const attr of node.attrs) {
        attr.value = decodeEntities(attr.value);
      }
      node.attrs = filterAttrs(node.name, node.attrs);
      return "keep";
    },
  };
}

module.exports = {
  sanitize,
  DROP_WITH_CONTENT,
};
