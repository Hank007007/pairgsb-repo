'use strict';

// Lease bookkeeping.
//
// A lease gives one job a readable env file for a bounded amount of time. The
// manager owns the lease table, the env files and the read handles handed to
// agents.

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const envfile = require('./envfile');

class LeaseError extends Error {
  constructor(code, message) {
    super(message);
    this.name = 'LeaseError';
    this.code = code;
  }
}

function newLeaseId() {
  return crypto.randomBytes(12).toString('hex');
}

function safeSegment(text) {
  return String(text).replace(/[^A-Za-z0-9_.-]/g, '_');
}

class LeaseManager {
  constructor(options) {
    const opts = options || {};
    this.leasesDir = opts.leasesDir;
    this.now = typeof opts.now === 'function' ? opts.now : () => Date.now();
    this.leases = new Map();
  }

  // request: { job, ttl_ms }
  // resolved: { max_duration_ms, values }
  issue(request, resolved) {
    const job = String(request.job);
    const leaseId = newLeaseId();
    const envPath = path.join(this.leasesDir, safeSegment(job) + '-' + leaseId + '.env');

    const ttl = Number.isFinite(request.ttl_ms) ? request.ttl_ms : resolved.max_duration_ms;
    const issuedAt = this.now();
    const expiresAt = issuedAt + ttl;

    envfile.write(resolved.values, envPath);

    const fd = fs.openSync(envPath, 'r');
    const lease = {
      lease_id: leaseId,
      job,
      env_path: envPath,
      issued_at: issuedAt,
      expires_at: expiresAt,
      state: 'active',
      fd,
    };

    this.leases.set(leaseId, lease);

    return this.describe(lease);
  }

  describe(lease) {
    return {
      lease_id: lease.lease_id,
      env_path: lease.env_path,
      expires_at: new Date(lease.expires_at).toISOString(),
    };
  }

  get(leaseId) {
    return this.leases.get(leaseId);
  }

  // Reads the env file through the handle captured at issue time.
  read(leaseId) {
    const lease = this.leases.get(leaseId);
    if (!lease) throw new LeaseError('unknown_lease', 'no such lease');

    const buffer = Buffer.alloc(fs.fstatSync(lease.fd).size);
    const bytes = fs.readSync(lease.fd, buffer, 0, buffer.length, 0);
    return buffer.subarray(0, bytes).toString('utf8');
  }

  expire(leaseId) {
    const lease = this.leases.get(leaseId);
    if (!lease) return false;

    this.leases.delete(leaseId);
    return true;
  }

  revoke(leaseId) {
    const lease = this.leases.get(leaseId);
    if (!lease) return false;

    lease.state = 'revoked';
    envfile.remove(lease.env_path);
    return true;
  }

  // Reclaims every lease whose deadline has passed.
  sweep() {
    const current = this.now();
    const reclaimed = [];

    for (const [leaseId, lease] of this.leases) {
      if (lease.expires_at <= current) {
        this.expire(leaseId);
        reclaimed.push(leaseId);
      }
    }

    return reclaimed;
  }
}

module.exports = { LeaseManager, LeaseError, newLeaseId };
