ALTER TABLE user_profile
    ADD COLUMN IF NOT EXISTS avatar_url text;

UPDATE user_profile
SET avatar_url = '/files/' || avatar_file_id || '/download'
WHERE avatar_url IS NULL
  AND avatar_file_id IS NOT NULL
  AND trim(avatar_file_id) <> '';

ALTER TABLE user_profile
    DROP COLUMN IF EXISTS avatar_file_id;
