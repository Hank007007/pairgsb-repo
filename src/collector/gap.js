'use strict';

const { streamKey } = require('../protocol/manifest');

function emptyStream(origin, epoch) {
  return {
    origin,
    epoch,
    watermark: -1,
    pending: new Set(),
  };
}

/**
 * Missing ranges for a stream: the sequence numbers between the contiguous
 * watermark and the highest stored record that have not been stored yet.
 * Ranges are maximal and inclusive on both ends.
 */
function gapRanges(stream) {
  if (stream.pending.size === 0) return [];
  const stored = Array.from(stream.pending).sort((a, b) => a - b);
  const ranges = [];
  let from = stream.watermark + 1;
  for (const seq of stored) {
    if (seq > from) ranges.push({ from, to: seq - 1 });
    from = seq + 1;
  }
  return ranges;
}

/**
 * Tracks, per (origin, epoch), the contiguous accepted watermark and the
 * records stored beyond it, so missing regions can be reported as ranges and
 * cleared as they are filled.
 */
class GapTracker {
  constructor() {
    this.streams = new Map();
  }

  streamFor(origin, epoch) {
    const key = streamKey(origin, epoch);
    let stream = this.streams.get(key);
    if (!stream) {
      stream = emptyStream(origin, epoch);
      this.streams.set(key, stream);
    }
    return stream;
  }

  /**
   * Feeds a newly stored sequence number to the tracker. Returns a summary of
   * what the tracker now believes about the stream.
   */
  observe(origin, epoch, seq) {
    const stream = this.streamFor(origin, epoch);

    if (seq <= stream.watermark || stream.pending.has(seq)) {
      return this.snapshot(stream, { accepted: false, reason: 'out_of_order' });
    }

    if (seq === stream.watermark + 1) {
      stream.watermark = seq;
      while (stream.pending.delete(stream.watermark + 1)) {
        stream.watermark += 1;
      }
    } else {
      stream.pending.add(seq);
    }

    return this.snapshot(stream, { accepted: true });
  }

  snapshot(stream, extra = {}) {
    return {
      origin: stream.origin,
      epoch: stream.epoch,
      watermark: stream.watermark,
      gaps: gapRanges(stream),
      ...extra,
    };
  }

  watermarkFor(origin, epoch) {
    const key = streamKey(origin, epoch);
    const stream = this.streams.get(key);
    return stream ? stream.watermark : -1;
  }

  gapsFor(origin, epoch) {
    const key = streamKey(origin, epoch);
    const stream = this.streams.get(key);
    return stream ? gapRanges(stream) : [];
  }

  /**
   * A human-readable, actionable description of what remains missing.
   */
  describe(origin, epoch) {
    const ranges = this.gapsFor(origin, epoch);
    if (ranges.length === 0) return `origin ${origin} has no gaps`;
    const missing = ranges.map((range) => `${range.from}..${range.to}`).join(', ');
    return `origin ${origin} missing seq ${missing}`;
  }

  report() {
    return Array.from(this.streams.values()).map((stream) => {
      const gaps = gapRanges(stream);
      return {
        origin: stream.origin,
        epoch: stream.epoch,
        watermark: stream.watermark,
        hasGap: gaps.length > 0,
        gaps,
      };
    });
  }
}

module.exports = {
  GapTracker,
  emptyStream,
  gapRanges,
};
