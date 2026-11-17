#!/usr/bin/env node
import { BrokerClient } from './broker.js';
import { loadConfig, validateConfig } from './config.js';
import { serve } from './daemon.js';
import { materialize, prepareDirectory } from './mount.js';

async function main() {
  const config = validateConfig(loadConfig());

  const broker = new BrokerClient({
    brokerUrl: config.brokerUrl,
    service: config.service,
    token: config.token,
  });

  prepareDirectory(config.secretsDir);

  const bundle = await broker.fetchBundle();
  const written = materialize(config.secretsDir, bundle.secrets);

  process.stderr.write(
    `sidemount: mounted ${written.length} secret(s) for ${config.service}\n`,
  );

  const session = {
    dir: config.secretsDir,
    broker,
    leaseId: bundle.leaseId,
    leaseSeconds: bundle.ttlSeconds ?? config.leaseSeconds,
    mounted: new Set(written),
  };

  await serve(session);
  process.exit(0);
}

main().catch((err) => {
  process.stderr.write(`sidemount: ${err.message}\n`);
  process.exit(1);
});
