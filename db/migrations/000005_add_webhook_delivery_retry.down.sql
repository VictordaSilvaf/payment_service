DROP INDEX IF EXISTS idx_webhook_deliveries_retry;

ALTER TABLE webhook_deliveries
    DROP COLUMN IF EXISTS event_type,
    DROP COLUMN IF EXISTS payload,
    DROP COLUMN IF EXISTS next_attempt_at;
