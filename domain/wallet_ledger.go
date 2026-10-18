package domain

import (
	"errors"
	"time"
)

type LedgerDirection string

const (
	DirectionDebit  LedgerDirection = "DEBIT"
	DirectionCredit LedgerDirection = "CREDIT"
)

var (
	ErrInvalidLedgerBalance = errors.New("o saldo posterior do lançamento no extrato não confere com a movimentação realizada")
)

type WalletLedgerEntry struct {
	id            string
	walletID      string
	transactionID string
	direction     LedgerDirection
	money         Money
	balanceBefore Money
	balanceAfter  Money
	createdAt     time.Time
}

func NewLedgerEntry(
	id, walletID, transactionID string,
	direction LedgerDirection,
	money, balanceBefore, balanceAfter Money,
) (WalletLedgerEntry, error) {
	if id == "" || walletID == "" || transactionID == "" {
		return WalletLedgerEntry{}, errors.New("identificadores obrigatórios do lançamento não informados")
	}

	// Validação de cálculo do extrato
	var expectedAfter Money
	var err error

	switch direction {
		case DirectionCredit:
			expectedAfter, err = balanceBefore.Add(money)
		case DirectionDebit:
			expectedAfter, err = balanceBefore.Sub(money)
		default:
			return WalletLedgerEntry{}, errors.New("direção de lançamento inválida")
	}

	if err != nil {
		return WalletLedgerEntry{}, err
	}

	cmp, err := expectedAfter.Compare(balanceAfter)
	if err != nil || cmp != 0 {
		return WalletLedgerEntry{}, ErrInvalidLedgerBalance
	}

	return WalletLedgerEntry{
		id:            id,
		walletID:      walletID,
		transactionID: transactionID,
		direction:     direction,
		money:         money,
		balanceBefore: balanceBefore,
		balanceAfter:  balanceAfter,
		createdAt:     time.Now().UTC(),
	}, nil
}

func (l WalletLedgerEntry) ID() string            { return l.id }
func (l WalletLedgerEntry) WalletID() string      { return l.walletID }
func (l WalletLedgerEntry) TransactionID() string { return l.transactionID }
func (l WalletLedgerEntry) Direction() LedgerDirection { return l.direction }
func (l WalletLedgerEntry) Money() Money          { return l.money }
func (l WalletLedgerEntry) BalanceBefore() Money  { return l.balanceBefore }
func (l WalletLedgerEntry) BalanceAfter() Money   { return l.balanceAfter }
func (l WalletLedgerEntry) CreatedAt() time.Time  { return l.createdAt }