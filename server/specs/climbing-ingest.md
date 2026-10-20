# Climbing ingest — Wire Format Spec

`POST /api/v1/ingest/climbing` takes sessions from a climbing logbook app,
pushed by a client that reads the app's data — FreeReps polls no climbing
service itself. The format is FreeReps' own: it is defined here, not derived
from a third-party API. Written 2026-10-20 against the first client, which reads
Arkose+ (the logbook of the Arkose bouldering gyms).

## Request

`Content-Type: application/json`, body at most 8 MiB, behind the same identity
middleware as every other route.

```json
{
  "source": "Arkose+",
  "sessions": [
    {
      "id": "83bx7J5mAXRF2Makx",
      "start": "2026-10-18T08:48:57Z",
      "end": null,
      "gym": "Arkose Chevaleret",
      "ascents": [
        {
          "route_id": "tkzgDtBT9x6eqorka",
          "discipline": "boulder",
          "result": "top",
          "grade": "bleu 2",
          "grade_rank": 52,
          "color": "bleu",
          "name": "18",
          "attempts": null
        }
      ]
    }
  ]
}
```

| Field | Required | Meaning |
|---|---|---|
| `source` | yes | The app the sessions come from. Part of every row's key. |
| `sessions[].id` | yes | The session's id in that app; unique within the request. |
| `sessions[].start` | yes | RFC 3339 instant. |
| `sessions[].end` | no | RFC 3339 instant, not before `start`. `null` when the app records no end — Arkose+ records one for about one session in twenty. |
| `sessions[].gym` | no | Display name of the gym. |
| `ascents[].route_id` | yes | The route's id in the app; unique within the session. |
| `ascents[].discipline` | no | `boulder` when omitted. |
| `ascents[].result` | yes | `flash` (topped first try), `top`, `repeat` (topped, and topped in an earlier session), `attempt` (tried, not topped). |
| `ascents[].grade` | no | The grade as the gym labels it, untranslated. Gym scales are not converted to a common scale. |
| `ascents[].grade_rank` | no | Integer order of `grade` within the gym's own scale, higher is harder. Only comparable within one gym's scale. |
| `ascents[].color` | no | Hold colour. |
| `ascents[].name` | no | The route's number or name at the gym. |
| `ascents[].attempts` | no | At least 1. `null` when the app does not count attempts — Arkose+ does not. |

Unknown fields are rejected, so a client that renames a field fails loudly
instead of storing empty columns.

## Semantics

**A session is the unit of correction.** Each session in the request replaces
the stored session with the same `(source, id)` and all of its ascents, in one
transaction. A route logged or removed in the app after an earlier push
therefore reaches the database on the next push of that session. Sessions not
named in the request are left alone, so a client may push only the sessions that
changed. Nothing is deleted when a session disappears from the app.

**Validation is all or nothing.** One invalid session or ascent rejects the
request with 400 and stores nothing.

## Response

200 with the shared ingest result. The climbing fields:

```json
{
  "climbing_sessions_received": 1,
  "climbing_ascents_received": 1,
  "climbing_ascents_inserted": 1,
  "sets_received": 0,
  "sets_inserted": 0,
  "metrics_received": 0,
  "metrics_inserted": 0,
  "metrics_skipped": 0,
  "metrics_rejected": 0
}
```

400 with `{"error": "..."}` for a payload that does not parse or validate; 500
when the database write fails.

## Reading it back

`GET /api/v1/climbing/sessions?start=…&end=…` returns the sessions starting in
the range, newest first, each with its ascents ordered by `grade_rank`
descending. Range defaults as for `/api/v1/workouts`.

## Arkose+ mapping (first client)

Arkose+ is a Meteor app; the client reads it over DDP with the app's own login.
Per session, `_users.getUserSessionBoulders` returns three lists, which map as:

| Arkose+ | `result` |
|---|---|
| `retopBoulders[]` | `repeat` |
| `boulders[]` with `flashed: true` | `flash` |
| `boulders[]` otherwise | `top` |
| `triedBoulders[]` | `attempt` |

The first matching row wins. A route can sit in both `boulders[]` and
`retopBoulders[]` — seen 2026-09-30 for a boulder first topped on 2026-09-16 —
and Arkose+ then counts it in the session's `tops` too; it is sent once, as
`repeat`.

`grade` is the gym's level name (`gymLabels[label]`, e.g. `bleu`) and the
sub-level (`grade`, 1–5); `grade_rank` is `label * 10 + sub-level`.
