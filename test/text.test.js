"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { decodeText } = require("../src/text");

test("decodeText returns ASCII text unchanged", () => {
  assert.equal(decodeText(Buffer.from("frameseal", "utf8")), "frameseal");
});

test("decodeText handles multi-byte characters", () => {
  const text = "网关 — gateway";

  assert.equal(decodeText(Buffer.from(text, "utf8")), text);
});

test("decodeText handles characters outside the BMP", () => {
  const text = "emoji \u{1f6a7} and \u{1f510}";

  assert.equal(decodeText(Buffer.from(text, "utf8")), text);
});

test("decodeText handles an empty payload", () => {
  assert.equal(decodeText(Buffer.alloc(0)), "");
});

test("decodeText rejects a non-buffer argument", () => {
  assert.throws(() => decodeText("string"), TypeError);
});
