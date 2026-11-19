import { isValidSubscriptionFilter } from '../acl/match.js';

/**
 * Authorize a subscription request.
 *
 * `filter` is exactly what the client sent, including any `$share/...`
 * wrapper. A granted subscription is recorded on the session together with
 * the decision that granted it.
 */
export function onSubscribe({ acl, identity, filter, session = null }) {
  if (!isValidSubscriptionFilter(filter)) {
    return { ok: false, reason: 'invalid-filter', filter };
  }

  const decision = acl.subscribe(identity, filter);
  if (!decision.allowed) {
    return { ok: false, reason: 'not-authorized', filter, decision };
  }

  const subscription = {
    filter,
    decision,
    version: acl.version,
    grantedAt: acl.loadedAt,
  };

  if (session && Array.isArray(session.subscriptions)) {
    session.subscriptions.push(subscription);
  }

  return { ok: true, subscription };
}

/**
 * Remove every subscription on the session whose filter is `filter`.
 */
export function unsubscribe({ session, filter }) {
  if (!session || !Array.isArray(session.subscriptions)) {
    return { ok: false, reason: 'no-session' };
  }
  const before = session.subscriptions.length;
  session.subscriptions = session.subscriptions.filter((entry) => entry.filter !== filter);
  return { ok: true, removed: before - session.subscriptions.length };
}
