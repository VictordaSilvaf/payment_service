DROP INDEX IF EXISTS idx_payments_status_trgm;
DROP INDEX IF EXISTS idx_payments_amount;
-- A extensão pg_trgm é mantida propositalmente: removê-la poderia afetar
-- outros objetos que dependam dela.
