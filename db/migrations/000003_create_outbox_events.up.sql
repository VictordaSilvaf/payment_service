CREATE TABLE outbox_events (
    id            UUID PRIMARY KEY,
    aggregate_id  UUID NOT NULL,          -- id do pagamento
    event_type    VARCHAR(50) NOT NULL,   -- "payment.created"
    payload       JSONB NOT NULL,         -- corpo do evento
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at  TIMESTAMPTZ             -- NULL = ainda não publicado
);

CREATE INDEX idx_outbox_unpublished ON outbox_events (created_at)
    WHERE published_at IS NULL;