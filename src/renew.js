import { materialize, removeMounted } from './mount.js';

export async function renewOnce(session) {
  if (!session.leaseId) {
    throw new Error('cannot renew without an active lease');
  }

  const bundle = await session.broker.renewLease(session.leaseId);
  const written = materialize(session.dir, bundle.secrets);

  const previous = session.mounted ?? new Set();
  for (const name of previous) {
    if (!written.includes(name)) {
      removeMounted(session.dir, name);
    }
  }
  session.mounted = new Set(written);

  session.leaseId = bundle.leaseId;
  if (typeof bundle.ttlSeconds === 'number') {
    session.leaseSeconds = bundle.ttlSeconds;
  }

  return written;
}
