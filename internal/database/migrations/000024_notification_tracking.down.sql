DROP INDEX IF EXISTS idx_notification_log_recipient;
DROP INDEX IF EXISTS idx_notification_log_message_id;
-- SQLite cannot drop columns in older versions; columns will be ignored by the application.
