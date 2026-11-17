'use strict';

const { createServer } = require('./server');

const PORT = Number.parseInt(process.env.PORT || '8080', 10);
const HOST = process.env.HOST || '127.0.0.1';

const server = createServer();

server.listen(PORT, HOST, () => {
  const address = server.address();
  const port = typeof address === 'object' && address ? address.port : PORT;
  process.stdout.write(`routeward listening on http://${HOST}:${port}\n`);
});

function shutdown(signal) {
  process.stdout.write(`routeward received ${signal}, shutting down\n`);
  server.close(() => process.exit(0));
}

process.on('SIGINT', () => shutdown('SIGINT'));
process.on('SIGTERM', () => shutdown('SIGTERM'));
