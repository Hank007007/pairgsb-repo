'use strict';

/**
 * End-to-end demonstration of the envelope wire format.
 *
 * Each case prints the envelope fragment it uses and what the public API does
 * with it. Run with: node scripts/demo.js
 */

const { createHmac } = require('node:crypto');

const {
  seal,
  open,
  codec,
  errors,
} = require('../src');

const KEY = 'demo-key-not-for-production-use-0000';

function describe(error) {
  if (error instanceof errors.EnvelopeError) {
    return `${error.name} (${error.code}): ${error.message}`;
  }
  return `${error.name}: ${error.message}`;
}

function attempt(label, token) {
  try {
    const session = open(token, { key: KEY });
    console.log(`  accepted  ${label}`);
    return { ok: true, session };
  } catch (error) {
    console.log(`  rejected  ${label} -> ${describe(error)}`);
    return { ok: false, error };
  }
}

function payloadOf(token) {
  return token.split('.')[0];
}

function macOf(token) {
  return token.split('.')[1];
}

function forge(payloadText, mac) {
  return `${codec.b64urlEncode(Buffer.from(payloadText))}.${mac}`;
}

function section(title) {
  console.log(`\n${title}`);
  console.log('-'.repeat(title.length));
}

const SESSION = {
  sub: 'user-1234',
  roles: ['reader', 'writer'],
  profile: { displayName: 'Ada Lovelace', locale: 'en-GB' },
};

section('1. round trip');
const token = seal(SESSION, { key: KEY });
console.log(`  token:  ${token}`);
console.log(`  length: ${token.length} bytes`);
const restored = open(token, { key: KEY });
console.log(`  session: ${JSON.stringify(restored)}`);

section('2. payload edited, signature left as it was');
{
  const payload = payloadOf(token);
  const mac = macOf(token);
  const original = codec.b64urlDecode(payload);

  const edited = Buffer.from(original);
  edited[edited.indexOf(Buffer.from('user-1234'))] = 'U'.charCodeAt(0);
  attempt('one byte changed inside a string value', `${codec.b64urlEncode(edited)}.${mac}`);

  attempt('payload replaced with invalid JSON', forge('{not json', mac));
  attempt('payload replaced with a JSON array', forge('["a","b"]', mac));
  attempt('payload replaced with an empty object', forge('{}', mac));

  const spaced = original
    .toString('utf8')
    .replace('{', '{   ')
    .replace('"roles"', '"roles"  ');
  attempt('whitespace added between members', forge(spaced, mac));
}

section('3. session reconstruction');
{
  const hostile = '{"sub":"user-1234","__proto__":{"isAdmin":true}}';
  const hostileToken = seal(JSON.parse(hostile), { key: KEY });
  console.log(`  payload: ${hostile}`);
  attempt('envelope carrying __proto__', hostileToken);

  const session = codec.decodePayload(codec.b64urlDecode(payloadOf(hostileToken)));
  console.log('  reconstructing the same payload bytes directly:');
  console.log(`    Object.getPrototypeOf(session) === Object.prototype: ${Object.getPrototypeOf(session) === Object.prototype}`);
  console.log(`    Object.getPrototypeOf(session) === null: ${Object.getPrototypeOf(session) === null}`);
  console.log(`    session.isAdmin: ${JSON.stringify(session.isAdmin)}`);
  console.log(`    Object.hasOwn(session, 'isAdmin'): ${Object.hasOwn(session, 'isAdmin')}`);
  console.log(`    Object.hasOwn(session, '__proto__'): ${Object.hasOwn(session, '__proto__')}`);
  console.log(`    JSON.stringify(session): ${JSON.stringify(session)}`);

  const nested = codec.decodePayload(Buffer.from('{"profile":{"__proto__":{"role":"admin"}}}'));
  console.log(`    nested object prototype is Object.prototype: ${Object.getPrototypeOf(nested.profile) === Object.prototype}`);
  console.log(`    nested profile.role: ${JSON.stringify(nested.profile.role)}`);
}

section('4. base64url segment forms');
{
  const payload = payloadOf(token);
  const mac = macOf(token);
  console.log(`  payload segment length: ${payload.length}`);

  attempt('payload segment with "=" padding', `${payload}=.${mac}`);
  attempt('payload segment with "!"', `${payload}!.${mac}`);
  attempt('payload segment with a space', `${payload} .${mac}`);
  attempt('payload segment extended to a length of 1 modulo 4', `${payload}A.${mac}`);
  attempt('signature segment with "=" padding', `${payload}.${mac}==`);
  attempt('signature segment with "!"', `${payload}.${mac}!`);

  const punctuated = seal({ sub: '~~~' }, { key: KEY });
  console.log(`  payload segment: ${payloadOf(punctuated)}`);
  attempt('segment containing "-", rewritten as "+"', `${payloadOf(punctuated).replace(/-/g, '+')}.${macOf(punctuated)}`);
  attempt('segment containing "-", left as is', punctuated);
}

section('5. payload bytes that are not valid UTF-8');
{
  const payload = payloadOf(token);
  const mac = macOf(token);
  const original = codec.b64urlDecode(payload);
  const broken = Buffer.from(original);
  broken[broken.indexOf(Buffer.from('Ada'))] = 0xff;

  attempt('invalid UTF-8 byte in payload, signature left as it was', `${codec.b64urlEncode(broken)}.${mac}`);

  const signedOverBrokenBytes = createHmac('sha256', KEY).update(broken).digest();
  attempt(
    'invalid UTF-8 payload, signature computed over those very bytes',
    `${codec.b64urlEncode(broken)}.${codec.b64urlEncode(signedOverBrokenBytes)}`
  );

  const reconstructed = codec.decodePayload(broken);
  console.log('  reconstructing the same bytes directly:');
  console.log(`    profile.displayName: ${JSON.stringify(reconstructed.profile.displayName)}`);
  console.log(`    contains a replacement character: ${reconstructed.profile.displayName.includes('�')}`);
}

section('6. declared size limits');
{
  const MAX_BYTES = 8192;
  const MAX_DEPTH = 12;
  const MAX_KEYS = 256;
  console.log(`  limits: ${MAX_BYTES} bytes, depth ${MAX_DEPTH}, ${MAX_KEYS} member names`);

  const big = { sub: 'user-1234', note: 'x'.repeat(MAX_BYTES) };
  const bigToken = seal(big, { key: KEY });
  console.log(`  oversized envelope is ${bigToken.length} bytes`);
  attempt(`envelope over ${MAX_BYTES} bytes`, bigToken);

  let deep = { value: 'leaf' };
  for (let level = 0; level < MAX_DEPTH + 4; level += 1) {
    deep = { child: deep };
  }
  attempt(`payload nested deeper than ${MAX_DEPTH}`, seal(deep, { key: KEY }));

  const wide = {};
  for (let index = 0; index < MAX_KEYS + 16; index += 1) {
    wide[`field${index}`] = index;
  }
  attempt(`payload with more than ${MAX_KEYS} member names`, seal(wide, { key: KEY }));
}

console.log('\ndemo complete.');
