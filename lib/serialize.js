"use strict";

const { VOID_ELEMENTS } = require("./tree");

/**
 * Serializes a node or an array of nodes to an HTML fragment.
 *
 * @param {object|Array<object>} nodes
 * @returns {string}
 */
function toHtml(nodes) {
  const list = Array.isArray(nodes) ? nodes : [nodes];
  let output = "";
  for (const node of list) {
    output += serializeNode(node);
  }
  return output;
}

function serializeNode(node) {
  if (!node) return "";
  if (node.type === "text") return escapeText(node.value);

  let output = "<" + node.name;
  for (const attr of node.attrs) {
    output += " " + attr.name + '="' + escapeAttribute(attr.value) + '"';
  }
  output += ">";

  if (VOID_ELEMENTS.has(node.name)) return output;

  for (const child of node.children) {
    output += serializeNode(child);
  }
  output += "</" + node.name + ">";
  return output;
}

function escapeText(value) {
  return String(value).replace(/&/g, "&amp;").replace(/</g, "&lt;");
}

function escapeAttribute(value) {
  return String(value).replace(/&/g, "&amp;").replace(/</g, "&lt;");
}

module.exports = {
  toHtml,
  escapeText,
  escapeAttribute,
};
