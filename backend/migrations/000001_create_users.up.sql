CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY,
    telegram_id   BIGINT       NOT NULL,
    username      TEXT         NOT NULL DEFAULT '',
    first_name    TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS users_telegram_id_idx ON users (telegram_id);
