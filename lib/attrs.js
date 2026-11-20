"use strict";

const { isSafeUrl } = require("./url");

const TAG_WHITELIST = new Set([
  "a",
  "p",
  "strong",
  "em",
  "code",
  "pre",
  "blockquote",
  "ul",
  "ol",
  "li",
  "br",
  "img",
]);

const ATTR_WHITELIST = new Set([
  "href",
  "title",
  "alt",
  "src",
  "width",
  "height",
  "class",
]);

const ATTR_BY_ELEMENT = {
  href: new Set(["a"]),
  src: new Set(["img"]),
  alt: new Set(["img"]),
  width: new Set(["img"]),
  height: new Set(["img"]),
  title: null,
  class: null,
};

const URL_ATTRS = new Set(["href", "src"]);

/**
 * Removes attributes that the policy does not admit.
 *
 * Expects attribute values to be decoded already.
 *
 * @param {string} tagName lowercase element name
 * @param {Array<{name: string, value: string}>} attrs
 * @returns {Array<{name: string, value: string}>}
 */
function filterAttrs(tagName, attrs) {
  const kept = [];
  const seen = new Set();

  for (const attr of attrs || []) {
    const name = attr.name.toLowerCase();
    if (!ATTR_WHITELIST.has(name)) continue;

    const elements = ATTR_BY_ELEMENT[name];
    if (elements && !elements.has(tagName)) continue;

    if (seen.has(name)) continue;
    seen.add(name);

    const value = attr.value;

    if (name === "href" && tagName === "a" && !isSafeUrl(value)) continue;
    if (name === "class" && !isUserClass(value)) continue;

    kept.push({ name, value });
  }

  return kept;
}

function isUserClass(value) {
  return typeof value === "string" && value.startsWith("user-");
}

module.exports = {
  filterAttrs,
  isUserClass,
  TAG_WHITELIST,
  ATTR_WHITELIST,
  ATTR_BY_ELEMENT,
  URL_ATTRS,
};
