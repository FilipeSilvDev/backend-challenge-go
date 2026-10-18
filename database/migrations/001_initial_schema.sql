-- Descrição: Criação das tabelas principais para carteiras, transações de apostas, ledger, inbox e outbox.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Tabela de Carteiras
CREATE TABLE IF NOT EXISTS wallets (
    id VARCHAR(64) PRIMARY KEY,
    player_id VARCHAR(64) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    balance_amount BIGINT NOT NULL CHECK (balance_amount >= 0),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uk_player_currency UNIQUE (player_id, currency) -- Invariante: Um jogador possui apenas uma carteira por moeda
);

-- 2. Tabela de Transação de Apostas
CREATE TABLE IF NOT EXISTS wager_transactions (
    id VARCHAR(64) PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    external_transaction_id VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    payload_hash VARCHAR(64) NOT NULL,
    wallet_id VARCHAR(64) NOT NULL REFERENCES wallets(id),
    player_id VARCHAR(64) NOT NULL,
    round_id VARCHAR(64) NOT NULL,
    game_id VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    reference_external_transaction_id VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    failure_code VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uk_provider_external_tx UNIQUE (provider_id, external_transaction_id) -- Evita duplicidade de transações por provedor
);

 -- Índice para busca rápida de referências pendentes ou histórico
CREATE INDEX idx_wager_tx_provider_ref ON wager_transactions(provider_id, reference_external_transaction_id);

-- 3. Tabela de Extratos
CREATE TABLE IF NOT EXISTS wallet_ledger_entries (
    id VARCHAR(64) PRIMARY KEY,
    wallet_id VARCHAR(64) NOT NULL REFERENCES wallets(id),
    transaction_id VARCHAR(64) NOT NULL REFERENCES wager_transactions(id),
    direction VARCHAR(16) NOT NULL, -- DEBIT, CREDIT
    amount BIGINT NOT NULL CHECK (amount > 0),
    balance_before BIGINT NOT NULL CHECK (balance_before >= 0),
    balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Imutabilidade: transação gera no máximo um lançamento por carteira
    CONSTRAINT uk_wallet_transaction UNIQUE (wallet_id, transaction_id)
);

-- 4. Tabela de Inbox
CREATE TABLE IF NOT EXISTS inbox_messages (
    consumer_name VARCHAR(64) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (consumer_name, message_id)
);

-- 5. Tabela de Outbox
CREATE TABLE IF NOT EXISTS outbox_events (
    id VARCHAR(64) PRIMARY KEY,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_delivery_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Índice para outbox buscar eventos pendentes de forma eficiente
CREATE INDEX idx_outbox_peding ON outbox_events(next_delivery_at) WHERE published_at IS NULL;