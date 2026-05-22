CREATE TABLE IF NOT EXISTS exchange_balances (
    asset TEXT PRIMARY KEY,
    free REAL NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS exchange_positions (
    symbol TEXT PRIMARY KEY,
    quantity REAL NOT NULL,
    avg_entry_price REAL NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
