import { createEngine } from '../src/index.js';

const engine = await createEngine({ policyDir: './policies' });

const requests = [
  {
    principal: '/users/alice',
    resource: '/docs/reports/q3',
    action: 'read',
    context: { role: 'member', authenticated: true },
  },
  {
    principal: '/users/alice',
    resource: '/docs/reports/q3',
    action: 'delete',
    context: { role: 'member', archived: true },
  },
  {
    principal: '/users/alice',
    resource: '/billing/invoices/42',
    action: 'refund',
    context: { role: 'member', locked: false, amount: 1200 },
  },
  {
    principal: '/users/bob',
    resource: '/healthz',
    action: 'read',
  },
];

for (const [index, result] of engine.decideMany(requests).entries()) {
  const request = requests[index];
  console.log(
    `${request.principal} ${request.action} ${request.resource} -> ${result.allow ? 'allow' : 'deny'} (${result.reason})`,
  );
}
