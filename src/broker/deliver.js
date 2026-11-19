import { topicMatchesFilter } from '../acl/match.js';

/**
 * Deliver a live message to one subscription.
 *
 * The subscription filter decides whether the topic is of interest; the ACL is
 * consulted for this delivery.
 */
export function deliverMessage({ acl, identity, subscription, topic, payload, retain = false }) {
  if (!topicMatchesFilter(subscription.filter, topic)) {
    return { delivered: false, reason: 'filter-mismatch', topic };
  }

  const decision = acl.canReceive(identity, topic);
  if (!decision.allowed) {
    return { delivered: false, reason: 'not-authorized', topic, decision };
  }

  return {
    delivered: true,
    topic,
    payload,
    retain,
    filter: subscription.filter,
  };
}

/**
 * Deliver the retained set to a subscription.
 *
 * The retained entries are filtered by the subscription's filter and stamped
 * with the client they are being delivered to. Each entry is re-authorized at
 * delivery time against its concrete topic: the cached subscription decision
 * only covers the subscription request itself and must not be reused after a
 * reload tightens the rules. Rejected entries are silently skipped.
 */
export function deliverRetained({ acl, identity, subscription, retained }) {
  const out = [];
  for (const entry of retained) {
    if (!topicMatchesFilter(subscription.filter, entry.topic)) {
      continue;
    }
    const decision = acl.canReceive(identity, entry.topic);
    if (!decision.allowed) {
      continue;
    }
    out.push({
      delivered: true,
      topic: entry.topic,
      payload: entry.payload,
      retain: true,
      filter: subscription.filter,
      clientId: identity?.clientId ?? null,
      decision,
    });
  }
  return out;
}
