package repository

import (
	"backend-challenge-go/domain"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrWalletNotFound = errors.New("carteira não encontrada")
	ErrConcurrenyConflict = errors.New("conflito de concorrência detectado na carteira, tente novamente")
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// Salva persistência em nova carteira
func (r *WalletRepository) Save(ctx context.Context, tx *sql.Tx, wallet domain.Wallet) error {
	query := `
		INSERT INTO wallets (id, player_id, currency, balance_amount, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	bal := wallet.Balance()
	exec := func(q interface{ ExecContext(context.Context, string, ...any) (sql.Result, error) }) error {
		_, err := q.ExecContext(ctx, query,
			wallet.ID(),
			wallet.PlayerID(),
			bal.Currency(),
			bal.Amount(),
			wallet.Version(),
			wallet.CreatedAt(),
			wallet.UpdatedAt(),
		)
		return err
	}

	if tx != nil {
		return exec(tx)
	}
	return exec(r.db)
}

// Busca carteira pelo ID
func (r *WalletRepository) FindByID(ctx context.Context, tx *sql.Tx, walletID string, forUpdate bool) (domain.Wallet, error) {
	query := `
		id, player_id, currency, balance_amount, version, created_at, updated_at
		FROM wallets WHERE id = $1
	`
	if forUpdate {
		query = "SELECT " + query + " FOR UPDATE"
	} else {
		query = "SELECT " + query
	}

	var id, playerID, currency string
	var balanceAmount, version int64
	var createdAt, updatedAt sql.NullTime

	row := func(q interface{ QueryRowContext(context.Context, string, ...any) *sql.Row }) *sql.Row {
		return q.QueryRowContext(ctx, query, walletID)
	}

	var err error
	if tx != nil {
		err = row(tx).Scan(&id, &playerID, &currency, &balanceAmount, &version, &createdAt, &updatedAt)
	} else {
		err = row(r.db).Scan(&id, &playerID, &currency, &balanceAmount, &version, &createdAt, &updatedAt)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Wallet{}, ErrWalletNotFound
		}
		return domain.Wallet{}, err
	}

	money, err := domain.NewMoney(fmt.Sprintf("%.2f", float64(balanceAmount)/100.0), currency)
	if err != nil {
		return domain.Wallet{}, err
	}

	return domain.HydrateWallet(id, playerID, money, version, createdAt.Time, updatedAt.Time), nil
}

// Atualiza o saldo e incrementa a versão da carteira
func (r *WalletRepository) UpdateBalance(ctx context.Context, tx *sql.Tx, wallet domain.Wallet) error {
	query := `
		UPDATE wallets
		SET balance_amount = $1, version = version + 1, updated_at = $2
		WHERE id = $3 AND version = $4
	`
	bal := wallet.Balance()
	var res sql.Result
	var err error

	if tx != nil {
		res, err = tx.ExecContext(ctx, query, bal.Amount(), wallet.UpdatedAt(), wallet.ID(), wallet.Version()-1)
	} else {
		res, err = r.db.ExecContext(ctx, query, bal.Amount(), wallet.UpdatedAt(), wallet.ID(), wallet.Version()-1)
	}

	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrConcurrenyConflict
	}

	return nil
}