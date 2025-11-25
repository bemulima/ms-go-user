#!/usr/bin/env bash
set -euo pipefail

# Simple end-to-end smoke for avatar upload + variant generation across user-service, filestorage, image-processor.
# Requires the microservices repo layout (ms-go-user, ms-go-filestorage, ms-go-image-processor) to be siblings.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FS_DIR="${ROOT}/ms-go-filestorage"
IMG_DIR="${ROOT}/ms-go-image-processor"
USER_DIR="${ROOT}/ms-go-user"

PROJECT="e2e-avatar"

JWT="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEiLCJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20iLCJleHAiOjQ3NjQwOTQyNTB9.EJ4foxdkHMSE7iSBWN26XNmmTUffQto7kTy4nsiRnZs"

cleanup() {
  docker compose -p "${PROJECT}-fs" -f "${FS_DIR}/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1 || true
  docker compose -p "${PROJECT}-img" -f "${IMG_DIR}/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1 || true
  docker compose -p "${PROJECT}-user" -f "${USER_DIR}/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "[+] Building fixture image..."
FIXTURE="/tmp/avatar-e2e.png"
cat > /tmp/gen-avatar-e2e.go <<'EOF'
package main
import (
  "image"
  "image/color"
  "image/png"
  "os"
)
func main() {
  img := image.NewRGBA(image.Rect(0,0,32,32))
  c := color.RGBA{255,0,0,255}
  for y:=0; y<32; y++ { for x:=0; x<32; x++ { img.Set(x,y,c) } }
  f,_ := os.Create(os.Args[1])
  png.Encode(f, img)
  f.Close()
}
EOF
GOCACHE=/tmp/gocache go run /tmp/gen-avatar-e2e.go "${FIXTURE}"
rm /tmp/gen-avatar-e2e.go

if ! command -v jq >/dev/null 2>&1; then
  echo "[-] jq is required on host to parse responses"
  exit 1
fi

echo "[+] Starting filestorage stack..."
MINIO_ENDPOINT=host.docker.internal:9000 COMPOSE_PROJECT_NAME=${PROJECT}-fs docker compose -f "${FS_DIR}/docker-compose.yml" up -d --build ms-filestorage

echo "[+] Starting image-processor stack..."
FILESTORAGE_URL=http://host.docker.internal:8088 DEFAULT_VARIANT_KIND=USER_MEDIA COMPOSE_PROJECT_NAME=${PROJECT}-img docker compose -f "${IMG_DIR}/docker-compose.yml" up -d --build ms-image-processor

echo "[+] Starting user-service stack..."
MS_FILESTORAGE_URL=http://host.docker.internal:8088 MS_IMAGE_PROCESSOR_URL=http://host.docker.internal:8084 AVATAR_PRESET_GROUP=avatar AVATAR_FILE_KIND=USER_MEDIA COMPOSE_PROJECT_NAME=${PROJECT}-user docker compose -f "${USER_DIR}/docker-compose.yml" up -d --build user-service

echo "[+] Applying migrations for user-service..."
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb < "${USER_DIR}/migrations/0001_init.up.sql"
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb < "${USER_DIR}/migrations/0002_user_identities.up.sql"
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb < "${USER_DIR}/migrations/0002_user_provider.up.sql"
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb < "${USER_DIR}/migrations/0003_user_status.up.sql"

echo "[+] Applying migrations for image-processor..."
docker compose -p ${PROJECT}-img -f "${IMG_DIR}/docker-compose.yml" exec -T postgres psql -U imageproc -d imageproc < "${IMG_DIR}/migrations/001_init.sql"

echo "[+] Seeding test user..."
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb -c "INSERT INTO \\\"user\\\" (id,email,is_active,status) VALUES ('11111111-1111-1111-1111-111111111111','user@example.com',true,'active') ON CONFLICT (id) DO NOTHING;"
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T postgres psql -U app -d userdb -c "INSERT INTO user_profile (user_id) VALUES ('11111111-1111-1111-1111-111111111111') ON CONFLICT (user_id) DO NOTHING;"

echo "[+] Waiting for services..."
sleep 5

echo "[+] Uploading avatar..."
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T nginx sh -c "cat > /tmp/avatar.png" < "${FIXTURE}"
UPLOAD_RES=$(docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T nginx sh -c "curl -s -f -X POST http://user-service:8080/users/me/avatar -H 'Authorization: Bearer ${JWT}' -F file=@/tmp/avatar.png -F processing_mode=EAGER" || true)
FILE_ID=$(echo "$UPLOAD_RES" | jq -r '.data.file_id // empty')
if [[ -z "$FILE_ID" ]]; then
  echo "[-] Upload failed: $UPLOAD_RES"
  exit 1
fi
echo "[+] Uploaded file_id=$FILE_ID"

echo "[+] Forcing variant generation..."
docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T nginx sh -c "curl -s -f -X POST http://host.docker.internal:8084/admin/images/${FILE_ID}/variants/generate -H 'Content-Type: application/json' -d '{\"preset_group\":\"avatar\",\"owner_id\":\"11111111-1111-1111-1111-111111111111\",\"file_kind\":\"USER_MEDIA\"}'" >/tmp/variant-gen.json
cat /tmp/variant-gen.json

echo "[+] Checking variants..."
VARIANTS=$(docker compose -p ${PROJECT}-user -f "${USER_DIR}/docker-compose.yml" exec -T nginx sh -c "curl -s -f http://host.docker.internal:8084/admin/images/${FILE_ID}/variants" || true)
COUNT=$(echo "$VARIANTS" | jq '.variants | length')
if [[ "$COUNT" -lt 1 ]]; then
  echo "[-] Variants not created: $VARIANTS"
  exit 1
fi
echo "[+] Variants created: $VARIANTS"
echo "[+] E2E avatar upload passed."
