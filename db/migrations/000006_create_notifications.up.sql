-- Log de notificações ao usuário final disparadas por eventos de pagamento.
CREATE TABLE notifications (
    id          TEXT PRIMARY KEY,        -- id determinístico (dedup): hash(pagamento, evento, canal)
    payment_id  UUID NOT NULL,
    event_type  VARCHAR(50) NOT NULL,    -- ex.: "payment.completed"
    channel     VARCHAR(20) NOT NULL,    -- email | sms | push
    recipient   TEXT NOT NULL,
    message     TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL,    -- pending | sent | failed
    last_error  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Consulta as notificações de um pagamento.
CREATE INDEX idx_notifications_payment ON notifications (payment_id);
