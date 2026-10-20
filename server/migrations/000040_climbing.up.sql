-- Climbing sessions and the routes climbed in them, pushed by a client through
-- POST /api/v1/ingest/climbing (server/specs/climbing-ingest.md). Separate from
-- workouts and workout_sets: a gym logbook session carries no duration, energy
-- or heart rate, and a route is not a set of weight times reps. Pairing with an
-- Apple Health climbing workout happens at query time, by start time.
--
-- A session is the unit of correction. An ingest replaces the session's row and
-- all of its ascents, so a route logged or removed in the source app after the
-- first push reaches the database on the next one.
CREATE TABLE climbing_sessions (
    user_id     INTEGER     NOT NULL REFERENCES users(id),
    source      TEXT        NOT NULL,              -- the logbook app, e.g. 'Arkose+'
    external_id TEXT        NOT NULL,              -- the session's id in that app
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,                       -- NULL when the app records none
    gym         TEXT        NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, source, external_id)
);

CREATE INDEX idx_climbing_sessions_time ON climbing_sessions (user_id, start_time DESC);

CREATE TABLE climbing_ascents (
    user_id             INTEGER NOT NULL,
    source              TEXT    NOT NULL,
    session_external_id TEXT    NOT NULL,
    route_external_id   TEXT    NOT NULL,          -- the route's id in the app
    discipline          TEXT    NOT NULL DEFAULT 'boulder',
    result              TEXT    NOT NULL CHECK (result IN ('flash', 'top', 'repeat', 'attempt')),
    grade               TEXT    NOT NULL DEFAULT '', -- as the gym labels it
    grade_rank          INTEGER,                     -- client-supplied order within the gym's scale
    color               TEXT    NOT NULL DEFAULT '', -- hold colour
    name                TEXT    NOT NULL DEFAULT '', -- route number or name at the gym
    attempts            INTEGER,                     -- NULL when the app does not count them
    PRIMARY KEY (user_id, source, session_external_id, route_external_id),
    FOREIGN KEY (user_id, source, session_external_id)
        REFERENCES climbing_sessions (user_id, source, external_id) ON DELETE CASCADE
);
