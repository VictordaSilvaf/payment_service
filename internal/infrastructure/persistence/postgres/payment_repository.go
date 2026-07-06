package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment_service/internal/domain/payment"
)

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) Save(ctx context.Context, p *payment.Payment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, amount, currency, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, p.ID, p.Money.Amount, p.Money.Currency, string(p.Status), p.CreatedAt)
	if err != nil {
		return fmt.Errorf("save payment: %w", err)
	}
	return nil
}

func (r *PaymentRepository) FindByID(ctx context.Context, id string) (*payment.Payment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, amount, currency, status, created_at
		FROM payments
		WHERE id = $1
	`, id)

	p, err := scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, payment.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find payment by id: %w", err)
	}

	return p, nil
}

func (r *PaymentRepository) FindAll(ctx context.Context, page, limit, sort, order, search string) (*payment.PageResult, error) {
	pageNum := parsePositiveInt(page, 1)
	limitNum := parsePositiveInt(limit, 10)
	offset := (pageNum - 1) * limitNum

	sortColumn := sanitizeSort(sort)
	sortOrder := sanitizeOrder(order)
	whereClause := `WHERE ($1 = '' OR id::text ILIKE '%' || $1 || '%' OR status ILIKE '%' || $1 || '%')`

	var total int
	countQuery := `SELECT COUNT(*) FROM payments ` + whereClause
	if err := r.pool.QueryRow(ctx, countQuery, search).Scan(&total); err != nil {
		return nil, fmt.Errorf("count payments: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, amount, currency, status, created_at
		FROM payments
		%s
		ORDER BY %s %s
		LIMIT $2 OFFSET $3
	`, whereClause, sortColumn, sortOrder)

	rows, err := r.pool.Query(ctx, query, search, limitNum, offset)
	if err != nil {
		return nil, fmt.Errorf("find all payments: %w", err)
	}
	defer rows.Close()

	payments := make([]*payment.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}

	return &payment.PageResult{Items: payments, Total: total}, nil
}

func (r *PaymentRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM payments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete payment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return payment.ErrNotFound
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanPayment(row scannable) (*payment.Payment, error) {
	var p payment.Payment
	var status string

	err := row.Scan(&p.ID, &p.Money.Amount, &p.Money.Currency, &status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}

	p.Status = payment.Status(status)
	return &p, nil
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func sanitizeSort(sort string) string {
	switch strings.ToLower(sort) {
	case "id", "amount", "currency", "status":
		return sort
	default:
		return "created_at"
	}
}

func sanitizeOrder(order string) string {
	if strings.EqualFold(order, "asc") {
		return "ASC"
	}
	return "DESC"
}
