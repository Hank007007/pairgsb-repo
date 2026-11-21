"use strict";

const { ProtocolError } = require("./errors");
const { LIMITS } = require("./limits");
const { OPCODES, isControlOpcode } = require("./opcodes");

const CLOSE_NO_STATUS = 1005;

/**
 * Reject control frames that violate the structural limits: payloads longer
 * than 125 bytes and fragmented control frames.
 */
function validateControlFrame(frame) {
  if (!isControlOpcode(frame.opcode)) {
    throw new ProtocolError(1002, "opcode is not a control opcode");
  }
  if (!frame.fin) {
    throw new ProtocolError(1002, "control frames must not be fragmented");
  }
  if (frame.payload.length > LIMITS.maxControlPayload) {
    throw new ProtocolError(1002, "control frame payload exceeds 125 bytes");
  }
}

/**
 * Decode a close frame payload into `{ code, reason }`.
 *
 * An empty payload means the peer sent no status code; the connection records
 * 1005 for its own bookkeeping.
 */
function parseClose(payload) {
  if (payload.length === 0) {
    return { code: CLOSE_NO_STATUS, reason: "" };
  }

  const code = payload.length >= 2 ? payload.readUInt16BE(0) : payload[0];
  const reason = payload.subarray(2).toString("utf8");

  return { code, reason };
}

/**
 * Wrap a status code and reason into the payload bytes of a close frame.
 * Used when the gateway originates a close.
 */
function encodeClose(code = 1000, reason = "") {
  const reasonBytes = Buffer.from(reason, "utf8");
  const payload = Buffer.allocUnsafe(2 + reasonBytes.length);
  payload.writeUInt16BE(code, 0);
  reasonBytes.copy(payload, 2);
  return { opcode: OPCODES.CLOSE, payload };
}

/** Interpret a ping; the gateway replies with a pong carrying the same bytes. */
function handlePing(frame) {
  return { type: "ping", payload: Buffer.from(frame.payload) };
}

function handlePong(frame) {
  return { type: "pong", payload: Buffer.from(frame.payload) };
}

module.exports = {
  CLOSE_NO_STATUS,
  validateControlFrame,
  parseClose,
  encodeClose,
  handlePing,
  handlePong,
};
