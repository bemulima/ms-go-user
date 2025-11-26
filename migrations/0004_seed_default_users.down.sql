DELETE FROM user_profile WHERE user_id IN (
    SELECT id FROM "user" WHERE email IN (
        'admin@example.com',
        'manager@example.com',
        'teacher@example.com',
        'student1@example.com',
        'student2@example.com',
        'student3@example.com',
        'user@example.com'
    )
);

DELETE FROM "user" WHERE email IN (
    'admin@example.com',
    'manager@example.com',
    'teacher@example.com',
    'student1@example.com',
    'student2@example.com',
    'student3@example.com',
    'user@example.com'
);
