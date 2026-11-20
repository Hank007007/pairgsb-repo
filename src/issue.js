import { digest, generateToken } from './token.js';
import { ERROR_CODES, TokenError } from './errors.js';
import {
  DEFAULT_PURPOSES,
  DEFAULT_TTL_MS,
  MAX_TTL_MS,
  addMilliseconds,
  toMillis,
} from './time.js';

const MAX_SUBJECT_LENGTH = 320;
const MAX_META_BYTES = 4096;

function normalizeContext(ctx) {
  if (ctx === null || typeof ctx !== 'object') {
    throw new TypeError('issue() requires a context object');
  }
  if (ctx.store === null || typeof ctx.store !== 'object') {
    throw new TypeError('context.store is required');
  }
  if (typeof ctx.pepper !== 'string' || ctx.pepper.length === 0) {
    throw new TypeError('context.pepper must be a non-empty string');
  }
  if (typeof ctx.clock !== 'function') {
    throw new TypeError('context.clock must be a function');
  }
  return {
    store: ctx.store,
    pepper: ctx.pepper,
    clock: ctx.clock,
    audit: ctx.audit ?? null,
    purposes: ctx.purposes ?? DEFAULT_PURPOSES,
    defaultTtlMs: ctx.defaultTtlMs ?? DEFAULT_TTL_MS,
    maxTtlMs: ctx.maxTtlMs ?? MAX_TTL_MS,
    idFactory: ctx.idFactory ?? generateId,
  };
}

let sequence = 0;

function generateId() {
  sequence += 1;
  return `tok_${Date.now().toString(36)}_${sequence.toString(36)}`;
}

function validateSubject(subject) {
  if (typeof subject !== 'string' || subject.trim().length === 0) {
    throw new TokenError(
      ERROR_CODES.INVALID_SUBJECT,
      'subject must be a non-empty string',
    );
  }
  const trimmed = subject.trim();
  if (trimmed.length > MAX_SUBJECT_LENGTH) {
    throw new TokenError(
      ERROR_CODES.INVALID_SUBJECT,
      `subject must be at most ${MAX_SUBJECT_LENGTH} characters`,
    );
  }
  return trimmed;
}

function validatePurpose(purpose, allowed) {
  if (typeof purpose !== 'string' || !allowed.includes(purpose)) {
    throw new TokenError(
      ERROR_CODES.INVALID_PURPOSE,
      `purpose must be one of: ${allowed.join(', ')}`,
    );
  }
  return purpose;
}

function validateMeta(meta) {
  if (meta === undefined || meta === null) {
    return {};
  }
  if (typeof meta !== 'object' || Array.isArray(meta)) {
    throw new TokenError(ERROR_CODES.INVALID_META, 'meta must be an object');
  }
  let encoded;
  try {
    encoded = JSON.stringify(meta);
  } catch {
    throw new TokenError(
      ERROR_CODES.INVALID_META,
      'meta must be JSON-serializable',
    );
  }
  if (Buffer.byteLength(encoded, 'utf8') > MAX_META_BYTES) {
    throw new TokenError(
      ERROR_CODES.INVALID_META,
      `meta must be at most ${MAX_META_BYTES} bytes when encoded`,
    );
  }
  return structuredClone(meta);
}

function validateTtl(ttlMs, fallback, max) {
  if (ttlMs === undefined || ttlMs === null) {
    return fallback;
  }
  if (typeof ttlMs !== 'number' || !Number.isFinite(ttlMs) || ttlMs <= 0) {
    throw new TokenError(
      ERROR_CODES.INVALID_TTL,
      'ttlMs must be a positive number of milliseconds',
    );
  }
  if (ttlMs > max) {
    throw new TokenError(
      ERROR_CODES.INVALID_TTL,
      `ttlMs must not exceed ${max} milliseconds`,
    );
  }
  return Math.floor(ttlMs);
}

export async function issue(ctx, input = {}) {
  const context = normalizeContext(ctx);
  const subject = validateSubject(input.subject);
  const purpose = validatePurpose(input.purpose, context.purposes);
  const meta = validateMeta(input.meta);
  const ttlMs = validateTtl(
    input.ttlMs,
    context.defaultTtlMs,
    context.maxTtlMs,
  );

  const token = generateToken();
  const issuedAt = context.clock();
  const expiresAt = addMilliseconds(issuedAt, ttlMs);
  const id = context.idFactory();

  const record = {
    id,
    subject,
    purpose,
    meta,
    issuedAt: new Date(toMillis(issuedAt)).toISOString(),
    expiresAt: expiresAt.toISOString(),
    digest: digest(token, context.pepper),
  };

  await context.store.put(token, record);

  if (context.audit && typeof context.audit.record === 'function') {
    context.audit.record({
      event: 'token.issued',
      id,
      subject,
      purpose,
      token,
    });
  }

  return {
    token,
    id,
    subject,
    purpose,
    issuedAt: record.issuedAt,
    expiresAt: record.expiresAt,
  };
}

export async function pruneExpired(ctx, now) {
  const context = normalizeContext(ctx);
  const cutoff = toMillis(now ?? context.clock());
  return context.store.pruneExpired(cutoff);
}
