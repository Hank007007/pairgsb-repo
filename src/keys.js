import { readFileSync } from 'node:fs';

/**
 * Key material loading.
 *
 * keys.json maps a tenant name to a list of versioned secrets:
 *
 *   {
 *     "global":   [ { "version": 1, "secret": "..." } ],
 *     "acme":     [
 *       { "version": 1, "secret": "...", "not_after": 1767225600 },
 *       { "version": 2, "secret": "..." }
 *     ]
 *   }
 *
 * The first entry of a tenant list is the version used for signing; the
 * remaining entries exist so that links issued before a rotation keep
 * verifying for a while.
 */

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function normalizeVersion(raw, where) {
  if (!isPlainObject(raw)) {
    throw new TypeError(`${where}: version entry must be an object`);
  }
  const version = Number(raw.version);
  if (!Number.isInteger(version) || version < 1) {
    throw new TypeError(`${where}: version must be a positive integer`);
  }
  if (typeof raw.secret !== 'string' || raw.secret.length === 0) {
    throw new TypeError(`${where}: secret must be a non-empty string`);
  }
  let notAfter = null;
  if (raw.not_after !== undefined && raw.not_after !== null) {
    notAfter = Number(raw.not_after);
    if (!Number.isFinite(notAfter)) {
      throw new TypeError(`${where}: not_after must be a unix timestamp`);
    }
  }
  return Object.freeze({ version, secret: raw.secret, not_after: notAfter });
}

/**
 * Parse the keyring document into plain JavaScript values.
 */
export function parseKeyring(text, source = 'keys.json') {
  let doc;
  try {
    doc = JSON.parse(text);
  } catch (err) {
    throw new Error(`${source}: not valid JSON (${err.message})`);
  }
  if (!isPlainObject(doc)) {
    throw new Error(`${source}: top level must be an object of tenants`);
  }

  const keyring = Object.create(null);
  for (const [tenant, versions] of Object.entries(doc)) {
    if (!Array.isArray(versions) || versions.length === 0) {
      throw new Error(`${source}: tenant "${tenant}" must list at least one version`);
    }
    const seen = new Set();
    const normalized = [];
    for (let i = 0; i < versions.length; i += 1) {
      const entry = normalizeVersion(versions[i], `${source}: tenant "${tenant}" [${i}]`);
      if (seen.has(entry.version)) {
        throw new Error(
          `${source}: tenant "${tenant}" contains duplicate version ${entry.version}`,
        );
      }
      seen.add(entry.version);
      normalized.push(entry);
    }
    keyring[tenant] = Object.freeze(normalized);
  }
  return keyring;
}

/**
 * Read and parse a keyring from disk.
 */
export function load(filePath) {
  const text = readFileSync(filePath, 'utf8');
  return parseKeyring(text, filePath);
}

/**
 * The version used to sign new links for a tenant.
 */
export function activeVersion(versions) {
  return versions[0];
}

/**
 * Every version that is acceptable at `now`, independent of file order.
 */
export function candidateVersions(versions, now = Math.floor(Date.now() / 1000)) {
  if (!Array.isArray(versions)) return [];
  const clock = Number(now);
  return versions
    .filter((version) => version.not_after == null || clock <= version.not_after)
    .sort((a, b) => b.version - a.version);
}
