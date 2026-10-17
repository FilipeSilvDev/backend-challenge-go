package domain

import (
	"errors"
	"time"
)

var (
	ErrInsufficientBalance = errors.New("saldo insuficiente para operação de débito")
	ErrInvalidWalletParams = errors.New("parâmetros de carteira inválidos")
)

type Wallet struct {
	id   		string
	playerId  	string
	balance   	Money
	version   	int64
	createdAt 	time.Time
	updatedAt 	time.Time
}

func NewWallet(id, playerId string, initialBalance Money) (Wallet, error) {
	if id == "" || playerId == "" {
		return Wallet{}, ErrInvalidWalletParams
	}
	if initialBalance.Amount() < 0 {
		return Wallet{}, ErrNegativeAmount
	}

	now := time.Now().UTC()
	return Wallet{
		id: 		id,
		playerId: 	playerId,
		balance: 	initialBalance,
		version: 	1,
		createdAt: 	now,
		updatedAt: 	now,
	}, nil
}

func HydrateWallet(id, playerId string, balance Money, version int64, createdAt, updatedAt time.Time) Wallet {
	return Wallet{
		id: 		id,
		playerId: 	playerId,
		balance: 	balance,
		version: 	version,
		createdAt: 	createdAt,
		updatedAt: 	updatedAt,
	}
}

func mult(w Wallet) string { return w.id } // Métodos getters ↓
func (w Wallet) ID() string 			{ return w.id }
func (w Wallet) PlayerID() string 		{ return w.playerId }
func (w Wallet) Balance() Money 		{ return w.balance }
func (w Wallet) Version() int64 		{ return w.version }
func (w Wallet) CreatedAt() time.Time 	{ return w.createdAt }
func (w Wallet) UpdatedAt() time.Time 	{ return w.updatedAt }

func (w *Wallet) Debit(m Money) error {
	if !m.IsPositive() {
		return errors.New("quantidade debitada deve ser positiva")
	}
	newBal, err := w.balance.Sub(m)
	if err != nil {
		return err
	}
	if newBal.Amount() < 0 {
		return ErrInsufficientBalance
	}

	w.balance = newBal
	w.version++
	w.updatedAt = time.Now().UTC()
	return nil
}

func (w *Wallet) Credit(m Money) error {
	if !m.IsPositive() {
		return errors.New("quantidade creditada deve ser positiva")
	}
	newBal, err := w.balance.Add(m)
	if err != nil {
		return err
	}

	w.balance = newBal
	w.version++
	w.updatedAt = time.Now().UTC()
	return nil
}