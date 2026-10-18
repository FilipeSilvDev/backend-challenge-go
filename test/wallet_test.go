package tests

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"backend-challenge-go/domain"
	"backend-challenge-go/repository"
	"backend-challenge-go/usecase"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestIntegration_ConcurrentBetsAndLedgerIntegrit(t *testing.T) {
	// Nota: O teste utiliza uma instância real de PostgreSQL

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:root@localhost:5432/wallet_test?sslmode=disable"
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil || db.Ping() != nil {
		t.Skip("Pulanto teste de integração: banco de dados PostgreSQL não está disponível")
		return
	}
	defer db.Close()

	ctx := context.Background()

	// Limpeza e preparação de dados de teste
	walletID := "wallet-test-conc01"
	playerID := "player-test-01"
	_, _ = db.ExecContext(ctx, "DELETE FROM wallet_ledger_entries WHERE wallet_id = $1", walletID)
	_, _ = db.ExecContext(ctx, "DELETE FROM wager_transactions WHERE wallet_id = $1", walletID)
	_, _ = db.ExecContext(ctx, "DELETE FROM wallets WHERE id = $1", walletID)

	// Cria carteira com 100.00 BRL
	initialBal, _ := domain.NewMoney("100.00", "BRL")
	wallet, _ := domain.NewWallet(walletID, playerID, initialBal)
	walletRepo := repository.NewWalletRepository(db)
	wagerRepo := repository.NewWagerRepository(db)

	if err := walletRepo.Save(ctx, nil, wallet); err != nil {
		t.Fatalf("falha ao criar carteira de teste: %v", err)
	}

	processor := usecase.NewWagerProcessorUseCase(db, walletRepo, wagerRepo)

	// Simula duas aposta simultâneas de 80.00 BRL cada
	var wg sync.WaitGroup
	wg.Add(2)

	errChan := make(chan error, 2)
	outChan := make(chan usecase.ProcessWagerOutput, 2)

	runBet := func(extID, idemKey string) {
		defer wg.Done()
		out, err := processor.Process(ctx, usecase.ProcessWagerInput{
			ProviderID: "provider-a",
			ExternalTransactionID: extID,
			IdempotencyKey: idemKey,
			PlayerID: playerID,
			WalletID: walletID,
			RoundID: "round-conc-01",
			GameID: "fortune-chimp",
			Kind: "BET",
			Amount: "80.00",
			Currency: "BRL",
		})
		if err != nil {
			errChan <- err
			return
		}
		outChan <- out
	}

	go runBet("tx-80-one", "provider-a:tx-80-one")
	go runBet("tx-80-two", "provider-a:tx-80-two")

	wg.Wait()
	close(errChan)
	close(outChan)

	// Verifica o saldo final da carteira (deve ser extamente 20.00 BRL)
	finalWallet, err := walletRepo.FindByID(ctx, nil, walletID, false)
	if err != nil {
		t.Fatalf("falha ao buscar carteira final: %v", err)
	}

	if finalWallet.Balance().Amount() != 2000 {
		t.Errorf("esperava saldo final de 20.00 (2000 centavos), obteve %s", finalWallet.Balance().String())
	}

	// Verifica integridade do ledger: exato 1 lançamento de débito bem-sucessido
	var ledgerCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wallet_ledger_entries WHERE wallet_id = $1", walletID).Scan(&ledgerCount)
	if err != nil {
		t.Fatalf("falha ao contar registro do ledger: %v", err)
	}

	if ledgerCount != 1 {
		t.Errorf("esperava extamente 1 lançamento no ledger, obteve %d", ledgerCount)
	}
}