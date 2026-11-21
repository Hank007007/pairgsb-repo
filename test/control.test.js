"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  CLOSE_NO_STATUS,
  validateControlFrame,
  parseClose,
  encodeClose,
  handlePing,
} = require("../src/control");
const { parseFrame } = require("../src/frame");
const { OPCODES } = require("../src/opcodes");
const { clientFrame } = require("../testlib/frames");

function frame(options) {
  return parseFrame(clientFrame(options)).frame;
}

test("validateControlFrame accepts a small ping", () => {
  const ping = frame({ opcode: OPCODES.PING, payload: "keepalive" });

  assert.doesNotThrow(() => validateControlFrame(ping));
});

test("validateControlFrame accepts a control frame of exactly 125 bytes", () => {
  const ping = frame({ opcode: OPCODES.PING, payload: Buffer.alloc(125, 0x61) });

  assert.doesNotThrow(() => validateControlFrame(ping));
});

test("parseClose reports the normal closure code and reason", () => {
  const payload = Buffer.alloc(2 + 3);
  payload.writeUInt16BE(1000, 0);
  payload.write("bye", 2);

  assert.deepEqual(parseClose(payload), { code: 1000, reason: "bye" });
});

test("parseClose treats an empty payload as no status code", () => {
  assert.deepEqual(parseClose(Buffer.alloc(0)), {
    code: CLOSE_NO_STATUS,
    reason: "",
  });
});

test("parseClose reads a two-byte payload as code only", () => {
  const payload = Buffer.alloc(2);
  payload.writeUInt16BE(1001, 0);

  assert.deepEqual(parseClose(payload), { code: 1001, reason: "" });
});

test("parseClose keeps a multi-byte reason intact", () => {
  const reason = "再见";
  const reasonBytes = Buffer.from(reason, "utf8");
  const payload = Buffer.alloc(2 + reasonBytes.length);
  payload.writeUInt16BE(1000, 0);
  reasonBytes.copy(payload, 2);

  assert.deepEqual(parseClose(payload), { code: 1000, reason });
});

test("encodeClose writes the code and reason", () => {
  const { opcode, payload } = encodeClose(1000, "done");

  assert.equal(opcode, OPCODES.CLOSE);
  assert.equal(payload.readUInt16BE(0), 1000);
  assert.equal(payload.subarray(2).toString("utf8"), "done");
});

test("encodeClose defaults to a normal closure", () => {
  const { payload } = encodeClose();

  assert.equal(payload.length, 2);
  assert.equal(payload.readUInt16BE(0), 1000);
});

test("handlePing echoes the payload", () => {
  const ping = frame({ opcode: OPCODES.PING, payload: "are you there" });
  const result = handlePing(ping);

  assert.equal(result.type, "ping");
  assert.equal(result.payload.toString("utf8"), "are you there");
  assert.notEqual(result.payload, ping.payload);
});
