import test from 'node:test';
import assert from 'node:assert/strict';

import { buildQuery, quoteIdent } from '../src/builder.mjs';
import { applyPage } from '../src/plan.mjs';
import { run } from '../src/pool.mjs';
import { inMemoryDatabase } from '../src/db.mjs';
import { handleQuery } from '../src/service.mjs';

function names(rows) {
  return rows.map((row) => row.name).sort();
}

function namesInOrder(rows) {
  return rows.map((row) => row.name);
}

function plain(rows) {
  return rows.map((row) => ({ ...row }));
}

function freshDb() {
  return inMemoryDatabase();
}

test('buildQuery composes a single SELECT over the items table', () => {
  const { sql } = buildQuery({});
  assert.match(sql, /^SELECT /);
  assert.match(sql, /FROM\s+"?items"?\b/);
  const statements = sql.split(';').filter((part) => part.trim().length > 0);
  assert.equal(statements.length, 1);
});

test('buildQuery always carries a paging window', () => {
  const { sql } = buildQuery({});
  assert.match(sql, /\bLIMIT\b/i);
  assert.match(sql, /\bOFFSET\b/i);
});

test('an empty request returns every row of the seed data', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {});
  assert.equal(status, 200);
  assert.equal(body.rows.length, 12);
});

test('applyPage appends a paging window to the statement', () => {
  const sql = applyPage('SELECT id FROM items ORDER BY id', { limit: 4, offset: 5 });
  assert.match(sql, /LIMIT 4\b/i);
  assert.match(sql, /OFFSET 5\b/i);
});

test('applyPage treats a missing window as the defaults', () => {
  const sql = applyPage('SELECT id FROM items ORDER BY id', {});
  assert.match(sql, /LIMIT 50\b/i);
  assert.match(sql, /OFFSET 0\b/i);
});

test('quoteIdent produces a usable column reference', () => {
  const db = freshDb();
  const column = quoteIdent('name');
  const row = db.prepare(`SELECT ${column} AS value FROM items WHERE id = 1`).get();
  assert.equal(row.value, 'Widget');
});

test('an eq filter returns the matching rows', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'status', op: 'eq', value: 'paid' },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), [
    'Bearing',
    'Flange',
    'Gadget',
    'Gizmo_50%',
    'Shim',
    'Widget',
  ]);
});

test('a ne filter excludes the matching rows', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'status', op: 'ne', value: 'paid' },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Bracket', 'Clamp', 'Coupler', 'Doohickey', 'Sprocket', 'Washer']);
});

test('a null value with eq selects the rows where the column is null', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'note', op: 'eq', value: null },
  });
  assert.equal(status, 200);
  assert.equal(body.rows.length, 5);
  for (const row of body.rows) {
    assert.equal(row.note, null);
  }
});

test('an in filter matches the listed identifiers', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'id', op: 'in', value: [1, 5, 9] },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Gizmo_50%', 'Washer', 'Widget']);
});

test('comparison operators on a numeric field select the right rows', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'amount', op: 'gte', value: 64.4 },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Bearing', 'Flange', 'Sprocket']);
});

test('a like filter performs a contains match', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'name', op: 'like', value: 'dg' },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Gadget', 'Widget']);
});

test('a like filter can match a whole word', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 'owner', op: 'like', value: 'car' },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Flange', 'Shim', 'Sprocket']);
});

test('is_null and not_null select by the presence of a value', () => {
  const db = freshDb();
  const missing = handleQuery(db, {
    filter: { field: 'note', op: 'is_null' },
  });
  assert.equal(missing.status, 200);
  assert.equal(missing.body.rows.length, 5);
  for (const row of missing.body.rows) {
    assert.equal(row.note, null);
  }

  const present = handleQuery(db, {
    filter: { field: 'note', op: 'not_null' },
  });
  assert.equal(present.status, 200);
  assert.equal(present.body.rows.length, 7);
  for (const row of present.body.rows) {
    assert.notEqual(row.note, null);
  }
});

test('and and or group their children', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: {
      and: [
        { field: 'status', op: 'eq', value: 'paid' },
        {
          or: [
            { field: 'owner', op: 'eq', value: 'alice' },
            { field: 'owner', op: 'eq', value: 'bob' },
          ],
        },
      ],
    },
  });
  assert.equal(status, 200);
  assert.deepEqual(names(body.rows), ['Bearing', 'Gadget', 'Gizmo_50%', 'Widget']);
});

test('a sort key orders the rows by that field', () => {
  const db = freshDb();
  const ascending = handleQuery(db, {
    sort: { field: 'amount', direction: 'asc' },
    limit: 3,
  });
  assert.equal(ascending.status, 200);
  assert.deepEqual(namesInOrder(ascending.body.rows), ['Shim', 'Washer', 'Coupler']);

  const descending = handleQuery(db, {
    sort: { field: 'amount', direction: 'desc' },
    limit: 3,
  });
  assert.equal(descending.status, 200);
  assert.deepEqual(namesInOrder(descending.body.rows), ['Bearing', 'Sprocket', 'Flange']);
});

test('several sort keys are applied in order', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    sort: [
      { field: 'owner', direction: 'asc' },
      { field: 'id', direction: 'desc' },
    ],
    limit: 4,
  });
  assert.equal(status, 200);
  assert.deepEqual(
    body.rows.map((row) => row.id),
    [8, 3, 1, 10],
  );
});

test('limit and offset select a page of the result', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    sort: { field: 'id', direction: 'asc' },
    limit: 4,
    offset: 5,
  });
  assert.equal(status, 200);
  assert.deepEqual(
    body.rows.map((row) => row.id),
    [6, 7, 8, 9],
  );
});

test('keys other than filter, sort, limit and offset are ignored', () => {
  const db = freshDb();
  const result = handleQuery(db, {
    filter: { field: 'status', op: 'eq', value: 'pending' },
    debug: true,
    cache: 'no',
  });
  assert.equal(result.status, 200);
  assert.deepEqual(names(result.body.rows), ['Bracket', 'Doohickey', 'Washer']);
});

test('a request body that is not an object is rejected as invalid', () => {
  const db = freshDb();
  for (const body of [null, [], 'filter', 42]) {
    const { status, body: response } = handleQuery(db, body);
    assert.equal(status, 400);
    assert.equal(response.error.code, 'invalid_query');
  }
});

test('a malformed filter is rejected as invalid', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    filter: { field: 7, op: 'eq', value: 'x' },
  });
  assert.equal(status, 400);
  assert.equal(body.error.code, 'invalid_query');
});

test('a malformed sort direction is rejected', () => {
  const db = freshDb();
  const { status, body } = handleQuery(db, {
    sort: { field: 'id', direction: 'sideways' },
  });
  assert.equal(status, 400);
  assert.equal(body.error.code, 'invalid_sort');
});

test('run returns the rows of a plain SELECT', () => {
  const db = freshDb();
  const rows = run(db, 'SELECT id, name FROM items WHERE id = 2');
  assert.deepEqual(plain(rows), [{ id: 2, name: 'Gadget' }]);
});

test('run accepts a statement with a trailing semicolon', () => {
  const db = freshDb();
  const rows = run(db, 'SELECT COUNT(*) AS count FROM items;');
  assert.equal(rows[0].count, 12);
});
