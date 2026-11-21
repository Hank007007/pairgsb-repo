'use strict';

const { createServer, listen } = require('./server');
const { issueToken } = require('./auth/token');
const { seed } = require('./seed');

async function main() {
  const { server, app } = createServer();
  seed(app);
  const address = await listen(server, Number(process.env.PORT || 0));
  const base = `http://127.0.0.1:${address.port}`;

  const tokens = {
    acme: issueToken({
      sub: 'hr-acme',
      tenant: 'acme',
      scopes: [
        'attr:profile:read', 'attr:profile:write',
        'attr:groups:read', 'attr:groups:write',
        'attr:roles:read', 'attr:roles:write',
        'attr:active:read', 'attr:active:write',
      ],
    }),
    globex: issueToken({
      sub: 'hr-globex',
      tenant: 'globex',
      scopes: ['attr:profile:read', 'attr:profile:write'],
    }),
  };

  process.stdout.write(`scimfence listening on ${base}\n`);
  process.stdout.write(`acme token:   ${tokens.acme}\n`);
  process.stdout.write(`globex token: ${tokens.globex}\n`);

  const shutdown = () => {
    server.close(() => process.exit(0));
  };
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

main().catch((err) => {
  process.stderr.write(`${err && err.stack ? err.stack : err}\n`);
  process.exit(1);
});
