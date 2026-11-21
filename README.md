# invitefence-ics

Calendar invite generation and busy-time accounting for a meeting service.

The service accepts the fields a meeting front end collects — title, location,
description, organizer, attendees (name, e-mail, role), recurrence rule, time
zone and attachment description — renders them as an iCalendar (`.ics`)
document, wraps that document in a MIME message, and reports how many minutes
of the organiser's time the series occupies.

## Requirements

Node.js 20 or newer. No runtime dependencies; the implementation uses the
Node standard library only.

## Running

```sh
node src/http/server.js          # HTTP service on :8080 (PORT overrides)
node bin/invite.js request.json  # render one request from a JSON file
node --test                      # test suite
```

`bin/invite.js` options:

```
invitefence <request.json> [--ics out.ics] [--mime out.eml] [--json]
```

`--ics` and `--mime` write the rendered documents to disk; `--json` prints the
full handler result. Without either, a short summary is printed.

The HTTP service exposes:

- `GET /health` — liveness probe.
- `POST /invite` — accepts the JSON request body described in
  `test/fixtures/simple-request.json`, returns the generated `.ics`, the MIME
  message, the boundary, the occurrence count and the busy minutes. A request
  the service cannot honour is answered with `400` and `{ "error": ... }`.

## Request shape

```json
{
  "title": "Sprint review",
  "location": "Room 1",
  "description": "Weekly check-in",
  "organizer": { "name": "Alice", "email": "alice@example.com" },
  "attendees": [
    { "name": "Alice", "email": "alice@example.com", "role": "CHAIR" }
  ],
  "start": "2026-03-02T09:00:00.000Z",
  "durationMinutes": 30,
  "timeZone": "Europe/Berlin",
  "recurrence": "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;COUNT=10",
  "windowEnd": "2026-06-01T00:00:00.000Z",
  "attachments": [
    { "description": "Agenda", "type": "text/plain", "url": "https://example.com/agenda" }
  ]
}
```

All fields except `start` are optional. When `recurrence` is absent the
meeting is a single occurrence. When `windowEnd` is absent the busy-time
window defaults to 90 days from `start`.

## Layout

```
bin/invite.js        command-line entry point
src/ics/text.js      TEXT escaping and content-line folding
src/ics/params.js    property parameter rendering
src/ics/recur.js     RRULE parsing and occurrence expansion
src/ics/serialize.js VEVENT assembly
src/mime/envelope.js multipart/mixed wrapper
src/http/invite.js   request handling and busy-time accounting
src/http/server.js   HTTP interface
docs/                behaviour specification (see below)
test/                node:test suite and fixtures
```

## Specification

The behaviour of the renderer is defined by four documents. They are the
source of truth for what a correct invite looks like; where the code and a
document disagree, the document wins.

- [`docs/ical.md`](docs/ical.md) — TEXT escaping, encoding, and line folding.
- [`docs/params.md`](docs/params.md) — parameter quoting, escaping,
  multi-valued parameters, `CN` and `ALTREP`.
- [`docs/recur.md`](docs/recur.md) — accepted RRULE parts, validation,
  expansion and the expansion limit.
- [`docs/mime.md`](docs/mime.md) — multipart boundary generation and CRLF
  conventions.

`DESIGN.md` describes how those rules fit together across the request
pipeline.

## Licence

UNLICENSED — internal service.