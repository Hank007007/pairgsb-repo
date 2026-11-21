import test from "node:test";
import assert from "node:assert/strict";

import { issue, read } from "../src/index.js";
import { CookieError } from "../src/errors.js";
import { NOW, NOW_SECONDS, makeClaims, makeKeyRing } from "./helpers.js";

const SERVICE = "profile";
const EXPIRY = NOW_SECONDS + 900;

test("issue produces a four-segment v2 cookie", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims(), { keys });

  const segments = cookie.split(".");
  assert.equal(segments.length, 4);
  assert.equal(segments[0], "v2");
  for (const segment of segments.slice(1)) {
    assert.match(segment, /^[A-Za-z0-9_-]+$/);
  }
});

test("read fills in defaults for claims that were not issued", () => {
  const keys = makeKeyRing();
  const partial = makeClaims();
  delete partial.iss;
  const cookie = issue(partial, { keys });

  const result = read(cookie, { keys, service: SERVICE, now: NOW });

  assert.equal(result.iss, "login");
  assert.equal(result.sub, partial.sub);
});

test("read returns the claims that were issued", () => {
  const keys = makeKeyRing();
  const claims = makeClaims();
  const cookie = issue(claims, { keys });

  const result = read(cookie, { keys, service: SERVICE, now: NOW });

  assert.equal(result.iss, claims.iss);
  assert.equal(result.sub, claims.sub);
  assert.deepEqual(result.aud, claims.aud);
  assert.deepEqual(result.scopes, claims.scopes);
  assert.equal(result.exp, claims.exp);
});

test("claims issued under the previous key can still be read during its window", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims(), { keys, kid: "2026-12" });

  const result = read(cookie, { keys, service: SERVICE, now: NOW });

  assert.equal(result.sub, "user-42");
});

test("a string audience is accepted for its named service", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims({ aud: SERVICE }), { keys });

  const result = read(cookie, { keys, service: SERVICE, now: NOW });

  assert.equal(result.aud, SERVICE);
});

test("a service outside the audience is refused", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims({ aud: ["billing"] }), { keys });

  assert.throws(
    () => read(cookie, { keys, service: SERVICE, now: NOW }),
    (error) => error instanceof CookieError && error.code === "E_AUDIENCE",
  );
});

test("a cookie past its expiry is refused", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims(), { keys });

  assert.throws(
    () => read(cookie, { keys, service: SERVICE, now: (EXPIRY + 1) * 1000 }),
    (error) => error instanceof CookieError && error.code === "E_EXPIRED",
  );
});

test("a cookie read before its not-before time is refused", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims({ nbf: NOW_SECONDS + 600 }), { keys });

  assert.throws(
    () => read(cookie, { keys, service: SERVICE, now: NOW }),
    (error) => error instanceof CookieError && error.code === "E_EXPIRED",
  );
});

test("a body altered after signing is refused", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims(), { keys });
  const segments = cookie.split(".");
  const middle = Math.floor(segments[2].length / 2);
  const current = segments[2][middle];
  const replacement = current === "A" ? "B" : "A";
  const flipped =
    segments[2].slice(0, middle) +
    replacement +
    segments[2].slice(middle + 1);

  assert.throws(
    () =>
      read([segments[0], segments[1], flipped, segments[3]].join("."), {
        keys,
        service: SERVICE,
        now: NOW,
      }),
    (error) => error instanceof CookieError && error.code === "E_MAC",
  );
});

test("an unsupported envelope version is refused", () => {
  const keys = makeKeyRing();
  const cookie = issue(makeClaims(), { keys });
  const rewritten = "v1" + cookie.slice(2);

  assert.throws(
    () => read(rewritten, { keys, service: SERVICE, now: NOW }),
    (error) => error instanceof CookieError && error.code === "E_FORMAT",
  );
});

