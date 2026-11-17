import { createService, createFixedClock, IssueError } from '../src/index.js';

function heading(text) {
  process.stdout.write(`\n== ${text} ==\n`);
}

function show(label, value) {
  process.stdout.write(`${label}: ${JSON.stringify(value)}\n`);
}

export function runDemo({ dataFile }) {
  const clock = createFixedClock('2026-11-18T00:00:00.000Z');
  const service = createService({
    dataFile,
    clock,
    graceMs: 15 * 60 * 1000,
    logger: {
      debug() {},
      info() {},
      warn(message, fields) {
        process.stdout.write(`warn: ${message} ${JSON.stringify(fields)}\n`);
      },
      error() {},
    },
  });

  heading('issue');
  const issued = service.issue({
    ownerId: 'checkout-team',
    scopes: ['orders:read', 'orders:write'],
    env: 'prod',
  });
  show('key', issued.key);
  show('verify as prod', service.verify(issued.key, { env: 'prod' }));

  heading('use a prod key against staging');
  show('verify as staging', service.verify(issued.key, { env: 'staging' }));

  heading('rotate');
  const rotated = service.rotate(issued.id8);
  show('successor', rotated.key);
  show('graceUntil', rotated.graceUntil);

  clock.advance(30 * 60 * 1000);
  heading('old key after the grace window');
  show('verify old key', service.verify(issued.key, { env: 'prod' }));

  heading('revoke after the key has been verified');
  show('verify before revoke', service.verify(rotated.key, { env: 'prod' }));
  show('revoke', service.revoke(rotated.id8));
  show('verify after revoke', service.verify(rotated.key, { env: 'prod' }));
  show('revoke again', service.revoke(rotated.id8));

  heading('storage file');
  for (const record of service.list()) {
    show(record.id8, {
      env: record.env,
      ownerId: record.ownerId,
      secretHash: record.secretHash,
    });
  }

  heading('collision check');
  const firstOwner = new Map();
  let collisions = 0;
  for (let index = 0; index < 2000; index += 1) {
    let entry;
    try {
      entry = service.issue({ ownerId: `batch-owner-${index}`, scopes: ['batch:read'] });
    } catch (error) {
      if (error instanceof IssueError) {
        process.stdout.write(`issue error: ${error.code}\n`);
        break;
      }
      throw error;
    }
    if (firstOwner.has(entry.id8)) collisions += 1;
    else firstOwner.set(entry.id8, entry.ownerId);
  }
  show('issued', 2000);
  show('distinct ids', firstOwner.size);
  show('collisions observed', collisions);

  let overwritten = 0;
  for (const [id8, owner] of firstOwner) {
    const record = service.store.getById(id8);
    if (!record || record.ownerId !== owner) overwritten += 1;
  }
  show('earlier records replaced by a later issue', overwritten);
}
