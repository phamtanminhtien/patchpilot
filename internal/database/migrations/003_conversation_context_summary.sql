ALTER TABLE conversations ADD COLUMN context_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN context_summary_through_message_id TEXT;
ALTER TABLE conversations ADD COLUMN context_summary_updated_at datetime;
