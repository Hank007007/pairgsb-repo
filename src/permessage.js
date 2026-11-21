"use strict";

const zlib = require("node:zlib");

const EXTENSION_NAME = "permessage-deflate";
const TRAILER = Buffer.from([0x00, 0x00, 0xff, 0xff]);

/**
 * Split a `Sec-WebSocket-Extensions` value into `{ name, params }` entries.
 */
function parseExtensionHeader(header) {
  if (typeof header !== "string" || header.trim() === "") {
    return [];
  }

  return header
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0)
    .map((entry) => {
      const parts = entry.split(";").map((part) => part.trim());
      const name = parts.shift().toLowerCase();
      const params = {};
      for (const part of parts) {
        const eq = part.indexOf("=");
        if (eq === -1) {
          params[part.toLowerCase()] = true;
        } else {
          const key = part.slice(0, eq).trim().toLowerCase();
          let value = part.slice(eq + 1).trim();
          if (value.startsWith('"') && value.endsWith('"')) {
            value = value.slice(1, -1);
          }
          params[key] = value;
        }
      }
      return { name, params };
    });
}

/**
 * Choose the compression configuration for the connection.
 *
 * Returns `{ name: "permessage-deflate" }` when the client offered the
 * extension and the offer is accepted, or `null` when the connection runs
 * without compression.
 */
function negotiate(header) {
  for (const entry of parseExtensionHeader(header)) {
    if (entry.name === EXTENSION_NAME) {
      return { name: EXTENSION_NAME };
    }
  }
  return null;
}

/** Render the negotiated configuration as a response header value. */
function extensionResponse(config) {
  if (!config) {
    return "";
  }
  return config.name;
}

/**
 * Decode one compressed message payload into its expanded bytes.
 *
 * The sender keeps the trailing sync-flush bytes off the wire, so the
 * decompressor is handed the payload with the `0x00 0x00 0xff 0xff` marker
 * restored.
 */
function inflateMessage(payload) {
  const input = Buffer.concat([payload, TRAILER]);
  return zlib.inflateRawSync(input, {
    finishFlush: zlib.constants.Z_SYNC_FLUSH,
  });
}

module.exports = {
  EXTENSION_NAME,
  TRAILER,
  parseExtensionHeader,
  negotiate,
  extensionResponse,
  inflateMessage,
};
