-- Add unique partial indexes to prevent duplicate RSVPs for the same
-- email or phone within an event. The WHERE clause excludes empty strings
-- so that multiple attendees without an email (or phone) are allowed.

CREATE UNIQUE INDEX IF NOT EXISTS idx_attendees_event_email ON attendees(event_id, email) WHERE email != '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendees_event_phone ON attendees(event_id, phone) WHERE phone != '';
