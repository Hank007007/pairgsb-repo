'use strict';

// Application wiring: policy, pipeline declarations, secret store, lease
// manager and per-job log masking all meet here.

const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');

const { Store } = require('./store');
const { LeaseManager } = require('./lease');
const { Transform: MaskStream } = require('./stream');
const api = require('../routes/api');

function readJsonFile(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function loadPipelines(directory) {
  const declarations = new Map();
  if (!directory) return declarations;

  const entries = fs.readdirSync(directory, { withFileTypes: true });
  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith('.json')) continue;
    const document = readJsonFile(path.join(directory, entry.name));
    if (document && typeof document.pipeline === 'string') {
      declarations.set(document.pipeline, document);
    }
  }

  return declarations;
}

class JobLogHub {
  constructor() {
    this.jobs = new Map();
  }

  open(job, secrets) {
    const transform = new MaskStream(secrets);
    const chunks = [];
    transform.on('data', (chunk) => chunks.push(Buffer.from(chunk)));

    const entry = { transform, chunks };
    this.jobs.set(job, entry);
    return entry;
  }

  get(job) {
    return this.jobs.get(job);
  }

  write(job, chunk) {
    const entry = this.jobs.get(job);
    if (!entry) return false;
    entry.transform.write(chunk);
    return true;
  }

  snapshot(job) {
    const entry = this.jobs.get(job);
    if (!entry) return null;
    return Buffer.concat(entry.chunks).toString('utf8');
  }

  close(job) {
    const entry = this.jobs.get(job);
    if (!entry) return;
    entry.transform.end();
    this.jobs.delete(job);
  }
}

function createApp(options) {
  const opts = options || {};

  const policy = opts.policy || { scopes: [], rules: [] };
  const pipelines = opts.pipelines instanceof Map
    ? opts.pipelines
    : loadPipelines(opts.pipelinesDir);
  const store = opts.store;
  const leasesDir = opts.leasesDir;

  const leases = new LeaseManager({ leasesDir, now: opts.now });
  const logs = new JobLogHub();

  const context = {
    policy,
    pipelines,
    store,
    leases,
    logs,
    storePath: opts.storePath || null,
    now: typeof opts.now === 'function' ? opts.now : () => Date.now(),
  };

  const server = http.createServer((req, res) => {
    api.handle(req, res, context);
  });

  return {
    server,
    context,
    leases,
    logs,
  };
}

function createAppFromFiles(options) {
  const opts = options || {};
  return createApp({
    policy: readJsonFile(opts.policyPath),
    pipelinesDir: opts.pipelinesDir,
    store: Store.open(opts.storePath, opts.storeKeyPath),
    leasesDir: opts.leasesDir,
    storePath: opts.storePath,
    now: opts.now,
  });
}

module.exports = { createApp, createAppFromFiles, loadPipelines, JobLogHub };
