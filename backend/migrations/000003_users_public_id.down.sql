DROP INDEX IF EXISTS users_public_id_idx;
ALTER TABLE users DROP COLUMN IF EXISTS public_id;
