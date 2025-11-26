INSERT INTO "user" (id, email, password_hash, status, is_active)
VALUES
    ('00000000-0000-0000-0000-0000000000a1', 'admin@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000a2', 'manager@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000a3', 'teacher@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b1', 'student1@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b2', 'student2@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b3', 'student3@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000c1', 'user@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true)
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Admin', NULL FROM "user" WHERE email = 'admin@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Manager', NULL FROM "user" WHERE email = 'manager@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Teacher', NULL FROM "user" WHERE email = 'teacher@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Student One', NULL FROM "user" WHERE email = 'student1@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Student Two', NULL FROM "user" WHERE email = 'student2@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'Student Three', NULL FROM "user" WHERE email = 'student3@example.com'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_profile (user_id, display_name, avatar_url)
SELECT id, 'User', NULL FROM "user" WHERE email = 'user@example.com'
ON CONFLICT (user_id) DO NOTHING;
