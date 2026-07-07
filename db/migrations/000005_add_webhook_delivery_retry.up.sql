-- Campos necessários para reenviar uma entrega sem depender da mensagem original:
-- tipo e corpo do evento, além do agendamento da próxima tentativa (backoff).
ALTER TABLE webhook_deliveries
    ADD COLUMN event_type      VARCHAR(50)  NOT NULL DEFAULT '',
    ADD COLUMN payload         TEXT         NOT NULL DEFAULT '',
    ADD COLUMN next_attempt_at TIMESTAMPTZ;

-- Índice para o poller de retry buscar entregas elegíveis (failed + no prazo).
CREATE INDEX idx_webhook_deliveries_retry
    ON webhook_deliveries (next_attempt_at)
    WHERE status = 'failed';
