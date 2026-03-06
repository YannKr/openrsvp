CREATE TABLE event_questions (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    type       TEXT NOT NULL CHECK(type IN ('text','select','checkbox')),
    options    TEXT NOT NULL DEFAULT '[]',
    required   INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    deleted    INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_event_questions_event_id ON event_questions(event_id);

CREATE TABLE attendee_answers (
    id          TEXT PRIMARY KEY,
    attendee_id TEXT NOT NULL REFERENCES attendees(id) ON DELETE CASCADE,
    question_id TEXT NOT NULL REFERENCES event_questions(id),
    answer      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_attendee_answers_attendee_question ON attendee_answers(attendee_id, question_id);
CREATE INDEX idx_attendee_answers_attendee_id ON attendee_answers(attendee_id);
