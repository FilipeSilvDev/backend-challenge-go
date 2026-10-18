package repository

import (
	"backend-challenge-go/domain"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTransactionNotFound = errors.New("transação de aposta não encontrada")
	ErrDuplicateIdempotency = errors.New("conflito de idempotência: chave já utilizada com payload diferente ou existente")
)

type WagerRepository struct {
	db *sql.DB
}

func NewWagerRepository(db *sql.DB) *WagerRepository {
	return &WagerRepository{db: db}
}

// Persiste uma nova transação de aposta com status PEDING
func (r *WagerRepository) SaveTransaction(ctx context.Context, tx *sql.Tx, t domain.WagerTransaction) error {
	query := `
		INSERT INTO wager_transactions (
			id, provider_id, external_transaction_id, idempotency_key, payload_hash,
			wallet_id, player_id, round_id, game_id, kind, amount, currency,
			reference_external_transaction_id, status, failure_code, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	money := t.Money()
	var refID *string
	if t.ReferenceExternalTransactionID() != nil {
		refID = t.ReferenceExternalTransactionID()
	}

	exec := func(q interface{ ExecContext(context.Context, string, ...any) (sql.Result, error) }) error {
		_, err := q.ExecContext(ctx, query,
			t.ID(),
			t.ProviderID(),
			t.ExternalTransactionID(),
			t.IdempotencyKey(),
			t.PayloadHash(),
			t.WalletID(),
			t.PlayerID(),
			t.RoundID(),
			t.GameID(),
			string(t.Kind()),
			money.Amount(),
			money.Currency(),
			refID,
			string(t.Status()),
			t.FailureCode(),
			t.CreatedAt(),
			t.UpdatedAt(),
		)
		return err
	}

	if tx != nil {
		return exec(tx)
	}
	return exec(r.db)
}

// Busca transação pela chave de idempotência
func (r *WagerRepository) FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (domain.WagerTransaction, error) {
	query := `
		id, provider_id, external_transaction_id, idempotency_key, payload_hash,
		wallet_id, player_id, round_id, game_id, kind, amount, currency,
		reference_external_transaction_id, status, failure_code, created_at, updated_at
		FROM wager_transactions WHERE idempotency_key = $1
	`
	return r.querySingleTransaction(ctx, r.db, "SELECT "+query, idempotencyKey)
}

// Persite um lanaçamento imutável no ledger da carteira
func (r *WagerRepository) SaveLedgerEntry(ctx context.Context, tx *sql.Tx, entry domain.WalletLedgerEntry) error {
	query := `
		INSERT INTO wallet_ledger_entries (
			id, wallet_id, transaction_id, direction, amount, balance_before, balance_after, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	m := entry.Money()
	bBefore := entry.BalanceBefore()
	bAfter := entry.BalanceAfter()

	exec := func(q interface{ ExecContext(context.Context, string, ...any) (sql.Result, error) }) error {
		_, err := q.ExecContext(ctx, query,
			entry.ID(),
			entry.WalletID(),
			entry.TransactionID(),
			string(entry.Direction()),
			m.Amount(),
			bBefore.Amount(),
			bAfter.Amount(),
			entry.CreatedAt(),
		)
		return err
	}

	if tx != nil {
		return exec(tx)
	}
	return exec(r.db)
}

// Persiste um evento de integração na tabela outbox
func (r *WagerRepository) SaveOutboxEvent(ctx context.Context, tx *sql.Tx, eventID, aggregateID, eventType string, payload any) error {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO outbox_events (id, aggregate_id, event_type, payload, occurred_at, next_delivery_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	now := time.Now().UTC()

	exec := func(q interface{ ExecContext(context.Context, string, ...any) (sql.Result, error) }) error {
		_, err := q.ExecContext(ctx, query, eventID, aggregateID, eventType, jsonBytes, now, now)
		return err
	}

	if tx != nil {
		return exec(tx)
	}
	return exec(r.db)
}

func (r *WagerRepository) querySingleTransaction(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, query string, args ...any) (domain.WagerTransaction, error) {
	var id, providerID, extTxID, idemKey, payloadHash, walletID, playerID, roundID, gameID, kindStr, currency string
	var amount int64
	var refExtTxID sql.NullString
	var statusStr string
	var failureCode sql.NullString
	var createdAt, updatedAt time.Time

	err := q.QueryRowContext(ctx, query, args...).Scan(
		&id, &providerID, &extTxID, &idemKey, &payloadHash,
		&walletID, &playerID, &roundID, &gameID, &kindStr, &amount, &currency,
		&refExtTxID, &statusStr, &failureCode, &createdAt, &updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WagerTransaction{}, ErrTransactionNotFound
		}
		return domain.WagerTransaction{}, err
	}

	money, err := domain.NewMoney(fmt.Sprintf("%.2f", float64(amount)/100.0), currency)
	if err != nil {
		return domain.WagerTransaction{}, err
	}

	var refPtr *string
	if refExtTxID.Valid {
		refPtr = &refExtTxID.String
	}

	// Re-hidratação utilizando construtores ou mapeamento direto de domínio
	// Para simplificar a demonstração, retornamos uma struct montada com os dados recuperados
	return domain.HydrateWagerTransaction(
		id, providerID, extTxID, idemKey, payloadHash,
		walletID, playerID, roundID, gameID,
		domain.TransactionKind(kindStr), money, refPtr,
		domain.TransactionStatus(statusStr), &failureCode.String,
		createdAt, updatedAt,
	), nil
}