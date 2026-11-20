export const ROLES = ['owner', 'editor', 'commenter', 'viewer'];

export function isValidRole(role) {
  return typeof role === 'string' && ROLES.includes(role);
}

/**
 * Numeric rank for a role: owner 3, editor 2, commenter 1, viewer 0.
 * Unknown roles rank below every valid role.
 */
export function rankOf(role) {
  const index = ROLES.indexOf(role);
  return index === -1 ? -1 : ROLES.length - 1 - index;
}

/**
 * Whether the granted role covers the required role.
 */
export function atLeast(granted, required) {
  return granted >= required;
}

/**
 * Highest role in a list of role names, or null when none is valid.
 */
export function maxRole(roles) {
  let best = null;
  for (const role of roles) {
    if (!isValidRole(role)) continue;
    if (best === null || rankOf(role) > rankOf(best)) {
      best = role;
    }
  }
  return best;
}
