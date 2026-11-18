export const TABLE_NAME = 'items';

export const FIELD_MAP = Object.freeze({
  id: Object.freeze({ column: 'id', sortable: true }),
  name: Object.freeze({ column: 'name', sortable: true }),
  status: Object.freeze({ column: 'status', sortable: true }),
  amount: Object.freeze({ column: 'amount', sortable: true }),
  owner: Object.freeze({ column: 'owner', sortable: true }),
  created_at: Object.freeze({ column: 'created_at', sortable: true }),
  tags: Object.freeze({ column: 'tags', sortable: false }),
  note: Object.freeze({ column: 'note', sortable: false }),
});

export const SORTABLE_FIELDS = Object.freeze(
  Object.keys(FIELD_MAP).filter((name) => FIELD_MAP[name].sortable === true),
);

export function isKnownField(name) {
  return Object.prototype.hasOwnProperty.call(FIELD_MAP, name);
}

export function isSortableField(name) {
  return (
    Object.prototype.hasOwnProperty.call(FIELD_MAP, name) &&
    FIELD_MAP[name].sortable === true
  );
}
