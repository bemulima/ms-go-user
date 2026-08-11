# HTTP contract

All /api/v1/users routes require AuthMiddleware. /admin/v1/users additionally requires admin or moderator via RBAC middleware. Self responses may include private status fields; other-user responses omit status/is_active and mask email.

PATCH /me accepts only display_name because its decoder rejects unknown fields. Avatar upload is limited to 5 MB and supports EAGER, LAZY, or DISABLED; the default is DISABLED. Remove identity returns HTTP 200 with status detached, not the 204 shown in the former wiki.

POST /internal/v1/users/active/resolve requires exact X-Internal-Token, a single strict JSON object, 1..1000 unique non-nil UUIDs, and a body at most 64 KiB. ACTIVE and NEW_USER rows with is_active true are active; result arrays preserve request order. The configured token currently defaults to change-me and must be overridden outside local development.
