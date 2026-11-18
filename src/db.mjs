import { openDatabase } from './pool.mjs';

export const SCHEMA = `
CREATE TABLE IF NOT EXISTS items (
  id         INTEGER PRIMARY KEY,
  name       TEXT    NOT NULL,
  status     TEXT    NOT NULL,
  amount     REAL    NOT NULL,
  owner      TEXT    NOT NULL,
  created_at TEXT    NOT NULL,
  tags       TEXT,
  note       TEXT
);
`;

const SEED_ROWS = [
  [1, 'Widget', 'paid', 19.5, 'alice', '2024-01-05T09:00:00Z', 'hardware', 'first batch'],
  [2, 'Gadget', 'paid', 42, 'bob', '2024-01-07T11:30:00Z', 'hardware', null],
  [3, 'Doohickey', 'pending', 7.25, 'alice', '2024-02-01T08:15:00Z', 'misc', 'needs review'],
  [4, 'Sprocket', 'refunded', 88, 'carol', '2024-02-14T16:45:00Z', 'hardware', null],
  [5, 'Gizmo_50%', 'paid', 31, 'bob', '2024-03-02T10:00:00Z', 'misc', 'promo size'],
  [6, 'Bracket', 'pending', 12.75, 'dave', '2024-03-19T14:20:00Z', 'hardware', null],
  [7, 'Flange', 'paid', 64.4, 'carol', '2024-04-04T07:05:00Z', 'misc', 'restock'],
  [8, 'Coupler', 'refunded', 5.5, 'alice', '2024-04-21T19:10:00Z', 'hardware', 'returned'],
  [9, 'Washer', 'pending', 2.1, 'dave', '2024-05-08T12:00:00Z', 'misc', null],
  [10, 'Bearing', 'paid', 120, 'bob', '2024-05-30T15:35:00Z', 'hardware', 'bulk'],
  [11, 'Shim', 'paid', 0.75, 'carol', '2024-06-11T09:45:00Z', 'misc', null],
  [12, 'Clamp', 'refunded', 17.6, 'dave', '2024-06-27T13:25:00Z', 'hardware', 'damaged'],
];

export function createDatabase(file = ':memory:') {
  const db = openDatabase(file);
  db.exec(SCHEMA);
  seedDatabase(db);
  return db;
}

export function seedDatabase(db) {
  const { count } = db.prepare('SELECT COUNT(*) AS count FROM items').get();
  if (count > 0) {
    return;
  }
  const insert = db.prepare(
    'INSERT INTO items (id, name, status, amount, owner, created_at, tags, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)',
  );
  db.exec('BEGIN');
  try {
    for (const row of SEED_ROWS) {
      insert.run(...row);
    }
    db.exec('COMMIT');
  } catch (error) {
    db.exec('ROLLBACK');
    throw error;
  }
}

export function inMemoryDatabase() {
  return createDatabase(':memory:');
}
