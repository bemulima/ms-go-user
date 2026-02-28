ALTER TABLE user_profile
    ADD COLUMN IF NOT EXISTS avatar_file_id text;

UPDATE user_profile
SET avatar_file_id = regexp_replace(avatar_url, '^.*/files/([^/]+)/download.*$', '\1')
WHERE avatar_file_id IS NULL
  AND avatar_url IS NOT NULL
  AND avatar_url ~ '/files/[^/]+/download';

ALTER TABLE user_profile
    DROP COLUMN IF EXISTS avatar_url;
