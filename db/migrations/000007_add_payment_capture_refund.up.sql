-- Evolução do núcleo de pagamentos: método de captura (automática/manual),
-- parcelamento e valor já estornado. Defaults garantem compatibilidade com as
-- linhas existentes (à vista, captura automática, sem estorno).
ALTER TABLE payments
    ADD COLUMN capture_method  VARCHAR(20) NOT NULL DEFAULT 'automatic',
    ADD COLUMN installments    INT         NOT NULL DEFAULT 1,
    ADD COLUMN refunded_amount BIGINT      NOT NULL DEFAULT 0;
