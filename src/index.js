/**
 * urlseal-presign — presigned URL issuing and verification.
 */

export {
  presign,
  buildCanonicalString,
  requestQuery,
  CANONICAL_VERSION,
  DEFAULT_EXPIRES_IN,
  DEFAULT_HOST,
  PROTOCOL_ALG,
  PROTOCOL_EXPIRES,
  PROTOCOL_METHOD,
  PROTOCOL_TENANT,
  PROTOCOL_VERSION,
  SIG_PARAM,
} from './presign.js';

export {
  verify,
  CLOCK_SKEW_SECONDS,
} from './verify.js';

export {
  parseKeyring,
  load,
  activeVersion,
  candidateVersions,
} from './keys.js';

export {
  parseQuery,
  stripProtocolParams,
  canonicalQuery,
  canonicalKey,
  encodePath,
} from './canonical.js';

export {
  hmac,
  signaturesMatch,
  resolveKey,
  DEFAULT_ALG,
} from './sign.js';
