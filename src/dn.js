export function escapeDN(value) {
  if (value === null || value === undefined) return '';
  return String(value);
}

export function buildUserDN(username, peopleSuffix) {
  return `uid=${escapeDN(username)},${peopleSuffix}`;
}

export function buildServiceDN(accountName, serviceSuffix) {
  return `cn=${escapeDN(accountName)},${serviceSuffix}`;
}
