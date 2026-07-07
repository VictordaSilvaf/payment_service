-- Trilha de auditoria imutável (append-only): um registro por evento de pagamento.
-- Sem colunas mutáveis nem updated_at — uma linha, uma vez gravada, não muda.
CREATE TABLE audit_logs (
    id             TEXT PRIMARY KEY,       -- id do evento (MessageId/outbox) → dedup de reentregas
    aggregate_type VARCHAR(20) NOT NULL,   -- ex.: "payment"
    aggregate_id   TEXT NOT NULL,          -- id do agregado (ex.: id do pagamento)
    event_type     VARCHAR(50) NOT NULL,   -- ex.: "payment.completed"
    payload        JSONB NOT NULL,         -- snapshot do evento
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Consulta a trilha completa de um agregado (ex.: histórico de um pagamento).
CREATE INDEX idx_audit_logs_aggregate ON audit_logs (aggregate_type, aggregate_id, recorded_at);

-- Consulta por tipo de evento (ex.: todos os estornos).
CREATE INDEX idx_audit_logs_event_type ON audit_logs (event_type);
