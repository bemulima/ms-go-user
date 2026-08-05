ALTER TABLE user_profile
    DROP CONSTRAINT IF EXISTS user_profile_gender_length_check,
    DROP CONSTRAINT IF EXISTS user_profile_birth_year_check,
    DROP COLUMN IF EXISTS gender,
    DROP COLUMN IF EXISTS birth_year,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name;
