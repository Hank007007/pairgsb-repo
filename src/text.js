"use strict";

/**
 * Turn a text message payload into the string handed to the application
 * layer.
 *
 * The function receives the payload of a complete message, after any
 * permessage-deflate decoding has taken place.
 */
function decodeText(payload) {
  if (!Buffer.isBuffer(payload)) {
    throw new TypeError("decodeText expects a Buffer");
  }
  return payload.toString("utf8");
}

module.exports = { decodeText };
