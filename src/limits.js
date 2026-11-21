"use strict";

/**
 * Resource ceilings used by the gateway front end. The numbers are fixed by
 * docs/frame-rules.md §9 and are part of the wire contract with the gateway
 * operators.
 */
const LIMITS = Object.freeze({
  maxFramePayload: 1024 * 1024,
  maxMessagePayload: 4 * 1024 * 1024,
  maxMessageFrames: 1024,
  maxInflatedPayload: 4 * 1024 * 1024,
  maxControlPayload: 125,
});

module.exports = { LIMITS };
