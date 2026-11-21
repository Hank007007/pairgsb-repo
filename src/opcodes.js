"use strict";

const OPCODES = Object.freeze({
  CONTINUATION: 0x0,
  TEXT: 0x1,
  BINARY: 0x2,
  CLOSE: 0x8,
  PING: 0x9,
  PONG: 0xa,
});

const CONTROL_OPCODES = new Set([OPCODES.CLOSE, OPCODES.PING, OPCODES.PONG]);
const DATA_OPCODES = new Set([OPCODES.CONTINUATION, OPCODES.TEXT, OPCODES.BINARY]);
const KNOWN_OPCODES = new Set([...CONTROL_OPCODES, ...DATA_OPCODES]);

function isControlOpcode(opcode) {
  return CONTROL_OPCODES.has(opcode);
}

function isDataOpcode(opcode) {
  return DATA_OPCODES.has(opcode);
}

function isKnownOpcode(opcode) {
  return KNOWN_OPCODES.has(opcode);
}

module.exports = {
  OPCODES,
  CONTROL_OPCODES,
  DATA_OPCODES,
  isControlOpcode,
  isDataOpcode,
  isKnownOpcode,
};
