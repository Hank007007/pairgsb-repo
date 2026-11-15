#!/usr/bin/env node
import process from 'node:process';

import { getKey } from './hmac.js';
import { verifyChain } from './verify.js';

const [, , command, dir = 'audit-data'] = process.argv;

if (command !== 'verify') {
  console.error('用法：node src/cli.js verify <目录>');
  process.exit(2);
}

const result = await verifyChain(dir, getKey());
console.log(JSON.stringify(result, null, 2));
process.exit(result.ok ? 0 : 1);
