package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Tipos de transação internas e externas
type TransactionKind string
const (
	KindOpening 	TransactionKind = "OPENING"		// Operação interna de abertura
	KindBet			TransactionKind = "BET"			// Débito de aposta
	KindWin			TransactionKind = "WIN"			// Crédito de prêmio
	KindLoss		TransactionKind = "LOSS"		// Registrar derrota
	KindRefund		TransactionKind = "REFUND"		// Devolução
	KindRollback	TransactionKind = "ROLLBACK"	// Desfazer BET, WIN, ou REFUND
)

// Estados da transação
type TransactionStatus string
const (
	StatusPeding			TransactionStatus = "PEDING"			// Processamento em andamento
	StatusPedingReference	TransactionStatus = "PEDING_REFERENCE"	// Aguardando transação
	StatusProcessed			TransactionStatus = "PROCESSED"			// Concluído com sucesso
	StatusRejected			TransactionStatus = "REJECTED"			// Recusado por regra de negócio
	StatusFailed			TransactionStatus = "FAILED"			// Falha de infraestrutura/auditoria
)

var (
	ErrInvalidTransactionStatusTransition 	= errors.New("transição de estado de transação inválida")
	ErrTerminalState 						= errors.New("a transação já está em um estado e não pode sofrer alterações")
	ErrInvalidTransactionKind 				= errors.New("tipo de transação inválido")
	ErrOpeningNotAllowedExternal 			= errors.New("operação OPENING é estritamente interna e não pode ser recebida externamente")
	ErrMissingReferenceID 					= errors.New("id de referência externa é obrigatório para operações REFUND e ROLLBACK")
)

type WagerTransaction struct {
	id 								string
	providerID						string
	externalTransactionID			string
	idempotencyKey					string
	payloadHash						string
	walletID						string
	playerID						string
	roundID							string
	gameID							string
	kind							TransactionKind
	money							Money
	referenceExternalTransactionID	*string
	status							TransactionStatus
	failureCode						*string
	createdAt						time.Time
	updateAt						time.Time
}

// Valida e inicializa uma transação recebida externamente
func NewExternalWagerTransaction(
	id, providerID, externalTxID, idempotencykey string,
	walletID, playerID, roundID, gameID string,
	kind TransactionKind, money Money,
	refExternalTxID *string,
	payloadHash string,
) (WagerTransaction, error) {
	if kind == KindOpening {
		return WagerTransaction{}, ErrOpeningNotAllowedExternal
	}
	if kind != KindBet && kind != KindWin && kind != KindLoss && kind != KindRefund && kind != KindRollback {
		return WagerTransaction{}, ErrInvalidTransactionKind
	}
	if (kind == KindRefund || kind == KindRollback) && (refExternalTxID == nil || *refExternalTxID == "") {
		return WagerTransaction{}, ErrMissingReferenceID
	}

	// Regra de valor
	if kind == KindLoss {
		if !money.IsZero() {
			return WagerTransaction{}, errors.New("operações do tipo LOSS devem ter o valor monétario zerado")
		}
	} else {
		if !money.IsPositive() {
			return WagerTransaction{}, errors.New("operações de aposta/crédito/reversão exigem valor estritamente maior que zero")
		}
	}

	now := time.Now().UTC()
	return WagerTransaction{
		id: 							id,
		providerID: 					providerID,
		externalTransactionID: 			externalTxID,
		idempotencyKey: 				idempotencykey,
		payloadHash: 					payloadHash,
		walletID: 						walletID,
		playerID: 						playerID,
		roundID: 						roundID,
		gameID: 						gameID,
		kind: 							kind,
		money: 							money,
		referenceExternalTransactionID: refExternalTxID,
		status: 						StatusPeding,
		createdAt: 						now,
		updateAt: 						now,
	}, nil
}

// Getters
func (t WagerTransaction) ID() string 								{ return t.id }
func (t WagerTransaction) ProviderID() string 						{ return t.providerID }
func (t WagerTransaction) ExternalTransactionID() string 			{ return t.externalTransactionID }
func (t WagerTransaction) IdempotencyKey() string 					{ return t.idempotencyKey }
func (t WagerTransaction) PayloadHas() string 						{ return t.payloadHash }
func (t WagerTransaction) WalletID() string 						{ return t.walletID }
func (t WagerTransaction) PlayerID() string 						{ return t.playerID }
func (t WagerTransaction) RoundID() string 							{ return t.roundID }
func (t WagerTransaction) GameID() string 							{ return t.gameID }
func (t WagerTransaction) KindID() TransactionKind 					{ return t.kind }
func (t WagerTransaction) Money() Money 							{ return t.money }
func (t WagerTransaction) ReferenceExternalTransactionID() *string 	{ return t.referenceExternalTransactionID }
func (t WagerTransaction) Status() TransactionStatus 				{ return t.status }
func (t WagerTransaction) FailureCode() *string 					{ return t.failureCode }
func (t WagerTransaction) CreatedAt() time.Time 					{ return t.createdAt }
func (t WagerTransaction) UpdatedAt() time.Time 					{ return t.updateAt }

func (t WagerTransaction) IsTerminal() bool {
	return t.status == StatusProcessed || t.status == StatusRejected || t.status == StatusFailed
}

// Transição de Estado
func (t *WagerTransaction) MarkAsPedingReference() error {
	if t.IsTerminal() {
		return ErrTerminalState
	}
	
	t.status = StatusPedingReference
	t.updateAt = time.Now().UTC()
	return nil
}
func (t *WagerTransaction) MarkAsProcessed() error {
	if t.IsTerminal() {
		return ErrTerminalState
	}

	t.status = StatusProcessed
	t.updateAt = time.Now().UTC()
	return nil
}
func (t *WagerTransaction) MarkAsRejected(failureCode string) error {
	if t.IsTerminal() {
		return ErrTerminalState
	}

	t.status = StatusRejected
	t.failureCode = &failureCode
	t.updateAt = time.Now().UTC()
	return  nil
}
func (t *WagerTransaction) MarkAsFailed(failureCode string) error {
	if t.IsTerminal() {
		return ErrTerminalState
	}

	t.status = StatusFailed
	t.failureCode = &failureCode
	t.updateAt = time.Now().UTC()
	return nil
}

// Gera a hash
func CalculatePayloadHash(providerID, externalTxID, playerID, walletID, roundID, gameID, kind, amount, currency string, refExternalID *string) string {
	refStr := ""
	if refExternalID != nil {
		refStr = *refExternalID
	}

	raw := fmt.Sprintf("ampint=%s|currency=%s|externalTransactionId=%s|gameId=%s|kind=%s|playerId=%s|providerId=%s|referenceExternalTransactionId=%s|roundId=%s|walletId=%s", amount,currency, externalTxID, gameID, kind, playerID, providerID, refStr, roundID, walletID)

	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}