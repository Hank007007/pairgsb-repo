"use strict";

const { tokenize } = require("./tokenizer");

const VOID_ELEMENTS = new Set(["br", "img"]);

/**
 * Builds a simple tree from an HTML fragment.
 *
 * Node shapes:
 *   { type: "element", name, attrs: [{ name, value }], children: [] }
 *   { type: "text", value }
 *
 * @param {string} html
 * @returns {Array<object>} top-level nodes
 */
function parseFragment(html) {
  const tokens = tokenize(html);
  const roots = [];
  const stack = [];

  const currentChildren = () => {
    if (stack.length === 0) return roots;
    return stack[stack.length - 1].children;
  };

  for (const token of tokens) {
    if (token.type === "text") {
      currentChildren().push({ type: "text", value: token.value });
      continue;
    }

    if (token.type === "startTag") {
      const node = {
        type: "element",
        name: token.name,
        attrs: token.attrs.map((attr) => ({ name: attr.name, value: attr.value })),
        children: [],
      };
      currentChildren().push(node);
      if (!VOID_ELEMENTS.has(token.name) && !token.selfClosing) {
        stack.push(node);
      }
      continue;
    }

    if (token.type === "endTag") {
      const index = findOpenElement(stack, token.name);
      if (index !== -1) {
        stack.length = index;
      }
    }
  }

  return roots;
}

function findOpenElement(stack, name) {
  for (let i = stack.length - 1; i >= 0; i -= 1) {
    if (stack[i].name === name) return i;
  }
  return -1;
}

/**
 * Walks a node and its descendants, letting a visitor decide what happens to
 * each element.
 *
 * The visitor may provide:
 *   enter(node) -> "keep" | "drop" | "unwrap" | undefined
 *   exit(node)  -> void
 *
 * "drop" removes the element and its descendants. "unwrap" removes the element
 * but keeps its sanitized children in its place. Text nodes are passed through
 * unchanged.
 *
 * @param {object} node
 * @param {{enter?: Function, exit?: Function}} visitor
 * @returns {Array<object>} replacement nodes
 */
function walk(node, visitor) {
  if (!node || node.type === "text") {
    return [node];
  }

  if (node.name === "template" || node.name === "noscript") {
    return [node];
  }

  const decision = visitor.enter ? visitor.enter(node) : undefined;
  if (decision === "drop") return [];

  const children = [];
  for (const child of node.children) {
    children.push(...walk(child, visitor));
  }

  if (decision === "unwrap") return children;

  node.children = children;
  if (visitor.exit) visitor.exit(node);
  return [node];
}

module.exports = {
  VOID_ELEMENTS,
  parseFragment,
  walk,
};
