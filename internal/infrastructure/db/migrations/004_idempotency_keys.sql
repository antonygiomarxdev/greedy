CREATE TABLE IF NOT EXISTS idempotency_keys (
    client_order_id   TEXT PRIMARY KEY,
    exchange_order_id TEXT,
    bot_id            TEXT NOT NULL,
    symbol            TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'reserved',
    created_at        INTEGER NOT NULL DEFAULT (unixepoch())
);
