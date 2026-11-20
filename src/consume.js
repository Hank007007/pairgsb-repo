import { digest, matches } from './token.js';
import { ERROR_CODES, TokenError } from './errors.js';
import { toMillis } from './time.js';

function normalizeContext(ctx) {
  if (ctx === null || typeof ctx !== 'object') {
    throw new TypeError('consume() requires a context object');
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
  };
}

function auditRejection(context, details) {
  if (context.audit && typeof context.audit.record === 'function') {
    context.audit.record({
      event: 'token.rejected',
      id: details.id ?? null,
      subject: details.subject ?? null,
      purpose: details.purpose ?? null,
      reason: details.reason ?? null,
      token: details.token,
    });
  }
}

export async function consume(ctx, input = {}) {
  const context = normalizeContext(ctx);
  const { token } = input;

  if (typeof token !== 'string' || token.length === 0) {
    throw new TokenError(ERROR_CODES.TOKEN_MISSING, 'token is required');
  }

  const record = await context.store.findByToken(token);

  if (record === null) {
    const used = await context.store.wasConsumed(token);
    if (used) {
      auditRejection(context, {
        token,
        id: null,
        subject: null,
        purpose: input.purpose ?? null,
        reason: 'already-used',
      });
      throw new TokenError(
        ERROR_CODES.TOKEN_ALREADY_USED,
        'token has already been used',
      );
    }
    auditRejection(context, {
      token,
      id: null,
      subject: input.subject ?? null,
      purpose: input.purpose ?? null,
      reason: 'unknown',
    });
    throw new TokenError(ERROR_CODES.TOKEN_UNKNOWN, 'token is not recognised');
  }

  const now = context.clock();
  if (toMillis(record.expiresAt) <= toMillis(now)) {
    try {
      await context.store.remove(token);
    } catch {}
    auditRejection(context, {
      token,
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      reason: 'expired',
    });
    throw new TokenError(ERROR_CODES.TOKEN_EXPIRED, 'token has expired');
  }

  if (!matches(record.digest, digest(token, context.pepper))) {
    auditRejection(context, {
      token,
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      reason: 'unknown',
    });
    throw new TokenError(ERROR_CODES.TOKEN_UNKNOWN, 'token is not recognised');
  }

  if (input.subject !== undefined && input.subject !== null) {
    const presented =
      typeof input.subject === 'string' ? input.subject.trim() : null;
    if (presented !== record.subject) {
      auditRejection(context, {
        token,
        id: record.id,
        subject: record.subject,
        purpose: record.purpose,
        reason: 'subject-mismatch',
      });
      throw new TokenError(
        ERROR_CODES.TOKEN_SUBJECT_MISMATCH,
        'token is bound to a different subject',
      );
    }
  }

  if (
    input.purpose !== undefined &&
    input.purpose !== null &&
    input.purpose !== record.purpose
  ) {
    auditRejection(context, {
      token,
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      reason: 'purpose-mismatch',
    });
    throw new TokenError(
      ERROR_CODES.TOKEN_PURPOSE_MISMATCH,
      'token is bound to a different purpose',
    );
  }

  let removed;
  try {
    removed = await context.store.remove(token);
  } catch {
    auditRejection(context, {
      token,
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      reason: 'storage-failure',
    });
    throw new TokenError(
      ERROR_CODES.STORAGE_FAILURE,
      'token could not be consumed because storage failed',
    );
  }

  if (!removed) {
    auditRejection(context, {
      token,
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      reason: 'already-used',
    });
    throw new TokenError(
      ERROR_CODES.TOKEN_ALREADY_USED,
      'token has already been used',
    );
  }

  if (context.audit && typeof context.audit.record === 'function') {
    context.audit.record({
      event: 'token.consumed',
      id: record.id,
      subject: record.subject,
      purpose: record.purpose,
      token,
    });
  }

  return {
    id: record.id,
    subject: record.subject,
    purpose: record.purpose,
    meta: record.meta,
    issuedAt: record.issuedAt,
    expiresAt: record.expiresAt,
    consumedAt: new Date(toMillis(now)).toISOString(),
  };
}
