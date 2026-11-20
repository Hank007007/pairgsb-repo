"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const { sanitize } = require("../lib");
const { render, withHighlight } = require("../lib/render");

const snapshots = JSON.parse(
  fs.readFileSync(path.join(__dirname, "snapshots", "comments.json"), "utf8")
);

test("script elements are removed with their contents", () => {
  assert.equal(
    sanitize('<p>before</p><script>alert(1)</script><p>after</p>'),
    "<p>before</p><p>after</p>"
  );
});

test("inline event handler attributes are removed", () => {
  assert.equal(
    sanitize('<p onclick="alert(1)">text</p>'),
    "<p>text</p>"
  );
  assert.equal(
    sanitize('<img src="/a.png" onerror="alert(1)">'),
    '<img src="/a.png">'
  );
});

test("whitelisted inline formatting survives", () => {
  assert.equal(
    sanitize("<p>hello <em>world</em> <strong>again</strong><br>next</p>"),
    "<p>hello <em>world</em> <strong>again</strong><br>next</p>"
  );
});

test("links keep their href and title", () => {
  assert.equal(
    sanitize('<a href="/posts/1" title="first post">read</a>'),
    '<a href="/posts/1" title="first post">read</a>'
  );
  assert.equal(
    sanitize('<a href="https://example.com/x">x</a>'),
    '<a href="https://example.com/x">x</a>'
  );
  assert.equal(
    sanitize('<a href="mailto:user@example.com">mail</a>'),
    '<a href="mailto:user@example.com">mail</a>'
  );
});

test("query separators in href survive as escaped text", () => {
  assert.equal(
    sanitize('<a href="/x?a=1&amp;b=2">x</a>'),
    '<a href="/x?a=1&amp;b=2">x</a>'
  );
});

test("images keep src, alt, width, and height", () => {
  assert.equal(
    sanitize('<img src="/img/a.png" alt="A" width="100" height="50">'),
    '<img src="/img/a.png" alt="A" width="100" height="50">'
  );
});

test("lists and block quotes survive", () => {
  assert.equal(
    sanitize("<blockquote><p>quote</p></blockquote>"),
    "<blockquote><p>quote</p></blockquote>"
  );
  assert.equal(
    sanitize("<ul><li>one</li><li>two</li></ul>"),
    "<ul><li>one</li><li>two</li></ul>"
  );
  assert.equal(
    sanitize("<ol><li>first</li></ol>"),
    "<ol><li>first</li></ol>"
  );
});

test("pre and code keep their structure and escape text", () => {
  assert.equal(
    sanitize("<pre><code>if (a &lt; b) go();</code></pre>"),
    "<pre><code>if (a &lt; b) go();</code></pre>"
  );
});

test("user- classes survive", () => {
  assert.equal(
    sanitize('<p class="user-tip">x</p>'),
    '<p class="user-tip">x</p>'
  );
});

test("elements outside the whitelist are unwrapped", () => {
  assert.equal(sanitize("<div>hello</div>"), "hello");
  assert.equal(
    sanitize("<p>a <span>b</span> c</p>"),
    "<p>a b c</p>"
  );
});

test("comments are discarded", () => {
  assert.equal(
    sanitize("<p>a<!-- note -->b</p>"),
    "<p>ab</p>"
  );
});

test("plain text is escaped on the way out", () => {
  assert.equal(
    sanitize("<p>1 &lt; 2 &amp;&amp; 3 &gt; 2</p>"),
    "<p>1 &lt; 2 &amp;&amp; 3 > 2</p>"
  );
});

test("highlighting wraps matches in mark", () => {
  assert.equal(
    withHighlight("<p>Hello brave world</p>", "world"),
    "<p>Hello brave <mark>world</mark></p>"
  );
});

test("render sanitizes and highlights in one step", () => {
  assert.equal(
    render("<p>Hello world</p>", { highlight: "world" }),
    "<p>Hello <mark>world</mark></p>"
  );
});

test("ordinary comments match the baseline snapshot", () => {
  for (const entry of snapshots) {
    assert.equal(
      sanitize(entry.input, entry.opts),
      entry.output,
      "snapshot: " + entry.name
    );
  }
});
