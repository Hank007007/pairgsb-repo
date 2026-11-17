'use strict';

class PathNormalizationError extends Error {
  constructor(message) {
    super(message);
    this.name = 'PathNormalizationError';
    this.statusCode = 400;
  }
}

function splitRequestTarget(requestTarget) {
  let target = typeof requestTarget === 'string' ? requestTarget : '/';
  const fragment = target.indexOf('#');
  if (fragment !== -1) {
    target = target.slice(0, fragment);
  }
  const query = target.indexOf('?');
  if (query !== -1) {
    target = target.slice(0, query);
  }
  return target;
}

function normalizePath(requestTarget) {
  const rawPath = splitRequestTarget(requestTarget);

  if (/%2f|%5c/i.test(rawPath)) {
    throw new PathNormalizationError('encoded path separator');
  }

  let decoded;
  try {
    decoded = decodeURIComponent(rawPath);
  } catch {
    throw new PathNormalizationError('malformed percent escape');
  }

  if (/%(?:2e|2f|5c|25)/i.test(decoded)) {
    throw new PathNormalizationError('residual escaped path structure');
  }

  if (!decoded.startsWith('/')) {
    decoded = '/' + decoded;
  }

  const segments = [];
  for (const segment of decoded.split('/')) {
    if (segment.length === 0 || segment === '.') {
      continue;
    }
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }

  return '/' + segments.join('/');
}

module.exports = { PathNormalizationError, normalizePath, splitRequestTarget };
