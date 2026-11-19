import { evaluate } from './evaluate.js';
import { loadAclDirectory, parseRuleset } from './parse.js';

/**
 * Build an ACL instance.
 *
 * `replaceRules` swaps the rule table as a whole, which keeps readers on a
 * consistent snapshot and makes the version counter monotonic across reloads.
 */
export function createAcl({ rules, loadedAt = null } = {}) {
  let version = 0;
  let table = rules ? parseRuleset(rules) : { principals: {} };
  let stamp = loadedAt;

  const acl = {
    get version() {
      return version;
    },

    get loadedAt() {
      return stamp;
    },

    get rules() {
      return table;
    },

    replaceRules(nextRules, meta = {}) {
      table = nextRules ? parseRuleset(nextRules) : { principals: {} };
      version += 1;
      stamp = meta.loadedAt ?? stamp;
      return version;
    },

    check(identity, action, subject) {
      return evaluate({ rules: table, identity, action, subject });
    },

    publish(identity, topic) {
      return acl.check(identity, 'publish', topic);
    },

    subscribe(identity, filter) {
      return acl.check(identity, 'subscribe', filter);
    },

    canReceive(identity, topic) {
      return acl.check(identity, 'subscribe', topic);
    },
  };

  return acl;
}

/**
 * Read the rule directory again and install the result. The previous rule set
 * stays in place if loading fails.
 */
export async function reloadAcl(acl, directory, { loadedAt = null } = {}) {
  const next = await loadAclDirectory(directory);
  const version = acl.replaceRules(next, {
    loadedAt: loadedAt ?? new Date().toISOString(),
  });
  return { version, rules: acl.rules };
}

/**
 * Install a SIGHUP listener that reloads the rule directory.
 *
 * Returns a disposer that removes the listener again.
 */
export function installReloadHandler({
  acl,
  directory,
  processRef = process,
  logger = console,
  onError = null,
}) {
  let pending = Promise.resolve();

  const handler = () => {
    pending = reloadAcl(acl, directory)
      .then(() => undefined)
      .catch((error) => {
        if (onError) {
          onError(error);
          return;
        }
        logger.error(`brokeracl: reload failed, keeping rules from version ${acl.version}:`, error);
      });
  };

  processRef.on('SIGHUP', handler);

  return {
    dispose() {
      processRef.off('SIGHUP', handler);
    },
    settled() {
      return pending;
    },
  };
}
