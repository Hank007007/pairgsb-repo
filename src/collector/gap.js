'use strict';

function emptyStream(origin) {
  return {
    origin,
    lastSeq: null,
    watermark: -1,
    hasGap: false,
    gapFrom: null,
    gapTo: null,
  };
}

/**
 * Keeps the highest sequence number observed per origin and the accepted
 * watermark that the collector reports back to the agent.
 */
class GapTracker {
  constructor() {
    this.streams = new Map();
  }

  streamFor(origin) {
    let stream = this.streams.get(origin);
    if (!stream) {
      stream = emptyStream(origin);
      this.streams.set(origin, stream);
    }
    return stream;
  }

  /**
   * Feeds an arrived sequence number to the tracker. Returns a summary of what
   * the tracker now believes about the stream.
   */
  observe(origin, seq) {
    const stream = this.streamFor(origin);

    if (stream.lastSeq !== null && seq <= stream.lastSeq) {
      return this.snapshot(stream, { accepted: false, reason: 'out_of_order' });
    }

    if (stream.lastSeq !== null && seq > stream.lastSeq + 1) {
      stream.hasGap = true;
      if (stream.gapFrom === null) {
        stream.gapFrom = stream.lastSeq + 1;
      }
      stream.gapTo = seq - 1;
    }

    stream.lastSeq = seq;
    if (!stream.hasGap) {
      stream.watermark = seq;
    }

    return this.snapshot(stream, { accepted: true });
  }

  snapshot(stream, extra = {}) {
    return {
      origin: stream.origin,
      lastSeq: stream.lastSeq,
      watermark: stream.watermark,
      hasGap: stream.hasGap,
      ...extra,
    };
  }

  watermarkFor(origin) {
    return this.streamFor(origin).watermark;
  }

  /**
   * A human-readable description of what remains missing for `origin`.
   */
  describe(origin) {
    const stream = this.streamFor(origin);
    if (!stream.hasGap) return `origin ${origin} has no gaps`;
    return `origin ${origin} has a gap after seq ${stream.gapFrom} (watermark ${stream.watermark})`;
  }

  report() {
    return Array.from(this.streams.keys()).map((origin) => {
      const stream = this.streams.get(origin);
      return {
        origin,
        watermark: stream.watermark,
        lastSeq: stream.lastSeq,
        hasGap: stream.hasGap,
      };
    });
  }
}

module.exports = {
  GapTracker,
  emptyStream,
};
