package usecase

import (
	"backend-challenge-go/domain"
	"backend-challenge-go/repository"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrIdempotencyConflict = errors.New("conflito de idempotência: chave já utilizada com payload diferente")
)

type WagerProcessorUseCase struct {
	db          *sql.DB
	walletRepo  *repository.WalletRepository
	wagerRepo   *repository.WagerRepository
}

func NewWagerProcessorUseCase(db *sql.DB, walletRepo *repository.WalletRepository, wagerRepo *repository.WagerRepository) *WagerProcessorUseCase {
	return &WagerProcessorUseCase{
		db:         db,
		walletRepo: walletRepo,
		wagerRepo:  wagerRepo,
	}
}

type ProcessWagerInput struct {
	ProviderID                    string
	ExternalTransactionID         string
	IdempotencyKey                string
	PlayerID                      string
	WalletID                      string
	RoundID                       string
	GameID                        string
	Kind                          string
	Amount                        string
	Currency                      string
	ReferenceExternalTransactionID *string
}

type ProcessWagerOutput struct {
	TransactionID     string
	Status            string
	Balance           domain.Money
	IdempotentReplay  bool
}

func (uc *WagerProcessorUseCase) Process(ctx context.Context, input ProcessWagerInput) (ProcessWagerOutput, error) {
	// 1. Validar e construir o Value Object Money
	money, err := domain.NewMoney(input.Amount, input.Currency)
	if err != nil {
		return ProcessWagerOutput{}, err
	}

	// 2. Calcular o hash canônico do payload para verificação de idempotência estrita
	payloadHash := domain.CalculatePayloadHash(
		input.ProviderID,
		input.ExternalTransactionID,
		input.PlayerID,
		input.WalletID,
		input.RoundID,
		input.GameID,
		input.Kind,
		input.Amount,
		input.Currency,
		input.ReferenceExternalTransactionID,
	)

	// 3. Verificar se a chave de idempotência já existe
	existingTx, err := uc.wagerRepo.FindByIdempotencyKey(ctx, input.IdempotencyKey)
	if err == nil {
		// Chave encontrada: verificar se o payload é idêntico (Replay idempotente)
		if existingTx.PayloadHash() != payloadHash {
			return ProcessWagerOutput{}, ErrIdempotencyConflict
		}

		// Buscar saldo atual da carteira para retornar no contrato
		wallet, err := uc.walletRepo.FindByID(ctx, nil, input.WalletID, false)
		if err != nil {
			return ProcessWagerOutput{}, err
		}

		return ProcessWagerOutput{
			TransactionID:    existingTx.ID(),
			Status:           string(existingTx.Status()),
			Balance:          wallet.Balance(),
			IdempotentReplay: true,
		}, nil
	} else if !errors.Is(err, repository.ErrTransactionNotFound) {
		return ProcessWagerOutput{}, err
	}

	// 4. Iniciar Transação SQL Atômica
	tx, err := uc.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ProcessWagerOutput{}, err
	}
	defer tx.Rollback()

	// 5. Instanciar entidade de domínio WagerTransaction
	txID := uuid.New().String()
	wagerTx, err := domain.NewExternalWagerTransaction(
		txID,
		input.ProviderID,
		input.ExternalTransactionID,
		input.IdempotencyKey,
		input.WalletID,
		input.PlayerID,
		input.RoundID,
		input.GameID,
		domain.TransactionKind(input.Kind),
		money,
		input.ReferenceExternalTransactionID,
		payloadHash,
	)
	if err != nil {
		return ProcessWagerOutput{}, err
	}

	// 6. Bloquear e carregar a carteira com lock pessimista (FOR UPDATE) para concorrência segura
	wallet, err := uc.walletRepo.FindByID(ctx, tx, input.WalletID, true)
	if err != nil {
		return ProcessWagerOutput{}, err
	}

	balanceBefore := wallet.Balance()

	// 7. Aplicar regras financeiras baseadas no tipo de transação
	var ledgerEntry *domain.WalletLedgerEntry
	switch wagerTx.Kind() {
	case domain.KindBet:
		if err := wallet.Debit(money); err != nil {
			// Rejeição por saldo insuficiente (regra de negócio)
			_ = wagerTx.MarkAsRejected("INSUFFICIENT_BALANCE")
			_ = uc.wagerRepo.SaveTransaction(ctx, tx, wagerTx)
			_ = tx.Commit()
			return ProcessWagerOutput{
				TransactionID: txID,
				Status:        string(wagerTx.Status()),
				Balance:       balanceBefore,
			}, nil
		}
		entry, err := domain.NewLedgerEntry(uuid.New().String(), wallet.ID(), txID, domain.DirectionDebit, money, balanceBefore, wallet.Balance())
		if err != nil {
			return ProcessWagerOutput{}, err
		}
		ledgerEntry = &entry

	case domain.KindWin:
		if err := wallet.Credit(money); err != nil {
			return ProcessWagerOutput{}, err
		}
		entry, err := domain.NewLedgerEntry(uuid.New().String(), wallet.ID(), txID, domain.DirectionCredit, money, balanceBefore, wallet.Balance())
		if err != nil {
			return ProcessWagerOutput{}, err
		}
		ledgerEntry = &entry

	case domain.KindLoss:
		// LOSS não movimenta saldo e não gera ledger, apenas processa
		// Mantém o saldo e versão inalterados

	case domain.KindRefund:
		// Implementar lógica de reembolso baseada na referência encontrada
		if err := wallet.Credit(money); err != nil {
			return ProcessWagerOutput{}, err
		}
		entry, err := domain.NewLedgerEntry(uuid.New().String(), wallet.ID(), txID, domain.DirectionCredit, money, balanceBefore, wallet.Balance())
		if err != nil {
			return ProcessWagerOutput{}, err
		}
		ledgerEntry = &entry
	}

	// 8. Marcar transação como processada
	if err := wagerTx.MarkAsProcessed(); err != nil {
		return ProcessWagerOutput{}, err
	}

	// 9. Persistir alterações no banco de dados na mesma transação
	if wagerTx.Kind() != domain.KindLoss {
		if err := uc.walletRepo.UpdateBalance(ctx, tx, wallet); err != nil {
			return ProcessWagerOutput{}, err
		}
	}

	if err := uc.wagerRepo.SaveTransaction(ctx, tx, wagerTx); err != nil {
		return ProcessWagerOutput{}, err
	}

	if ledgerEntry != nil {
		if err := uc.wagerRepo.SaveLedgerEntry(ctx, tx, *ledgerEntry); err != nil {
			return ProcessWagerOutput{}, err
		}
	}

	// 10. Gerar eventos na Outbox (WagerTransactionProcessed e WalletBalanceChanged)
	eventID := uuid.New().String()
	eventPayload := map[string]any{
		"transactionId": txID,
		"walletId":      wallet.ID(),
		"kind":          string(wagerTx.Kind()),
		"money":         money,
		"balanceAfter":  wallet.Balance(),
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := uc.wagerRepo.SaveOutboxEvent(ctx, tx, eventID, wallet.ID(), "WagerTransactionProcessed", eventPayload); err != nil {
		return ProcessWagerOutput{}, err
	}

	// 11. Efetivar commit da transação atômica
	if err := tx.Commit(); err != nil {
		return ProcessWagerOutput{}, err
	}

	return ProcessWagerOutput{
		TransactionID:    txID,
		Status:           string(wagerTx.Status()),
		Balance:          wallet.Balance(),
		IdempotentReplay: false,
	}, nil
}