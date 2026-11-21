"use strict";

class ProtocolError extends Error {
  constructor(closeCode, message) {
    super(message);
    this.name = "ProtocolError";
    this.closeCode = closeCode;
  }
}

class InvalidPayloadError extends ProtocolError {
  constructor(message) {
    super(1007, message);
    this.name = "InvalidPayloadError";
  }
}

class MessageTooLargeError extends ProtocolError {
  constructor(message) {
    super(1009, message);
    this.name = "MessageTooLargeError";
  }
}

module.exports = {
  ProtocolError,
  InvalidPayloadError,
  MessageTooLargeError,
};
