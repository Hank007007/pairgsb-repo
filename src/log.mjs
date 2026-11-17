import { SecretStore } from './secrets.mjs';

export function createLogger(options = {}) {
  const secrets = options.secrets instanceof SecretStore ? options.secrets : new SecretStore();
  const stdout = options.stdout ?? process.stdout;
  const stderr = options.stderr ?? process.stderr;

  const emit = (stream, message) => {
    const text = secrets.mask(String(message));
    stream.write(text.endsWith('\n') ? text : `${text}\n`);
  };

  return {
    secrets,
    mask(text) {
      return secrets.mask(text);
    },
    info(message) {
      emit(stdout, message);
    },
    warn(message) {
      emit(stderr, `warning: ${message}`);
    },
    error(message) {
      emit(stderr, `error: ${message}`);
    },
    stepOutput(stream, line) {
      emit(stream === 'stderr' ? stderr : stdout, line);
    },
  };
}

export function createMemoryLogger(options = {}) {
  const secrets = options.secrets instanceof SecretStore ? options.secrets : new SecretStore();
  const out = [];
  const err = [];
  const stream = {
    write(chunk) {
      const text = secrets.mask(String(chunk));
      out.push(text.endsWith('\n') ? text : `${text}\n`);
      return true;
    },
  };
  const errorStream = {
    write(chunk) {
      const text = secrets.mask(String(chunk));
      err.push(text.endsWith('\n') ? text : `${text}\n`);
      return true;
    },
  };
  const logger = createLogger({ secrets, stdout: stream, stderr: errorStream });
  return {
    ...logger,
    get stdout() {
      return out.join('');
    },
    get stderr() {
      return err.join('');
    },
    get output() {
      return `${out.join('')}${err.join('')}`;
    },
  };
}
