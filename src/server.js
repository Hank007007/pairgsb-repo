'use strict';

const http = require('node:http');
const { createApp } = require('./app');

function createServer(options = {}) {
  const app = createApp(options);
  return http.createServer((req, res) => {
    app(req, res);
  });
}

module.exports = { createServer };
