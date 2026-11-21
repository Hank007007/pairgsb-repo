"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { parseFrame, readExtendedLength } = require("../src/frame");
const { ProtocolError } = require("../src/errors");
const { OPCODES } = require("../src/opcodes");
const { buildFrame, clientFrame } = require("../testlib/frames");

test("readExtendedLength decodes the two-byte form", () => {
  const header = Buffer.from([0x00, 0x7e, 0x01, 0x2c]);
  const result = readExtendedLength(header, 2, 126);

  assert.deepEqual(result, { length: 300, bytes: 2 });
});

test("readExtendedLength decodes the eight-byte form", () => {
  const header = Buffer.alloc(10);
  header[0] = 0x00;
  header[1] = 0x7f;
  header.writeBigUInt64BE(70000n, 2);

  const result = readExtendedLength(header, 2, 127);

  assert.deepEqual(result, { length: 70000, bytes: 8 });
});

test("readExtendedLength waits for the whole extended field", () => {
  assert.equal(readExtendedLength(Buffer.from([0x00, 0x7e, 0x01]), 2, 126), null);
  assert.equal(readExtendedLength(Buffer.alloc(6, 0x00), 2, 127), null);
});

test("readExtendedLength uses the length code for short payloads", () => {
  assert.deepEqual(readExtendedLength(Buffer.alloc(2), 2, 12), {
    length: 12,
    bytes: 0,
  });
});

test("parseFrame decodes a small masked text frame", () => {
  const wire = clientFrame({ payload: "hello" });
  const decoded = parseFrame(wire);

  assert.notEqual(decoded, null);
  assert.equal(decoded.bytesRead, wire.length);
  assert.equal(decoded.frame.fin, true);
  assert.equal(decoded.frame.opcode, OPCODES.TEXT);
  assert.equal(decoded.frame.masked, true);
  assert.equal(decoded.frame.payload.toString("utf8"), "hello");
});

test("parseFrame decodes a payload using the two-byte extended length", () => {
  const text = "a".repeat(300);
  const wire = clientFrame({ payload: text });
  const decoded = parseFrame(wire);

  assert.equal(decoded.frame.length, 300);
  assert.equal(decoded.frame.payload.toString("utf8"), text);
  assert.equal(decoded.bytesRead, wire.length);
});

test("parseFrame decodes a payload using the eight-byte extended length", () => {
  const text = "b".repeat(70000);
  const wire = clientFrame({ payload: text });
  const decoded = parseFrame(wire);

  assert.equal(decoded.frame.length, 70000);
  assert.equal(decoded.frame.payload.length, 70000);
  assert.equal(decoded.frame.payload.toString("utf8"), text);
  assert.equal(decoded.bytesRead, wire.length);
});

test("parseFrame returns null until the fixed header is present", () => {
  assert.equal(parseFrame(Buffer.alloc(0)), null);
  assert.equal(parseFrame(Buffer.from([0x81])), null);
});

test("parseFrame returns null until the payload is complete", () => {
  const wire = clientFrame({ payload: "complete me" });

  assert.equal(parseFrame(wire.subarray(0, wire.length - 1)), null);
  assert.equal(parseFrame(wire.subarray(0, 4)), null);
});

test("parseFrame reports how many bytes the frame occupies", () => {
  const first = clientFrame({ payload: "one" });
  const second = clientFrame({ payload: "two" });
  const stream = Buffer.concat([first, second]);

  const decoded = parseFrame(stream);

  assert.equal(decoded.bytesRead, first.length);
  assert.equal(decoded.frame.payload.toString("utf8"), "one");
});

test("parseFrame rejects a reserved opcode", () => {
  const wire = buildFrame({ opcode: 0x3, payload: "x" });

  assert.throws(
    () => parseFrame(wire),
    (error) => error instanceof ProtocolError && error.closeCode === 1002,
  );
});

test("parseFrame records the frame flags", () => {
  const wire = clientFrame({ fin: false, opcode: OPCODES.BINARY, payload: "abc" });
  const decoded = parseFrame(wire);

  assert.equal(decoded.frame.fin, false);
  assert.equal(decoded.frame.opcode, OPCODES.BINARY);
});
