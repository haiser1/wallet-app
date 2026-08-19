-- 001_init.sql
-- Wallet & Transaction System Schema

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- USERS
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(100) UNIQUE NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- WALLETS (one per user)
-- ============================================================================
CREATE TABLE IF NOT EXISTS wallets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    balance     BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency    VARCHAR(3) NOT NULL DEFAULT 'IDR',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);

-- ============================================================================
-- TRANSACTIONS (groups related ledger entries)
-- ============================================================================
CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            VARCHAR(20) NOT NULL CHECK (type IN ('topup', 'transfer', 'reversal')),
    reference_id    UUID REFERENCES transactions(id),
    idempotency_key VARCHAR(255),
    status          VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'reversed')),
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_idempotency_key
    ON transactions(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- ============================================================================
-- LEDGER ENTRIES (double-entry bookkeeping, append-only)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ledger_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    wallet_id       UUID NOT NULL REFERENCES wallets(id),
    entry_type      VARCHAR(6) NOT NULL CHECK (entry_type IN ('debit', 'credit')),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    balance_after   BIGINT NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet_id ON ledger_entries(wallet_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_transaction_id ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet_created ON ledger_entries(wallet_id, created_at DESC);

-- ============================================================================
-- IDEMPOTENCY KEYS (race-condition-proof via UNIQUE primary key)
-- ============================================================================
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key             VARCHAR(255) PRIMARY KEY,
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    response_code   INT NOT NULL,
    response_body   JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '24 hours'
);

-- ============================================================================
-- SYSTEM ACCOUNT (counter-party for top-ups)
-- ============================================================================
INSERT INTO users (id, username, email)
VALUES ('00000000-0000-0000-0000-000000000000', 'SYSTEM', 'system@internal')
ON CONFLICT (id) DO NOTHING;

INSERT INTO wallets (id, user_id, currency)
VALUES ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000000', 'IDR')
ON CONFLICT (user_id) DO NOTHING;
