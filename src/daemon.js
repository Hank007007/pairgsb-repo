import readline from 'node:readline';

import { cleanup } from './mount.js';
import { renewOnce } from './renew.js';

const RETRY_FLOOR_SECONDS = 1;

function armLeaseRenewal(session, logger) {
  const delayMs = Math.max(RETRY_FLOOR_SECONDS, session.leaseSeconds) * 1000;
  const timer = setTimeout(async () => {
    try {
      await renewOnce(session);
      logger.info('lease renewed');
    } catch (err) {
      logger.error(`lease renewal failed: ${err.message}`);
    }
    armLeaseRenewal(session, logger);
  }, delayMs);

  if (typeof timer.unref === 'function') timer.unref();
  return timer;
}

export async function serve(session, options = {}) {
  const {
    input = process.stdin,
    output = process.stdout,
    logger = console,
  } = options;

  const timer = armLeaseRenewal(session, logger);

  const onTerminate = () => {
    process.exit(0);
  };
  process.once('SIGTERM', onTerminate);

  try {
    const lines = readline.createInterface({ input, crlfDelay: Infinity });

    for await (const raw of lines) {
      const command = raw.trim();
      if (command.length === 0) continue;

      if (command === 'renew') {
        try {
          await renewOnce(session);
          output.write('renewed\n');
        } catch (err) {
          output.write(`renew failed: ${err.message}\n`);
        }
        continue;
      }

      if (command === 'stop') {
        output.write('stopped\n');
        return;
      }

      output.write(`unknown command: ${command}\n`);
    }
  } finally {
    clearTimeout(timer);
    process.removeListener('SIGTERM', onTerminate);
  }

  cleanup(session.dir);
}
