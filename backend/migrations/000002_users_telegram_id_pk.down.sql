ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE users RENAME COLUMN id TO telegram_id;
ALTER TABLE users ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE users ADD PRIMARY KEY (id);
CREATE UNIQUE INDEX IF NOT EXISTS users_telegram_id_idx ON users (telegram_id);
