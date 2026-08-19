-- Re-key users on the Telegram id instead of a generated UUID. Done in place so
-- existing rows survive: the telegram_id column becomes the primary key.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
DROP INDEX IF EXISTS users_telegram_id_idx;
ALTER TABLE users DROP COLUMN IF EXISTS id;
ALTER TABLE users RENAME COLUMN telegram_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);
