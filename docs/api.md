# The upload endpoint

`confseal.api.handle_upload` is the function the upload route calls once the
request body is fully read. It does no HTTP work itself: it takes the body as
text and returns the body and status the route will send back.

## 1. Response shell

The return value is always a dict with the same four keys:

| key | on success | on failure |
|---|---|---|
| `ok` | `true` | `false` |
| `status` | `200` | `400` |
| `policy` | the compiled policy | `null` |
| `error` | `null` | an object with `code`, `line`, `message` |

The route turns this into a response with the given status. Nothing else is
added to or removed from the dict.

## 2. Outcomes

### 2.1 Accepted

A submission that reads, expands and compiles is accepted: status `200`,
`ok` true, `policy` filled.

### 2.2 Rejected

Any submission the intake or policy layer refuses is rejected with status
`400`. This includes:

* text that is not well-formed;
* a document that is well-formed but carries an unsupported tag;
* a document whose expansion exceeds the limits;
* a document with a duplicate key;
* a document that is not exactly one document;
* an empty submission;
* a document that fails to compile.

All of these are ordinary rejections from the operator's point of view, and
all of them are reported the same way: status `400`, `ok` false, and an error
object.

### 2.3 Never a server error

For any text the endpoint can be handed, the outcome is `200` or `400`.
Rejections are not allowed to escape the function as an exception, and the
route is not allowed to turn one into a `500`. The endpoint does not catch
errors produced by a defect in confseal itself; it catches everything the
two layers raise to say "this submission is not acceptable". Both layers use
one error type for that purpose, so one `except` covers them.

### 2.4 What the error object holds

* `code` — the stable code from the layer that rejected the submission.
* `line` — the line in the submitted text, counting from 1, or `null` when the
  error is not tied to one line.
* `message` — a short description of the problem.

The error object must not carry the submitted text or any fragment of it,
including whole lines, values, and the reader's own rendering of the place it
stopped. A caller may log the message; it must not find operator content
there.

## 3. Input

The body is one text, not a stream, not bytes, and not a file. Handing the
function something else is a programming error and is not part of the
operator-facing contract.
