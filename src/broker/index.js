import { topicMatchesFilter } from '../acl/match.js';
import { splitSharedSubscription } from '../acl/share.js';
import { onSubscribe, unsubscribe } from './subscribe.js';
import { deliverMessage, deliverRetained } from './deliver.js';
import { publishWill } from './will.js';

/**
 * Minimal in-memory broker facade used by the plugin tests and examples.
 *
 * It owns sessions, the retained store and the fan-out loop. Authorization is
 * delegated to the supplied ACL instance.
 */
export function createBroker({ acl, retained = [] } = {}) {
  if (!acl) {
    throw new TypeError('createBroker requires an acl instance');
  }

  const sessions = new Map();
  const retainedStore = new Map(retained.map((entry) => [entry.topic, { ...entry }]));

  function sessionFor(client) {
    if (!sessions.has(client.id)) {
      sessions.set(client.id, {
        id: client.id,
        identity: client.identity ?? { clientId: client.id },
        subscriptions: [],
        will: null,
        connected: true,
      });
    }
    const session = sessions.get(client.id);
    if (client.identity) {
      session.identity = client.identity;
    }
    session.connected = true;
    return session;
  }

  function matchFilterOf(subscription) {
    const shared = splitSharedSubscription(subscription.filter);
    return shared ? shared.filter : subscription.filter;
  }

  function activeSubscriptions() {
    return [...sessions.values()]
      .filter((session) => session.connected)
      .flatMap((session) =>
        session.subscriptions.map((subscription) => ({ session, subscription })),
      );
  }

  function fanout(message) {
    const deliveries = [];
    for (const { session, subscription } of activeSubscriptions()) {
      const innerFilter = matchFilterOf(subscription);
      if (!topicMatchesFilter(innerFilter, message.topic)) {
        continue;
      }
      const result = deliverMessage({
        acl,
        identity: session.identity,
        subscription: { ...subscription, filter: innerFilter },
        topic: message.topic,
        payload: message.payload,
        retain: Boolean(message.retain),
      });
      if (result.delivered) {
        deliveries.push({ clientId: session.id, ...result });
      }
    }
    return deliveries;
  }

  /**
   * Internal publish channel. Accepts an already authorized message and fans it
   * out to interested subscriptions.
   */
  function publishRaw(message) {
    if (message.retain) {
      retainedStore.set(message.topic, { topic: message.topic, payload: message.payload });
    }
    return fanout(message);
  }

  function subscribe(client, filter) {
    const session = sessionFor(client);
    const result = onSubscribe({ acl, identity: session.identity, filter, session });
    if (!result.ok) {
      return result;
    }

    const retainedDeliveries = deliverRetained({
      acl,
      identity: session.identity,
      subscription: { ...result.subscription, filter: matchFilterOf(result.subscription) },
      retained: [...retainedStore.values()],
    });

    return { ...result, retainedDeliveries };
  }

  /**
   * Bring back a client whose session was kept across a disconnect.
   *
   * Subscriptions on the session survive the reconnect, so the retained set is
   * replayed for each of them without a new subscribe request.
   */
  function resume(client) {
    const session = sessionFor(client);
    session.connected = true;
    const retainedDeliveries = session.subscriptions.flatMap((subscription) =>
      deliverRetained({
        acl,
        identity: session.identity,
        subscription: { ...subscription, filter: matchFilterOf(subscription) },
        retained: [...retainedStore.values()],
      }),
    );

    return { ok: true, session, retainedDeliveries };
  }

  function unsubscribeClient(client, filter) {
    return unsubscribe({ session: sessionFor(client), filter });
  }

  function publish(client, topic, payload, options = {}) {
    const session = sessionFor(client);
    const decision = acl.publish(session.identity, topic);
    if (!decision.allowed) {
      return { ok: false, reason: 'not-authorized', decision };
    }

    const message = {
      topic,
      payload,
      retain: Boolean(options.retain),
      qos: options.qos ?? 0,
      source: 'client',
      clientId: session.id,
    };

    return { ok: true, message, deliveries: publishRaw(message) };
  }

  function attachWill(client, will) {
    sessionFor(client).will = will;
    return { ok: true };
  }

  /**
   * Close a connection.
   *
   * A clean disconnect drops the session. An abnormal one keeps it so the
   * client can resume its subscriptions later, and publishes the will first.
   */
  function disconnect(client, { clean = false } = {}) {
    const session = sessionFor(client);
    const will = session.will;
    session.will = null;

    if (clean) {
      sessions.delete(client.id);
      return { ok: true, will: null };
    }

    session.connected = false;

    if (!will) {
      return { ok: true, will: null };
    }

    return publishWill({
      acl,
      identity: session.identity,
      will,
      publish: publishRaw,
    });
  }

  return {
    acl,
    get retained() {
      return [...retainedStore.values()];
    },
    sessionFor,
    subscribe,
    resume,
    unsubscribe: unsubscribeClient,
    publish,
    attachWill,
    disconnect,
  };
}
