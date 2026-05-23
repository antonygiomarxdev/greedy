CREATE TABLE IF NOT EXISTS credentials (
    exchange    TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT 'default',
    api_key     BLOB NOT NULL,
    api_secret  BLOB NOT NULL,
    passphrase  BLOB,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (exchange, label)
);
