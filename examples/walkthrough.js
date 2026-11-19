import { fileURLToPath } from 'node:url';

import { createAcl, reloadAcl } from '../src/acl/reload.js';
import { createBroker } from '../src/broker/index.js';

const directory = fileURLToPath(new URL('../acl/', import.meta.url));

const acl = createAcl({ rules: { principals: {} } });
await reloadAcl(acl, directory);

const broker = createBroker({
  acl,
  retained: [
    { topic: 'fleet/dev-1001/status', payload: '{"online":true}' },
    { topic: 'farm/public/notice', payload: '{"text":"maintenance at 22:00"}' },
  ],
});

const dev = { id: 'dev-1001', identity: { username: 'nobody', clientId: 'dev-1001' } };
const ops = { id: 'ops-console', identity: { username: 'ops' } };

console.log('rule version', acl.version, 'loaded at', acl.loadedAt);

console.log('\n[1] subscription decisions');
for (const [client, filter] of [
  [dev, 'fleet/dev-1001/command'],
  [dev, 'fleet/dev-1002/command'],
  [ops, 'farm/public/notice'],
  [ops, 'farm/private/notice'],
]) {
  const result = broker.subscribe(client, filter);
  console.log(
    ' ',
    client.id.padEnd(12),
    filter.padEnd(28),
    result.ok ? 'granted' : result.reason,
  );
}

console.log('\n[2] publish decisions');
for (const [client, topic] of [
  [dev, 'fleet/dev-1001/status'],
  [dev, 'fleet/dev-1002/status'],
  [ops, 'farm/public/notice'],
]) {
  const result = broker.publish(client, topic, 'payload');
  console.log(
    ' ',
    client.id.padEnd(12),
    topic.padEnd(28),
    result.ok ? `delivered to ${result.deliveries.length}` : result.reason,
  );
}

console.log('\n[3] retained replay');
const replay = broker.subscribe(ops, 'farm/public/notice');
console.log(' ', 'ops-console', 'farm/public/notice'.padEnd(28), replay.ok
  ? `${replay.retainedDeliveries.length} retained`
  : replay.reason);

console.log('\n[4] will on abnormal disconnect');
broker.attachWill(dev, { topic: 'fleet/dev-1001/status', payload: '{"online":false}' });
const outcome = broker.disconnect(dev, { clean: false });
console.log(' ', 'dev-1001', 'will', outcome.ok ? 'published' : outcome.reason);

console.log('\n[5] SIGHUP-style reload');
acl.replaceRules(acl.rules, { loadedAt: new Date().toISOString() });
console.log(' ', 'new rule version', acl.version, 'loaded at', acl.loadedAt);
