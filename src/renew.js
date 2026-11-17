import { materialize } from './mount.js';

export async function renewOnce(session) {
  if (!session.leaseId) {
    throw new Error('cannot renew without an active lease');
  }

  const bundle = await session.broker.renewLease(session.leaseId);
  const written = materialize(session.dir, bundle.secrets);

  session.leaseId = bundle.leaseId;
  if (typeof bundle.ttlSeconds === 'number') {
    session.leaseSeconds = bundle.ttlSeconds;
  }

  return written;
}
