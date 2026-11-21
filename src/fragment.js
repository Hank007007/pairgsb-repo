"use strict";

const { OPCODES } = require("./opcodes");

const START_OPCODES = new Set([OPCODES.TEXT, OPCODES.BINARY]);

/**
 * Reassembles data messages.
 *
 * Every decoded frame is offered to `append`, which hands back a message
 * object `{ opcode, payload, compressed }` for the application layer. Content
 * that has not been handed back is drained with `takeMessage` or `flush`.
 */
class MessageAssembler {
  constructor() {
    this._reset();
  }

  _reset() {
    this.opcode = null;
    this.compressed = false;
    this.fragments = [];
  }

  /** True while the assembler is holding content. */
  get pending() {
    return this.fragments.length > 0;
  }

  /**
   * Offer one frame. Returns a message ready for the application layer, or
   * `null` when the frame does not produce one.
   */
  append(frame) {
    if (this.opcode === null) {
      this.opcode = frame.opcode;
      this.compressed = frame.rsv1 === true;
      this.fragments = [frame.payload];

      if (START_OPCODES.has(frame.opcode)) {
        return this.takeMessage();
      }
      return null;
    }

    this.fragments.push(frame.payload);
    return null;
  }

  /** Remove and return the collected message, or `null` when there is none. */
  takeMessage() {
    if (this.opcode === null) {
      return null;
    }

    const message = {
      opcode: this.opcode,
      payload: Buffer.concat(this.fragments),
      compressed: this.compressed,
    };

    this._reset();
    return message;
  }

  /**
   * Drain whatever the assembler is holding. The connection calls this while
   * it tears down so buffered content is not silently dropped.
   */
  flush() {
    return this.takeMessage();
  }
}

module.exports = { MessageAssembler };
