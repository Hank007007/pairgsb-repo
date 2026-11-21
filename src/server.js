'use strict';

const http = require('node:http');

const { createApp } = require('./http/app');

function createServer(options = {}) {
  const app = createApp(options);
  const server = http.createServer((req, res) => {
    app.handle(req, res).catch((err) => {
      if (!res.headersSent) {
        res.writeHead(500, { 'content-type': 'application/scim+json' });
        res.end(JSON.stringify({
          schemas: ['urn:ietf:params:scim:api:messages:2.0:Error'],
          status: '500',
          detail: err && err.message ? err.message : 'internal error',
        }));
      }
    });
  });
  return { server, app };
}

function listen(server, port = 0) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', () => resolve(server.address()));
  });
}

module.exports = { createServer, listen };
