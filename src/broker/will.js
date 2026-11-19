/**
 * Publish a client's will message.
 *
 * `publish` is the broker's internal publish channel, the same one used for
 * messages that already passed authorization.
 */
export function publishWill({ acl, identity, will, publish }) {
  if (!will || typeof will.topic !== 'string' || will.topic.length === 0) {
    return { ok: false, reason: 'invalid-will' };
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
