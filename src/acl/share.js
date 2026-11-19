/**
 * Split a `$share/<group>/<filter>` subscription into its parts.
 * Returns `null` for ordinary subscriptions or malformed wrappers.
 */
export function splitSharedSubscription(filter) {
  if (typeof filter !== 'string') {
    return null;
  }
  const levels = filter.split('/');
  if (levels[0] !== '$share' || levels.length < 3) {
    return null;
  }
  const group = levels[1];
  const inner = levels.slice(2).join('/');
  if (group.length === 0 || inner.length === 0) {
    return null;
  }
  return { group, filter: inner };
}
