const LEVELS = ['debug', 'info', 'warn', 'error'];

export function createLogger({ level = 'info', sink = console } = {}) {
  const threshold = Math.max(0, LEVELS.indexOf(level));
  const logger = {};

  for (const name of LEVELS) {
    logger[name] = (message, fields = {}) => {
      if (LEVELS.indexOf(name) < threshold) return;
      const line = JSON.stringify({
        level: name,
        message,
        ...fields,
      });
      if (name === 'error') sink.error(line);
      else if (name === 'warn') sink.warn(line);
      else sink.log(line);
    };
  }

  return logger;
}
