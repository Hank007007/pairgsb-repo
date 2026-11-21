"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { MessageAssembler } = require("../src/fragment");
const { parseFrame } = require("../src/frame");
const { OPCODES } = require("../src/opcodes");
const { clientFrame } = require("../testlib/frames");

function frame(options) {
  return parseFrame(clientFrame(options)).frame;
}

test("append hands back a single-frame text message", () => {
  const assembler = new MessageAssembler();
  const message = assembler.append(frame({ opcode: OPCODES.TEXT, payload: "hi" }));

  assert.notEqual(message, null);
  assert.equal(message.opcode, OPCODES.TEXT);
  assert.equal(message.payload.toString("utf8"), "hi");
  assert.equal(message.compressed, false);
});

test("append hands back a single-frame binary message", () => {
  const assembler = new MessageAssembler();
  const bytes = Buffer.from([0x00, 0x01, 0xff, 0xfe]);

  const message = assembler.append(
    frame({ opcode: OPCODES.BINARY, payload: bytes }),
  );

  assert.equal(message.opcode, OPCODES.BINARY);
  assert.deepEqual(message.payload, bytes);
});

test("flush returns null when nothing is pending", () => {
  const assembler = new MessageAssembler();

  assert.equal(assembler.pending, false);
  assert.equal(assembler.flush(), null);
});

test("takeMessage returns null when nothing was collected", () => {
  const assembler = new MessageAssembler();

  assert.equal(assembler.takeMessage(), null);
});

test("messages arrive in order", () => {
  const assembler = new MessageAssembler();

  const first = assembler.append(frame({ opcode: OPCODES.TEXT, payload: "first" }));
  const second = assembler.append(frame({ opcode: OPCODES.TEXT, payload: "second" }));

  assert.equal(first.payload.toString("utf8"), "first");
  assert.equal(second.payload.toString("utf8"), "second");
});
