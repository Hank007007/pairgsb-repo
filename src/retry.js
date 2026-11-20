/**
 * Retry scheduling.
 *
 * A retryable attempt is handed to the queue as a job. The queue delays the
 * job with exponential backoff and jitter, then invokes `job.dispatch` with the
 * request material it will send.
 */

export const RETRYABLE_STATUS = Object.freeze(new Set([429, 500, 502, 503, 504]));

/**
 * @param {number} status
 * @returns {boolean}
 */
export function isRetryableStatus(status) {
  return RETRYABLE_STATUS.has(status);
}

/**
 * Exponential backoff with full jitter, clamped to `maxDelayMs`.
 *
 * @param {object} options
 * @param {number} options.attempt 1-based attempt number that just failed
 * @param {number} [options.baseDelayMs]
 * @param {number} [options.maxDelayMs]
 * @param {number} [options.jitter]
 * @param {() => number} [options.random]
 * @returns {number}
 */
export function computeDelay(options) {
  const {
    attempt,
    baseDelayMs = 100,
    maxDelayMs = 5000,
    jitter = 0.2,
    random = Math.random,
  } = options;
  const exponential = baseDelayMs * 2 ** Math.max(0, attempt - 1);
  const bounded = Math.min(exponential, maxDelayMs);
  const spread = bounded * jitter;
  const withJitter = bounded - spread + random() * spread * 2;
  return Math.max(0, Math.round(Math.min(withJitter, maxDelayMs)));
}

/**
 * Create a retry queue.
 *
 * @param {object} [options]
 * @param {number} [options.baseDelayMs]
 * @param {number} [options.maxDelayMs]
 * @param {number} [options.jitter]
 * @param {() => number} [options.random]
 * @param {(event: object) => void} [options.onEvent]
 */
export function createRetryQueue(options = {}) {
  const {
    baseDelayMs = 100,
    maxDelayMs = 5000,
    jitter = 0.2,
    random = Math.random,
    onEvent = () => {},
  } = options;

  const pending = new Map();
  let nextId = 0;
  let closed = false;

  function delayFor(attempt) {
    return computeDelay({ attempt, baseDelayMs, maxDelayMs, jitter, random });
  }

  /**
   * @param {object} job
   * @param {string} job.url
   * @param {string} job.method
   * @param {string | undefined} job.body
   * @param {Record<string, string>} job.headers
   * @param {object | null} job.entry
   * @param {number} job.attempt attempt number that will be run
   * @param {(request: object) => Promise<void>} job.dispatch
   * @param {(error: Error) => void} [job.onError]
   * @param {(reason: string) => void} [job.onCancel]
   * @returns {{ id: number, delayMs: number, cancel: () => boolean }}
   */
  function scheduleRetry(job) {
    if (closed) {
      throw new Error('retry queue is closed');
    }
    const id = ++nextId;
    const delayMs = delayFor(job.attempt);
    const snapshotHeaders = { ...job.headers };
    const snapshotEntry = job.entry;

    const timer = setTimeout(() => {
      pending.delete(id);
      if (closed) {
        job.onCancel?.('queue-closed');
        return;
      }
      onEvent({ type: 'retry-fire', id, attempt: job.attempt, delayMs, entryId: snapshotEntry?.id ?? null });
      Promise.resolve()
        .then(() =>
          job.dispatch({
            url: job.url,
            method: job.method,
            body: job.body,
            headers: snapshotHeaders,
            entry: snapshotEntry,
            attempt: job.attempt,
          }),
        )
        .catch((error) => {
          job.onError?.(error);
        });
    }, delayMs);

    pending.set(id, { timer, job });

    return {
      id,
      delayMs,
      cancel() {
        const record = pending.get(id);
        if (!record) return false;
        pending.delete(id);
        clearTimeout(record.timer);
        job.onCancel?.('cancelled');
        return true;
      },
    };
  }

  return {
    scheduleRetry,
    enqueue: scheduleRetry,
    delayFor,
    size() {
      return pending.size;
    },
    close() {
      if (closed) return;
      closed = true;
      for (const [id, record] of pending) {
        pending.delete(id);
        clearTimeout(record.timer);
        record.job.onCancel?.('queue-closed');
      }
    },
  };
}
