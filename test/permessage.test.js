"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const zlib = require("node:zlib");

const {
  EXTENSION_NAME,
  TRAILER,
  parseExtensionHeader,
  negotiate,
  extensionResponse,
  inflateMessage,
} = require("../src/permessage");

/** Compress a payload the way a permessage-deflate sender would. */
function compress(payload) {
  const deflated = zlib.deflateRawSync(payload, {
    finishFlush: zlib.constants.Z_SYNC_FLUSH,
  });
  return deflated.subarray(0, deflated.length - 4);
}

test("parseExtensionHeader reads a single offer", () => {
  assert.deepEqual(parseExtensionHeader("permessage-deflate"), [
    { name: "permessage-deflate", params: {} },
  ]);
});

test("parseExtensionHeader reads parameters", () => {
  const parsed = parseExtensionHeader(
    "permessage-deflate; client_max_window_bits=15; server_no_context_takeover",
  );

  assert.equal(parsed[0].params.client_max_window_bits, "15");
  assert.equal(parsed[0].params.server_no_context_takeover, true);
});

test("parseExtensionHeader reads several offers", () => {
  const parsed = parseExtensionHeader("permessage-deflate, x-custom; mode=fast");

  assert.equal(parsed.length, 2);
  assert.equal(parsed[1].name, "x-custom");
});

test("parseExtensionHeader handles an empty value", () => {
  assert.deepEqual(parseExtensionHeader(""), []);
  assert.deepEqual(parseExtensionHeader(undefined), []);
});

test("negotiate accepts the permessage-deflate offer", () => {
  assert.deepEqual(negotiate("permessage-deflate"), { name: EXTENSION_NAME });
});

test("negotiate ignores offers without the extension", () => {
  assert.equal(negotiate("x-custom"), null);
  assert.equal(negotiate(""), null);
});

test("extensionResponse renders the accepted extension", () => {
  assert.equal(extensionResponse(negotiate("permessage-deflate")), EXTENSION_NAME);
  assert.equal(extensionResponse(null), "");
});

test("inflateMessage expands a compressed payload", () => {
  const text = "the quick brown fox jumps over the lazy dog";
  const wire = compress(Buffer.from(text, "utf8"));

  const out = inflateMessage(wire);

  assert.equal(out.toString("utf8"), text);
});

test("inflateMessage expands a larger payload", () => {
  const text = "frameseal gateway ".repeat(500);
  const wire = compress(Buffer.from(text, "utf8"));

  const out = inflateMessage(wire);

  assert.equal(out.length, Buffer.byteLength(text));
  assert.equal(out.toString("utf8"), text);
});

test("inflateMessage round-trips an empty message", () => {
  const wire = compress(Buffer.alloc(0));

  assert.deepEqual(inflateMessage(wire), Buffer.alloc(0));
});

test("TRAILER is the sync-flush marker", () => {
  assert.deepEqual(TRAILER, Buffer.from([0x00, 0x00, 0xff, 0xff]));
});
