ALTER TABLE user_profile
    ADD COLUMN IF NOT EXISTS first_name text,
    ADD COLUMN IF NOT EXISTS last_name text,
    ADD COLUMN IF NOT EXISTS birth_year integer,
    ADD COLUMN IF NOT EXISTS gender text;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_profile_birth_year_check'
    ) THEN
        ALTER TABLE user_profile
            ADD CONSTRAINT user_profile_birth_year_check
                CHECK (birth_year IS NULL OR birth_year BETWEEN 1900 AND 9999);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_profile_gender_length_check'
    ) THEN
        ALTER TABLE user_profile
            ADD CONSTRAINT user_profile_gender_length_check
                CHECK (gender IS NULL OR char_length(gender) <= 64);
    END IF;
END
$$;
