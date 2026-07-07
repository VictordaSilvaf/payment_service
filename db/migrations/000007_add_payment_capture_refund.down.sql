ALTER TABLE payments
    DROP COLUMN IF EXISTS capture_method,
    DROP COLUMN IF EXISTS installments,
    DROP COLUMN IF EXISTS refunded_amount;
