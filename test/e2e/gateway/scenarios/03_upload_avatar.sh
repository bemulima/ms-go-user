#!/usr/bin/env bash
set -euo pipefail

wiki_ref="wiki/USER_PROFILE.md (Загрузка аватара)"
USER_API="/api/user/v1/users"

token="${E2E_USER_ACCESS_TOKEN:-}"
if [[ -z "${token}" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref}" "есть access token из сценария 01" "token отсутствует" "env E2E_USER_ACCESS_TOKEN empty" "blocker" "ms-go-user/test/e2e"
  return 0
fi

b64decode() {
  if base64 --help 2>&1 | grep -q -- " -d"; then
    base64 -d
  else
    base64 -D
  fi
}

tmp_png="$(mktemp)"
trap 'rm -f "${tmp_png}"' EXIT

# 1x1 transparent PNG
cat <<'B64' | b64decode >"${tmp_png}"
iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO8h9SgAAAAASUVORK5CYII=
B64

tmp_resp="$(mktemp)"
trap 'rm -f "${tmp_png}" "${tmp_resp}"' EXIT

code="$(curl -sS --max-time "${HTTP_TIMEOUT}" \
  -o "${tmp_resp}" -w "%{http_code}" \
  -X POST \
  -H "Authorization: Bearer ${token}" \
  -F "file=@${tmp_png};type=image/png" \
  -F "processing_mode=EAGER" \
  "${GATEWAY_URL}${USER_API}/me/avatar" || true)"

resp="$(cat "${tmp_resp}")"

if [[ "${code}" != "201" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref}" "HTTP 201" "HTTP ${code}" "POST ${USER_API}/me/avatar resp=${resp}" "blocker" "ms-go-user/ms-go-filestorage/ms-go-image-processor/ms-getway"
  return 0
fi

if ! echo "${resp}" | jq -e '(.download_url? or .data.download_url?) and (.profile.avatar_url? or .data.profile.avatar_url? or .data.profile.avatarURL?)' >/dev/null 2>&1; then
  record_mismatch "ms-go-user" "${wiki_ref}" "response содержит download_url + profile.avatar_url" "поля отсутствуют" "POST ${USER_API}/me/avatar resp=${resp}" "major" "ms-go-user"
  return 0
fi

record_ok "user upload avatar returns 201 and response contains urls"
return 0
