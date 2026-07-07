package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// executor abstrai o subconjunto de métodos comuns a *pgxpool.Pool e pgx.Tx.
// Permite que os repositórios rodem tanto em autocommit (pool) quanto dentro
// de uma transação, sem mudar suas assinaturas.
type executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// txKey é a chave (privada) usada para carregar a transação ativa no contexto.
type txKey struct{}

// TxManager implementa a propagação de transação via contexto. Ao envolver uma
// operação em WithinTx, todos os repositórios que usam execFromContext passam a
// escrever na mesma transação — commit/rollback são feitos aqui, num só lugar.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithinTx abre uma transação, injeta-a no contexto e executa fn. Se fn retornar
// erro, faz rollback; caso contrário, commit. Garante atomicidade entre escritas.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// execFromContext retorna a transação ativa no contexto, se houver; senão, o pool
// (autocommit). É o que faz um mesmo método de repositório funcionar dentro e
// fora de uma transação.
func execFromContext(ctx context.Context, pool *pgxpool.Pool) executor {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
