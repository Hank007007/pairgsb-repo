"use strict";

const NAMED_REFERENCES = {
  amp: "&",
  lt: "<",
  gt: ">",
  quot: '"',
  apos: "'",
};

/**
 * Decodes character references in a string.
 *
 * @param {string} input
 * @returns {string}
 */
function decodeEntities(input) {
  return String(input).replace(/&(amp|lt|gt|quot|apos);/g, (match, name) => {
    return NAMED_REFERENCES[name];
  });
}

const TAG_NAME_START = /[A-Za-z]/;
const RAW_TEXT_ELEMENTS = new Set([
  "script",
  "style",
  "xmp",
  "iframe",
  "noembed",
  "noframes",
  "textarea",
  "title",
]);

function isAsciiWhitespace(ch) {
  return ch === " " || ch === "\t" || ch === "\n" || ch === "\r" || ch === "\f";
}

/**
 * Tokenizes an HTML fragment into start tags, end tags, and text.
 *
 * Token shapes:
 *   { type: "startTag", name, attrs: [{ name, value }], selfClosing }
 *   { type: "endTag", name }
 *   { type: "text", value }
 *
 * Attribute values are kept exactly as they appear between the quotes (or as
 * the unquoted run). Character references are decoded later, at the point where
 * the value is consumed.
 *
 * @param {string} html
 * @returns {Array<object>}
 */
function tokenize(html) {
  const input = String(html == null ? "" : html);
  const tokens = [];
  let index = 0;

  while (index < input.length) {
    const lt = input.indexOf("<", index);
    if (lt === -1) {
      tokens.push({ type: "text", value: input.slice(index) });
      break;
    }
    if (lt > index) {
      tokens.push({ type: "text", value: input.slice(index, lt) });
      index = lt;
    }

    if (input.startsWith("<!--", index)) {
      const end = input.indexOf("-->", index + 4);
      index = end === -1 ? input.length : end + 3;
      continue;
    }

    if (input.startsWith("<!", index) || input.startsWith("<?", index)) {
      const end = input.indexOf(">", index + 2);
      index = end === -1 ? input.length : end + 1;
      continue;
    }

    if (input.startsWith("</", index)) {
      const end = input.indexOf(">", index + 2);
      if (end === -1) {
        tokens.push({ type: "text", value: input.slice(index) });
        break;
      }
      const rawName = input.slice(index + 2, end).trim();
      const name = rawName.split(/[\t\n\f\r ]/)[0].toLowerCase();
      tokens.push({ type: "endTag", name });
      index = end + 1;
      continue;
    }

    if (!TAG_NAME_START.test(input[index + 1] || "")) {
      tokens.push({ type: "text", value: "<" });
      index += 1;
      continue;
    }

    const tag = readStartTag(input, index);
    tokens.push(tag.token);
    index = tag.next;

    if (RAW_TEXT_ELEMENTS.has(tag.token.name) && !tag.token.selfClosing) {
      const close = findRawTextClose(input, index, tag.token.name);
      if (close.index > index) {
        tokens.push({ type: "text", value: input.slice(index, close.index) });
      }
      index = close.index;
    }
  }

  return tokens;
}

function readStartTag(input, start) {
  let index = start + 1;
  let name = "";
  while (index < input.length) {
    const ch = input[index];
    if (isAsciiWhitespace(ch) || ch === "/" || ch === ">") break;
    name += ch;
    index += 1;
  }

  const attrs = [];
  let selfClosing = false;

  while (index < input.length) {
    while (index < input.length && isAsciiWhitespace(input[index])) index += 1;
    if (index >= input.length) break;

    if (input[index] === ">") {
      index += 1;
      break;
    }
    if (input[index] === "/") {
      selfClosing = true;
      index += 1;
      continue;
    }

    let attrName = "";
    while (index < input.length) {
      const ch = input[index];
      if (ch === "=" || ch === ">" || ch === "/") break;
      attrName += ch;
      index += 1;
    }

    if (attrName === "") {
      index += 1;
      continue;
    }

    while (index < input.length && isAsciiWhitespace(input[index])) index += 1;

    let value = "";
    if (input[index] === "=") {
      index += 1;
      while (index < input.length && isAsciiWhitespace(input[index])) index += 1;
      const quote = input[index];
      if (quote === '"' || quote === "'") {
        index += 1;
        const end = input.indexOf(quote, index);
        if (end === -1) {
          value = input.slice(index);
          index = input.length;
        } else {
          value = input.slice(index, end);
          index = end + 1;
        }
      } else {
        const valueStart = index;
        while (index < input.length && !isAsciiWhitespace(input[index]) && input[index] !== ">") {
          index += 1;
        }
        value = input.slice(valueStart, index);
      }
    }

    attrs.push({ name: attrName, value });
  }

  return {
    token: {
      type: "startTag",
      name: name.toLowerCase(),
      attrs,
      selfClosing,
    },
    next: index,
  };
}

function findRawTextClose(input, from, name) {
  const lower = input.toLowerCase();
  const needle = "</" + name;
  let index = lower.indexOf(needle, from);
  while (index !== -1) {
    const after = input[index + needle.length] || "";
    if (after === "" || after === ">" || after === "/" || isAsciiWhitespace(after)) {
      return { index };
    }
    index = lower.indexOf(needle, index + 1);
  }
  return { index: input.length };
}

module.exports = {
  decodeEntities,
  tokenize,
};
