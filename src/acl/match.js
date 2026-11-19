/**
 * Topic filter helpers shared by the ACL and broker layers.
 *
 * Filters are split on `/`. An empty level is a legal, meaningful level and
 * never equals a wildcard. Matching is case sensitive.
 */

/**
 * True when `filter` is a syntactically well formed topic filter.
 *
 * Wildcards must occupy a whole level, and `#` may only appear as the final
 * level.
 */
export function isValidFilter(filter) {
  if (typeof filter !== 'string' || filter.length === 0) {
    return false;
  }

  const levels = filter.split('/');
  for (let index = 0; index < levels.length; index += 1) {
    const level = levels[index];
    if (level === '#') {
      if (index !== levels.length - 1) {
        return false;
      }
      continue;
    }
    if (level.includes('#')) {
      return false;
    }
    if (level.includes('+') && level !== '+') {
      return false;
    }
  }

  return true;
}

/**
 * True when `topic` is a concrete topic name: no wildcards.
 */
export function isConcreteTopic(topic) {
  if (!isValidFilter(topic)) {
    return false;
  }
  const levels = topic.split('/');
  return !levels.includes('+') && !levels.includes('#');
}

/**
 * True when the client supplied filter is a well formed subscription filter,
 * including the `$share/<group>/<filter>` form.
 */
export function isValidSubscriptionFilter(filter) {
  if (typeof filter !== 'string' || filter.length === 0) {
    return false;
  }

  const levels = filter.split('/');
  if (levels[0] !== '$share') {
    return isValidFilter(filter);
  }
  if (levels.length < 3) {
    return false;
  }

  const group = levels[1];
  if (group.length === 0 || group.includes('+') || group.includes('#')) {
    return false;
  }

  const inner = levels.slice(2).join('/');
  if (inner.length === 0) {
    return false;
  }
  if (inner.split('/')[0] === '$share') {
    return false;
  }

  return isValidFilter(inner);
}

/**
 * True when a concrete topic name is selected by a subscription filter.
 */
export function topicMatchesFilter(filter, topic) {
  if (typeof filter !== 'string' || typeof topic !== 'string') {
    return false;
  }

  const filterLevels = filter.split('/');
  const topicLevels = topic.split('/');

  for (let index = 0; index < filterLevels.length; index += 1) {
    const level = filterLevels[index];

    if (level === '#') {
      return true;
    }
    if (index >= topicLevels.length) {
      return false;
    }
    if (level === '+') {
      continue;
    }
    if (level !== topicLevels[index]) {
      return false;
    }
  }

  return filterLevels.length === topicLevels.length;
}

/**
 * True when the rule filter applies to the supplied subject.
 *
 * `subject` is either a concrete topic name (publish authorization) or
 * another topic filter (subscribe authorization).
 */
export function filterCovers(ruleFilter, subject) {
  if (typeof ruleFilter !== 'string' || typeof subject !== 'string') {
    return false;
  }
  if (ruleFilter === '#') {
    return true;
  }

  const prefix = ruleFilter.endsWith('/#') ? ruleFilter.slice(0, -2) : ruleFilter;
  return subject.startsWith(prefix) || prefix.startsWith(subject);
}
