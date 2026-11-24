'use strict';

// HTTP surface of the sidecar. Every route is local-only by construction: the
// server listens on a unix socket or a named pipe.

const { allows, resolveScopes } = require('../lib/policy');
const { LeaseError } = require('../lib/lease');

const MAX_BODY = 1024 * 1024;

function sendJson(res, status, payload) {
  const body = Buffer.from(JSON.stringify(payload), 'utf8');
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': body.length,
  });
  res.end(body);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;

    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY) {
        reject(new Error('body_too_large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

async function readJson(req) {
  const body = await readBody(req);
  if (body.length === 0) return {};
  return JSON.parse(body.toString('utf8'));
}

function stringList(value) {
  if (!Array.isArray(value)) return [];
  return value.filter((item) => typeof item === 'string');
}

async function handleLease(req, res, context) {
  let request;
  try {
    request = await readJson(req);
  } catch {
    sendJson(res, 400, { error: 'invalid_json' });
    return;
  }

  if (!request || typeof request.job !== 'string' || request.job === '') {
    sendJson(res, 400, { error: 'invalid_job' });
    return;
  }

  const pipelineName = request.pipeline;
  const declaration = context.pipelines.get(pipelineName);
  if (!declaration) {
    sendJson(res, 404, { error: 'unknown_pipeline' });
    return;
  }

  const branch = typeof request.branch === 'string' ? request.branch : '';
  const requestedScopes = stringList(request.scopes);

  for (const scope of requestedScopes) {
    if (!allows(context.policy, { pipeline: pipelineName, branch, scope })) {
      sendJson(res, 403, { error: 'policy_denied', scope });
      return;
    }
  }

  const grants = [];
  for (const scope of requestedScopes) {
    const values = context.store.fetch(scope) || {};
    grants.push({ scope, values });
  }

  const merged = resolveScopes(grants, context.policy.scopes);

  const lease = context.leases.issue(request, {
    max_duration_ms: declaration.max_duration_ms,
    values: merged,
  });

  const secrets = [];
  for (const grant of grants) {
    for (const name of Object.keys(grant.values)) {
      secrets.push(grant.values[name]);
    }
  }
  context.logs.open(request.job, secrets);

  sendJson(res, 200, lease);
}

function parseLeaseId(pathname) {
  const match = /^\/v1\/lease\/([^/]+)$/.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

function parseLogJob(pathname) {
  const match = /^\/v1\/logs\/([^/]+)$/.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

async function handle(req, res, context) {
  const url = new URL(req.url, 'http://localhost');
  const pathname = url.pathname;

  try {
    if (req.method === 'POST' && pathname === '/v1/lease') {
      await handleLease(req, res, context);
      return;
    }

    const leaseId = parseLeaseId(pathname);
    if (leaseId !== null) {
      if (req.method === 'GET') {
        try {
          const contents = context.leases.read(leaseId);
          res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' });
          res.end(contents);
        } catch (error) {
          if (error instanceof LeaseError) {
            sendJson(res, 404, { error: error.code });
            return;
          }
          throw error;
        }
        return;
      }

      if (req.method === 'DELETE') {
        const revoked = context.leases.revoke(leaseId);
        sendJson(res, revoked ? 200 : 404, { revoked });
        return;
      }
    }

    const job = parseLogJob(pathname);
    if (job !== null) {
      if (req.method === 'POST') {
        const body = await readBody(req);
        if (!context.logs.write(job, body)) {
          sendJson(res, 404, {
            error: 'unknown_job',
            job,
            detail: 'log stream is not open for this job',
            store: context.storePath,
            secrets: context.store.listNames(),
          });
          return;
        }
        sendJson(res, 202, { accepted: body.length });
        return;
      }

      if (req.method === 'GET') {
        const output = context.logs.snapshot(job);
        if (output === null) {
          sendJson(res, 404, {
            error: 'unknown_job',
            job,
            detail: 'log stream is not open for this job',
            store: context.storePath,
            secrets: context.store.listNames(),
          });
          return;
        }
        res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' });
        res.end(output);
        return;
      }
    }

    sendJson(res, 404, { error: 'not_found' });
  } catch (error) {
    sendJson(res, 500, { error: 'internal_error' });
  }
}

module.exports = { handle };
