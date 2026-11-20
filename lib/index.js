"use strict";

const { sanitize } = require("./sanitize");
const render = require("./render");
const { parseFragment, walk } = require("./tree");
const { toHtml } = require("./serialize");

module.exports = {
  sanitize,
  render,
  parseFragment,
  walk,
  toHtml,
};
