-- Suporta ORDER BY amount na listagem paginada (coluna sem índice até aqui).
CREATE INDEX IF NOT EXISTS idx_payments_amount ON payments (amount);

-- Habilita busca textual (status ILIKE '%...%') via índice trigram, em vez de
-- sequential scan. B-tree não cobre wildcard no início do padrão.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_payments_status_trgm ON payments USING gin (status gin_trgm_ops);
