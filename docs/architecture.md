# Architecture

ms-go-user owns user lifecycle/status and profile data. Authentication credentials and tokens were split to ms-go-auth by migration 0006; roles belong to ms-go-rbac; file bytes and image derivatives belong to their respective services.

HTTP adapters expose authenticated user routes, admin/moderator routes, and a token-protected internal active-user batch. Core NATS user.create-user provisions an idempotent user/profile pair and may import missing OAuth profile fields. Provider avatars are limited to HTTPS allowlisted hosts, 5 MB, and JPEG/PNG/WebP before storage.

Two ownership conflicts remain. user_identity/user_provider and user-facing identity endpoints coexist with ms-go-auth auth_identity. Also AuthMiddleware accepts X-User-Id and X-User-Role as trusted without checking the caller/proxy. Direct service exposure therefore permits header spoofing. These are current risks requiring separate compatibility and security work.
