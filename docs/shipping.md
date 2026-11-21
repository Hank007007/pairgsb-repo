# Shipping: source file, cursor, spool

## §1 Model

The relay owns one **source file** per node and one **spool** beside it. The
source file is append-only and is written by the business process. The relay is
the only writer of the spool and the cursor.

Events leave the relay only after the collector has acknowledged them (§3 of
`delivery.md`). Until then they remain recoverable from the spool, across
process restarts.

## §2 Source file and reading

1. Every line of the source file is one JSON object with at least `seq`, `ts`,
   and `payload`. `seq` is a monotonically increasing integer assigned by the
   producer; it is unique per source file.
2. A line is **complete** only if it is terminated by `\n`. A trailing fragment
   without a terminator is not an event yet, even if it parses as JSON.
3. A complete line that cannot be parsed as JSON, or that lacks a required
   field, is a **malformed line**. It must never be silently skipped: the reader
   must either surface an error for it or carry it forward unchanged so that a
   later restart still sees it. In no case may the cursor move past it.
4. The reader returns complete lines in file order and reports the byte offset
   after the last complete line it returned.

## §3 Cursor

1. The cursor is the position in the source file up to which events have been
   **durably accepted by the collector**. It is not "the position last read".
2. The cursor advances only as the result of a verified acknowledgement
   (`delivery.md` §3).
3. Cursor persistence and spool persistence together form one unit: after any
   crash, a restart must observe either (a) cursor not yet advanced and the
   events still present in the spool, or (b) cursor advanced and the events
   durably recorded as delivered/gapped. It must never be possible to observe a
   cursor that has moved past events that are absent from the spool and absent
   from the delivery record. Durability means the data has been forced to
   stable storage before the cursor that covers it is considered committed.

## §4 Spool

1. The spool is an append-only queue of events that have been read but not yet
   acknowledged. Appends are atomic per record: a partially written trailing
   record is distinguishable from a complete one.
2. On load, every complete, valid record is preserved in order. A corrupt or
   truncated record must be reported or retained; it must not be dropped
   silently. If the spool cannot be loaded safely, the relay must fail loudly
   rather than start with a queue that has quietly lost entries.
3. Records stay in the order defined by `batch-format.md` §3. The spool never
   reorders records.
4. A record leaves the spool only when the batch containing it has been
   acknowledged by the collector.

## §5 Capacity and gaps

1. The spool has a maximum record count. When appending would exceed it, the
   **oldest** records are dropped so the newest are kept.
2. Dropping is data loss and must be visible. Every dropped record is replaced
   by, or accompanied by, a **gap record** in the event stream sent to the
   collector. A gap record identifies the range of dropped `seq` values, the
   count, and the reason (`capacity`).
3. Gap records are themselves ordered and digested like ordinary records
   (`batch-format.md` §4); they count as shipped data once acknowledged.
4. A gap record is not a substitute for delivery of the dropped events; it is a
   declaration that they could not be delivered. Dropped events are never
   retried.
5. Trimming is deterministic: given the same sequence of appends and the same
   capacity, the same records are dropped and the same gaps are produced.
