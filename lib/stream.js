'use strict';

// Log stream forwarding.
//
// A job's raw stdout/stderr is written into this transform and the masked
// bytes are read from the other side.

const { Transform: NodeTransform } = require('node:stream');
const { mask } = require('./mask');

class Transform extends NodeTransform {
  constructor(secrets, options) {
    super(options);
    this.secrets = Array.isArray(secrets) ? secrets.slice() : [];
  }

  _transform(chunk, encoding, callback) {
    try {
      const text = chunk.toString('utf8');
      const pieces = text.split('\n');

      for (let i = 0; i < pieces.length; i += 1) {
        if (i === pieces.length - 1) {
          if (pieces[i] !== '') this.push(mask(this.secrets, pieces[i]));
        } else {
          this.push(mask(this.secrets, pieces[i] + '\n'));
        }
      }

      callback();
    } catch (error) {
      callback(error);
    }
  }

  _flush(callback) {
    callback();
  }
}

module.exports = { Transform };
