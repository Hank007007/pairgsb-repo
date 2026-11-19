import test from 'node:test';
import assert from 'node:assert/strict';

import {
  filterCovers,
  isConcreteTopic,
  isValidFilter,
  topicMatchesFilter,
} from '../src/acl/match.js';

test('topicMatchesFilter matches a literal filter exactly', () => {
  assert.equal(topicMatchesFilter('sensors/a/state', 'sensors/a/state'), true);
  assert.equal(topicMatchesFilter('sensors/a/state', 'sensors/a/other'), false);
  assert.equal(topicMatchesFilter('sensors/a/state', 'sensors/b/state'), false);
});

test('topicMatchesFilter handles + and trailing #', () => {
  assert.equal(topicMatchesFilter('sensors/+/state', 'sensors/a/state'), true);
  assert.equal(topicMatchesFilter('sensors/+/state', 'sensors/a/b/state'), false);
  assert.equal(topicMatchesFilter('sensors/#', 'sensors/a/b'), true);
  assert.equal(topicMatchesFilter('#', 'anything/at/all'), true);
});

test('isValidFilter and isConcreteTopic classify a literal filter', () => {
  assert.equal(isValidFilter('sensors/a/state'), true);
  assert.equal(isValidFilter(''), false);
  assert.equal(isConcreteTopic('sensors/a/state'), true);
  assert.equal(isConcreteTopic('sensors/a/#'), false);
});

test('filterCovers matches a literal rule filter', () => {
  assert.equal(filterCovers('sensors/a/state', 'sensors/a/state'), true);
  assert.equal(filterCovers('sensors/a/state', 'sensors/a/other'), false);
  assert.equal(filterCovers('farm/a/state', 'sensors/a/state'), false);
});

test('filterCovers lets a trailing # cover the subtree', () => {
  assert.equal(filterCovers('sensors/#', 'sensors/a'), true);
  assert.equal(filterCovers('sensors/#', 'sensors/a/b'), true);
  assert.equal(filterCovers('#', 'anything'), true);
});
