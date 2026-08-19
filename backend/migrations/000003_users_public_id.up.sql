-- Opaque identifier published to other players, so the Telegram id in `id`
-- never leaves the server. Backfilled for existing rows.
ALTER TABLE users ADD COLUMN IF NOT EXISTS public_id UUID NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX IF NOT EXISTS users_public_id_idx ON users (public_id);
