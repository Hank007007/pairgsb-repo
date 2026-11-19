'use strict';

const { createProxy } = require('./proxy');
const { loadVault } = require('./vault');

function main() {
  const vaultFile = process.env.EGRESS_VAULT_FILE;
  if (!vaultFile) {
    console.error('EGRESS_VAULT_FILE is required');
    process.exitCode = 2;
    return;
  }

  let vault;
  try {
    vault = loadVault(vaultFile);
  } catch (err) {
    console.error(`egressvault: ${err.message}`);
    process.exitCode = 2;
    return;
  }

  const bind = process.env.EGRESS_BIND || '127.0.0.1';
  const port = Number.parseInt(process.env.EGRESS_PORT || '0', 10);

  const proxy = createProxy({ vault, vaultFile });
  proxy.server.listen(port, bind, () => {
    const address = proxy.server.address();
    const shown = typeof address === 'object' && address !== null ? `${address.address}:${address.port}` : String(address);
    console.log(`egressvault listening on ${shown} with ${vault.rules.length} rule(s)`);
  });

  const shutdown = () => {
    proxy.close().finally(() => process.exit(0));
  };
  process.once('SIGINT', shutdown);
  process.once('SIGTERM', shutdown);
}

if (require.main === module) main();

module.exports = { main };
