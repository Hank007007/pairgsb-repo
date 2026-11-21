"use strict";

const { parseFrame } = require("./frame");
const { MessageAssembler } = require("./fragment");
const {
  validateControlFrame,
  parseClose,
  handlePing,
  handlePong,
} = require("./control");
const { negotiate, inflateMessage } = require("./permessage");
const { decodeText } = require("./text");
const { ProtocolError } = require("./errors");
const { OPCODES, isControlOpcode } = require("./opcodes");

const STATES = Object.freeze({
  OPEN: "open",
  CLOSING: "closing",
  CLOSED: "closed",
});

/**
 * Front-end connection for one accepted client.
 *
 * The connection owns the receive buffer, the frame parser and the message
 * assembler. Callers push raw bytes in with `feed`; decoded messages and
 * control frames are reported through the callbacks passed to the
 * constructor:
 *
 *   onMessage({ type: "text" | "binary", data, compressed })
 *   onPing({ type: "ping", payload })
 *   onPong({ type: "pong", payload })
 *   onClose({ code, reason, byPeer, error })
 *
 * State transitions:
 *
 *   open -> closing -> closed
 *
 * `feed` is only accepted while the connection is open. A protocol violation
 * moves the connection through `closing` to `closed` and is reported to the
 * caller as a thrown error.
 */
class Connection {
  constructor(options = {}) {
    this.state = STATES.OPEN;
    this.buffer = Buffer.alloc(0);
    this.compression = negotiate(options.extensionsHeader ?? "");
    this.assembler = new MessageAssembler();

    this.onMessage = options.onMessage ?? (() => {});
    this.onPing = options.onPing ?? (() => {});
    this.onPong = options.onPong ?? (() => {});
    this.onClose = options.onClose ?? (() => {});

    this.closeCode = null;
    this.closeReason = "";
  }

  get isOpen() {
    return this.state === STATES.OPEN;
  }

  /**
   * Push bytes received from the peer into the decoder.
   */
  feed(chunk) {
    if (this.state !== STATES.OPEN) {
      throw new Error(`connection is ${this.state}, not open`);
    }
    if (!Buffer.isBuffer(chunk)) {
      chunk = Buffer.from(chunk);
    }

    this.buffer =
      this.buffer.length === 0
        ? chunk
        : Buffer.concat([this.buffer, chunk]);

    while (this.state === STATES.OPEN) {
      let decoded;
      try {
        decoded = parseFrame(this.buffer);
      } catch (error) {
        this._fail(error);
        throw error;
      }

      if (decoded === null) {
        return;
      }

      this.buffer = this.buffer.subarray(decoded.bytesRead);

      try {
        this._handleFrame(decoded.frame);
      } catch (error) {
        this._fail(error);
        throw error;
      }
    }
  }

  /**
   * Close the connection from the gateway side. The peer is not contacted by
   * this class; the caller is responsible for writing the close frame.
   */
  close(code = 1000, reason = "") {
    if (this.state === STATES.OPEN) {
      this.closeCode = code;
      this.closeReason = reason;
      this.state = STATES.CLOSING;
    }
    if (this.state === STATES.CLOSING) {
      this.state = STATES.CLOSED;
      this.onClose({
        code: this.closeCode,
        reason: this.closeReason,
        byPeer: false,
        error: false,
      });
    }
  }

  /**
   * Release anything the assembler still holds. Used when the connection ends
   * and buffered content has to be accounted for.
   */
  end() {
    const message = this.assembler.flush();
    if (message !== null) {
      this._deliver(message);
    }
  }

  _handleFrame(frame) {
    const message = this.assembler.append(frame);

    if (isControlOpcode(frame.opcode)) {
      this._handleControl(frame);
    }

    if (message !== null) {
      this._deliver(message);
    }
  }

  _handleControl(frame) {
    validateControlFrame(frame);

    if (frame.opcode === OPCODES.CLOSE) {
      const close = parseClose(frame.payload);
      this.closeCode = close.code;
      this.closeReason = close.reason;
      this._enterClosing();
      this.state = STATES.CLOSED;
      this.onClose({
        code: close.code,
        reason: close.reason,
        byPeer: true,
        error: false,
      });
      return;
    }

    if (frame.opcode === OPCODES.PING) {
      this.onPing(handlePing(frame));
      return;
    }

    if (frame.opcode === OPCODES.PONG) {
      this.onPong(handlePong(frame));
    }
  }

  _deliver(message) {
    let payload = message.payload;

    if (this.compression !== null) {
      payload = inflateMessage(payload);
    }

    if (message.opcode === OPCODES.TEXT) {
      this.onMessage({
        type: "text",
        data: decodeText(payload),
        compressed: message.compressed,
      });
      return;
    }

    this.onMessage({
      type: "binary",
      data: payload,
      compressed: message.compressed,
    });
  }

  _enterClosing() {
    if (this.state === STATES.OPEN) {
      this.state = STATES.CLOSING;
    }
    if (this.state === STATES.CLOSING) {
      this.state = STATES.CLOSED;
    }
  }

  _fail(error) {
    const closeCode =
      error instanceof ProtocolError ? error.closeCode : 1002;

    this.closeCode = closeCode;
    this.closeReason = error.message;

    this._enterClosing();

    this.onClose({
      code: closeCode,
      reason: error.message,
      byPeer: false,
      error: true,
    });
  }
}

module.exports = {
  Connection,
  STATES,
};
