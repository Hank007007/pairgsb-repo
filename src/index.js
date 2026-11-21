'use strict';

module.exports = {
  ...require('./protocol/manifest'),
  forwarder: {
    ...require('./forwarder/spool'),
    ...require('./forwarder/cursor'),
    ...require('./forwarder/client'),
  },
  collector: {
    ...require('./collector/verify'),
    ...require('./collector/dedup'),
    ...require('./collector/gap'),
    ...require('./collector/store'),
    ...require('./collector/server'),
  },
};
