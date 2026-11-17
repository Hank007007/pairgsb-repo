const DEFAULT_BROKER_URL = 'http://127.0.0.1:8200';
const DEFAULT_SECRETS_DIR = '/run/secrets';
const DEFAULT_LEASE_SECONDS = 300;

export function loadConfig(env = process.env) {
  return {
    service: env.SIDEMOUNT_SERVICE ?? '',
    brokerUrl: env.SIDEMOUNT_BROKER_URL ?? DEFAULT_BROKER_URL,
    token: env.SIDEMOUNT_IDENTITY_TOKEN ?? '',
    secretsDir: env.SIDEMOUNT_SECRETS_DIR ?? DEFAULT_SECRETS_DIR,
    leaseSeconds: Number(env.SIDEMOUNT_LEASE_SECONDS ?? DEFAULT_LEASE_SECONDS),
  };
}

export function validateConfig(config) {
  if (typeof config.service !== 'string' || config.service.length === 0) {
    throw new Error('SIDEMOUNT_SERVICE is required');
  }

  let url;
  try {
    url = new URL(config.brokerUrl);
  } catch {
    throw new Error('SIDEMOUNT_BROKER_URL must be an absolute URL');
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('SIDEMOUNT_BROKER_URL must use http or https');
  }

  if (typeof config.secretsDir !== 'string' || config.secretsDir.length === 0) {
    throw new Error('SIDEMOUNT_SECRETS_DIR must be a path');
  }

  if (!Number.isFinite(config.leaseSeconds) || config.leaseSeconds <= 0) {
    throw new Error('SIDEMOUNT_LEASE_SECONDS must be a positive number');
  }

  return config;
}
