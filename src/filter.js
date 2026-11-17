export function escapeFilterValue(value) {
  if (value === null || value === undefined) return '';
  return String(value).replace(/[*()]/g, (character) => {
    return `\\${character.charCodeAt(0).toString(16).padStart(2, '0')}`;
  });
}

export function buildUserFilter(username) {
  return `(uid=${username})`;
}

export function buildOuFilter(ou) {
  return `(ou=${escapeFilterValue(ou)})`;
}

export function buildEqualityFilter(attribute, value) {
  return `(${attribute}=${escapeFilterValue(value)})`;
}

export function buildPresentFilter(attribute) {
  return `(${attribute}=*)`;
}
