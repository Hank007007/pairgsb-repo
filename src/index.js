"use strict";

const { parseFrame, readExtendedLength } = require("./frame");
const { MessageAssembler } = require("./fragment");
const {
  CLOSE_NO_STATUS,
  validateControlFrame,
  parseClose,
  encodeClose,
  handlePing,
  handlePong,
} = require("./control");
const {
  EXTENSION_NAME,
  negotiate,
  extensionResponse,
  inflateMessage,
} = require("./permessage");
const { decodeText } = require("./text");
const { Connection, STATES } = require("./conn");
const {
  ProtocolError,
  InvalidPayloadError,
  MessageTooLargeError,
} = require("./errors");
const { LIMITS } = require("./limits");
const { OPCODES } = require("./opcodes");

module.exports = {
  parseFrame,
  readExtendedLength,
  MessageAssembler,
  CLOSE_NO_STATUS,
  validateControlFrame,
  parseClose,
  encodeClose,
  handlePing,
  handlePong,
  EXTENSION_NAME,
  negotiate,
  extensionResponse,
  inflateMessage,
  decodeText,
  Connection,
  STATES,
  ProtocolError,
  InvalidPayloadError,
  MessageTooLargeError,
  LIMITS,
  OPCODES,
};
