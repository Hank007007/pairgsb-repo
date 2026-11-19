/**
 * Publish a client's will message.
 *
 * The will represents the client's own publication, so it must pass the same
 * publish check with the client's authenticated identity before entering the
 * broker's internal publish channel. A rejected will is neither published nor
 * written to the retained store.
 */
export function publishWill({ acl, identity, will, publish }) {
  if (!will || typeof will.topic !== 'string' || will.topic.length === 0) {
    return { ok: false, reason: 'invalid-will' };
  }

  const decision = acl.publish(identity, will.topic);
  if (!decision.allowed) {
    return { ok: false, reason: 'not-authorized', decision };
  }

  const message = {
    topic: will.topic,
    payload: will.payload ?? null,
    retain: Boolean(will.retain),
    qos: will.qos ?? 0,
    source: 'will',
    clientId: identity?.clientId ?? null,
  };

  return { ok: true, message, deliveries: publish(message) };
}
