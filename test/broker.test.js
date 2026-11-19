import test from 'node:test';
import assert from 'node:assert/strict';

import { createAcl } from '../src/acl/reload.js';
import { onSubscribe, unsubscribe } from '../src/broker/subscribe.js';
import { deliverMessage, deliverRetained } from '../src/broker/deliver.js';
import { publishWill } from '../src/broker/will.js';
import { createBroker } from '../src/broker/index.js';

const rules = {
  principals: {
    'user:alice': {
      rules: [
        { action: 'publish', topic: 'sensors/a', effect: 'allow' },
        { action: 'subscribe', topic: 'sensors/a', effect: 'allow' },
      ],
    },
    'user:bob': {
      rules: [
        { action: 'publish', topic: 'lab/bench', effect: 'allow' },
        { action: 'subscribe', topic: 'sensors/a', effect: 'allow' },
      ],
    },
    'client:dev-1': {
      rules: [{ action: 'publish', topic: 'fleet/dev-1/status', effect: 'allow' }],
    },
  },
};

function newAcl() {
  return createAcl({ rules });
}

test('onSubscribe records a granted subscription on the session', () => {
  const acl = newAcl();
  const session = { subscriptions: [] };

  const result = onSubscribe({
    acl,
    identity: { username: 'alice' },
    filter: 'sensors/a',
    session,
  });

  assert.equal(result.ok, true);
  assert.equal(session.subscriptions.length, 1);
  assert.equal(session.subscriptions[0].filter, 'sensors/a');
  assert.equal(session.subscriptions[0].version, acl.version);
});

test('onSubscribe rejects a filter with no grant and a malformed filter', () => {
  const acl = newAcl();

  const denied = onSubscribe({ acl, identity: { username: 'alice' }, filter: 'farm/a' });
  assert.equal(denied.ok, false);
  assert.equal(denied.reason, 'not-authorized');
  assert.equal(denied.decision.reason, 'default-deny');

  const malformed = onSubscribe({ acl, identity: { username: 'alice' }, filter: 'sensors/#/a' });
  assert.equal(malformed.ok, false);
  assert.equal(malformed.reason, 'invalid-filter');
});

test('unsubscribe drops every entry carrying the requested filter', () => {
  const acl = newAcl();
  const session = { subscriptions: [] };
  onSubscribe({ acl, identity: { username: 'alice' }, filter: 'sensors/a', session });
  onSubscribe({ acl, identity: { username: 'bob' }, filter: 'sensors/a', session });

  const result = unsubscribe({ session, filter: 'sensors/a' });

  assert.equal(result.removed, 2);
  assert.deepEqual(session.subscriptions, []);
});

test('deliverMessage follows the subscription filter and the receiver identity', () => {
  const acl = newAcl();
  const subscription = { filter: 'sensors/a', decision: { allowed: true } };

  const delivered = deliverMessage({
    acl,
    identity: { username: 'alice' },
    subscription,
    topic: 'sensors/a',
    payload: 'ok',
  });
  assert.equal(delivered.delivered, true);
  assert.equal(delivered.payload, 'ok');

  const offFilter = deliverMessage({
    acl,
    identity: { username: 'alice' },
    subscription,
    topic: 'sensors/b',
    payload: 'ok',
  });
  assert.equal(offFilter.delivered, false);
  assert.equal(offFilter.reason, 'filter-mismatch');

  const unauthorized = deliverMessage({
    acl,
    identity: { username: 'mallory' },
    subscription,
    topic: 'sensors/a',
    payload: 'ok',
  });
  assert.equal(unauthorized.delivered, false);
  assert.equal(unauthorized.reason, 'not-authorized');
});

test('deliverRetained selects retained entries for a granted subscription', () => {
  const acl = newAcl();
  const granted = { filter: 'sensors/a', decision: { allowed: true } };

  const out = deliverRetained({
    acl,
    identity: { username: 'alice' },
    subscription: granted,
    retained: [
      { topic: 'sensors/a', payload: 'first' },
      { topic: 'sensors/b', payload: 'second' },
    ],
  });

  assert.equal(out.length, 1);
  assert.equal(out[0].topic, 'sensors/a');
  assert.equal(out[0].retain, true);
});

test('publishWill publishes an allowed will and rejects one without a topic', () => {
  const acl = newAcl();
  const published = [];

  const result = publishWill({
    acl,
    identity: { username: 'alice' },
    will: { topic: 'sensors/a', payload: 'offline' },
    publish: (message) => {
      published.push(message);
      return [{ clientId: 'observer' }];
    },
  });

  assert.equal(result.ok, true);
  assert.equal(published.length, 1);
  assert.equal(published[0].topic, 'sensors/a');
  assert.equal(published[0].source, 'will');
  assert.equal(result.deliveries.length, 1);

  const invalid = publishWill({
    acl,
    identity: { username: 'alice' },
    will: { payload: 'offline' },
    publish: () => [],
  });
  assert.equal(invalid.ok, false);
  assert.equal(invalid.reason, 'invalid-will');
});

test('broker authorizes publishes and fans out to subscribers', () => {
  const acl = newAcl();
  const broker = createBroker({ acl });
  const alice = { id: 'c-alice', identity: { username: 'alice' } };
  const bob = { id: 'c-bob', identity: { username: 'bob' } };

  broker.subscribe(bob, 'sensors/a');
  const accepted = broker.publish(alice, 'sensors/a', 'hello');

  assert.equal(accepted.ok, true);
  assert.equal(accepted.deliveries.length, 1);
  assert.equal(accepted.deliveries[0].clientId, 'c-bob');

  const refused = broker.publish(alice, 'farm/a', 'nope');
  assert.equal(refused.ok, false);
  assert.equal(refused.reason, 'not-authorized');
});

test('broker replays retained entries on subscribe and on resume', () => {
  const acl = newAcl();
  const broker = createBroker({
    acl,
    retained: [{ topic: 'sensors/a', payload: 'snapshot' }],
  });
  const alice = { id: 'c-alice', identity: { username: 'alice' } };

  const replay = broker.subscribe(alice, 'sensors/a');
  assert.equal(replay.ok, true);
  assert.equal(replay.retainedDeliveries.length, 1);
  assert.equal(replay.retainedDeliveries[0].payload, 'snapshot');

  broker.disconnect(alice, { clean: false });
  const resumed = broker.resume(alice);
  assert.equal(resumed.ok, true);
  assert.equal(resumed.session.subscriptions.length, 1);
  assert.equal(resumed.retainedDeliveries.length, 1);
});

test('broker skips the will on a clean disconnect', () => {
  const acl = newAcl();
  const broker = createBroker({ acl });
  const alice = { id: 'c-alice', identity: { username: 'alice' } };

  broker.attachWill(alice, { topic: 'sensors/a', payload: 'offline' });
  const closed = broker.disconnect(alice, { clean: true });

  assert.equal(closed.ok, true);
  assert.equal(closed.will, null);
});
