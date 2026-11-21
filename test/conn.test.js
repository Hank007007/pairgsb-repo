"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { Connection, STATES } = require("../src/conn");
const { OPCODES } = require("../src/opcodes");
const { clientFrame } = require("../testlib/frames");

function makeConnection(options = {}) {
  const events = { messages: [], pings: [], pongs: [], closes: [] };
  const connection = new Connection({
    onMessage: (message) => events.messages.push(message),
    onPing: (message) => events.pings.push(message),
    onPong: (message) => events.pongs.push(message),
    onClose: (message) => events.closes.push(message),
    ...options,
  });
  return { connection, events };
}

test("connection delivers a single text message", () => {
  const { connection, events } = makeConnection();

  connection.feed(clientFrame({ payload: "hello gateway" }));

  assert.equal(events.messages.length, 1);
  assert.equal(events.messages[0].type, "text");
  assert.equal(events.messages[0].data, "hello gateway");
});

test("connection delivers a binary message", () => {
  const { connection, events } = makeConnection();
  const bytes = Buffer.from([0xff, 0x00, 0x10]);

  connection.feed(clientFrame({ opcode: OPCODES.BINARY, payload: bytes }));

  assert.equal(events.messages[0].type, "binary");
  assert.deepEqual(events.messages[0].data, bytes);
});

test("connection handles a frame split across two reads", () => {
  const { connection, events } = makeConnection();
  const wire = clientFrame({ payload: "split me" });

  connection.feed(wire.subarray(0, 3));
  assert.equal(events.messages.length, 0);

  connection.feed(wire.subarray(3));

  assert.equal(events.messages.length, 1);
  assert.equal(events.messages[0].data, "split me");
});

test("connection handles two frames in one read", () => {
  const { connection, events } = makeConnection();

  connection.feed(
    Buffer.concat([
      clientFrame({ payload: "one" }),
      clientFrame({ payload: "two" }),
    ]),
  );

  assert.deepEqual(
    events.messages.map((message) => message.data),
    ["one", "two"],
  );
});

test("connection hands pings and pongs to the callbacks", () => {
  const { connection, events } = makeConnection();

  connection.feed(
    Buffer.concat([
      clientFrame({ opcode: OPCODES.PING, payload: "ping" }),
      clientFrame({ opcode: OPCODES.PONG, payload: "pong" }),
    ]),
  );

  assert.equal(events.pings.length, 1);
  assert.equal(events.pings[0].payload.toString("utf8"), "ping");
  assert.equal(events.pongs.length, 1);
  assert.equal(events.pongs[0].payload.toString("utf8"), "pong");
});

test("connection reports a peer close", () => {
  const { connection, events } = makeConnection();
  const payload = Buffer.alloc(2 + 3);
  payload.writeUInt16BE(1000, 0);
  payload.write("bye", 2);

  connection.feed(clientFrame({ opcode: OPCODES.CLOSE, payload }));

  assert.equal(events.closes.length, 1);
  assert.equal(events.closes[0].code, 1000);
  assert.equal(events.closes[0].reason, "bye");
  assert.equal(events.closes[0].byPeer, true);
  assert.equal(connection.state, STATES.CLOSED);
});

test("connection runs the open/closing/closed transition", () => {
  const { connection } = makeConnection();

  assert.equal(connection.state, STATES.OPEN);

  connection.close(1001, "going away");
  assert.equal(connection.state, STATES.CLOSED);
});

test("connection refuses input once it is closed", () => {
  const { connection } = makeConnection();

  connection.close();

  assert.throws(() => connection.feed(clientFrame({ payload: "late" })));
});

test("end is safe when nothing is buffered", () => {
  const { connection, events } = makeConnection();

  connection.end();

  assert.equal(events.messages.length, 0);
});
