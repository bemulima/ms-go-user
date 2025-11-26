-- Reset seeded users to deterministic UUIDs and backfill profiles.
BEGIN;

DELETE FROM user_profile
WHERE user_id IN (SELECT id FROM "user" WHERE email IN (
    'admin@example.com',
    'manager@example.com',
    'teacher@example.com',
    'student1@example.com',
    'student2@example.com',
    'student3@example.com',
    'user@example.com'
));

DELETE FROM "user"
WHERE email IN (
    'admin@example.com',
    'manager@example.com',
    'teacher@example.com',
    'student1@example.com',
    'student2@example.com',
    'student3@example.com',
    'user@example.com'
);

INSERT INTO "user" (id, email, password_hash, status, is_active)
VALUES
    ('00000000-0000-0000-0000-0000000000a1', 'admin@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000a2', 'manager@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000a3', 'teacher@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b1', 'student1@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b2', 'student2@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000b3', 'student3@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true),
    ('00000000-0000-0000-0000-0000000000c1', 'user@example.com', '$2a$10$7xzTCUMmJ9GY1XkWWPgr7OWX7U0rM4SgWJPfdYq0MKzpv64C.fwd2', 'ACTIVE', true);

INSERT INTO user_profile (user_id, display_name, avatar_url)
VALUES
    ('00000000-0000-0000-0000-0000000000a1', 'Admin', NULL),
    ('00000000-0000-0000-0000-0000000000a2', 'Manager', NULL),
    ('00000000-0000-0000-0000-0000000000a3', 'Teacher', NULL),
    ('00000000-0000-0000-0000-0000000000b1', 'Student One', NULL),
    ('00000000-0000-0000-0000-0000000000b2', 'Student Two', NULL),
    ('00000000-0000-0000-0000-0000000000b3', 'Student Three', NULL),
    ('00000000-0000-0000-0000-0000000000c1', 'User', NULL);

COMMIT;
