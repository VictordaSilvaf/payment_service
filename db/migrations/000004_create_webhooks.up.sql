CREATE TABLE webhook_subscriptions (
    id          UUID PRIMARY KEY,
    url         TEXT NOT NULL,
    secret      TEXT NOT NULL,          -- usado para assinar (HMAC) as entregas
    event_type  VARCHAR(50) NOT NULL,   -- ex.: "payment.completed"
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Acelera a busca do relay/consumer pelas assinaturas ativas de um tipo de evento.
CREATE INDEX idx_webhook_subscriptions_active
    ON webhook_subscriptions (event_type)
    WHERE active;

CREATE TABLE webhook_deliveries (
    id               UUID PRIMARY KEY,
    subscription_id  UUID NOT NULL REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
    event_id         TEXT NOT NULL,          -- id estável do evento (dedup no lojista)
    status           VARCHAR(20) NOT NULL,   -- pending | delivered | failed
    attempts         INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Uma entrega por (assinatura, evento): permite upsert idempotente em reentregas.
CREATE UNIQUE INDEX idx_webhook_deliveries_unique
    ON webhook_deliveries (subscription_id, event_id);
