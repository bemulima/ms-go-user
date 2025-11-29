ALTER TABLE "user" ADD COLUMN IF NOT EXISTS password_hash text;
ALTER TABLE "user" ALTER COLUMN email SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_email_key'
    ) THEN
        ALTER TABLE "user" ADD CONSTRAINT user_email_key UNIQUE (email);
    END IF;
END$$;
ALTER TABLE "user" ALTER COLUMN status SET DEFAULT 'ACTIVE';
UPDATE "user" SET status = 'ACTIVE' WHERE status = 'NEW_USER';
