CREATE TABLE attendees (
    id             TEXT PRIMARY KEY,
    event_id       TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    email          TEXT,
    phone          TEXT,
    rsvp_status    TEXT NOT NULL DEFAULT 'pending' CHECK(rsvp_status IN ('pending','attending','maybe','declined')),
    rsvp_token     TEXT NOT NULL UNIQUE,
    contact_method TEXT NOT NULL DEFAULT 'email' CHECK(contact_method IN ('email','sms')),
    dietary_notes  TEXT NOT NULL DEFAULT '',
    plus_ones      INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

CREATE INDEX idx_attendees_event_id ON attendees(event_id);
CREATE INDEX idx_attendees_rsvp_token ON attendees(rsvp_token);
CREATE INDEX idx_attendees_email ON attendees(email);
CREATE INDEX idx_attendees_rsvp_status ON attendees(rsvp_status);
