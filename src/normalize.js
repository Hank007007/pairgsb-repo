'use strict';

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
  let decoded;
  try {
    decoded = decodeURIComponent(rawPath);
  } catch {
    decoded = rawPath;
  }
  if (decoded.length === 0) {
    return '/';
  }
  return decoded;
}

module.exports = { normalizePath, splitRequestTarget };
