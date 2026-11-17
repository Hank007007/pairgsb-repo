'use strict';

const envelope = require('./envelope');
const codec = require('./codec');
const errors = require('./errors');
const { KEY_ENV, resolveKey } = require('./key');

module.exports = {
  seal: envelope.seal,
  open: envelope.open,
  codec,
  errors,
  KEY_ENV,
  resolveKey,
};
