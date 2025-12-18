#!/usr/bin/env bash
set -euo pipefail

wiki_ref="wiki/USER_PROFILE.md (Обновление профиля)"
USER_API="/api/user/v1/users"

token="${E2E_USER_ACCESS_TOKEN:-}"
if [[ -z "${token}" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref}" "есть access token из сценария 01" "token отсутствует" "env E2E_USER_ACCESS_TOKEN empty" "blocker" "ms-go-user/test/e2e"
  return 0
fi

# Позитив: PATCH /users/me
display_name="E2E User $(date +%s)"
body="$(jq -nc --arg dn "${display_name}" '{display_name:$dn}')"
raw="$(http_json PATCH "${USER_API}/me" "${body}" "${token}")"
st="$(printf '%s\n' "${raw}" | extract_status)"
resp="$(printf '%s\n' "${raw}" | extract_body)"

if [[ "${st}" != "200" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref}" "HTTP 200" "HTTP ${st}" "PATCH ${USER_API}/me resp=${resp}" "blocker" "ms-go-user/ms-getway"
  return 0
fi

got_dn="$(echo "${resp}" | jq -r '.display_name // .data.display_name // empty')"
if [[ "${got_dn}" != "${display_name}" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref}" "response.display_name обновлён" "display_name не совпадает" "PATCH ${USER_API}/me resp=${resp}" "major" "ms-go-user"
  return 0
fi
record_ok "user update profile (display_name) returns 200 and persists"

# Негатив (wiki): avatar_url должен быть валидным URL → ожидаем 400 на невалидный.
bad_body="$(jq -nc '{avatar_url:"not-a-url"}')"
bad_raw="$(http_json PATCH "${USER_API}/me" "${bad_body}" "${token}")"
bad_st="$(printf '%s\n' "${bad_raw}" | extract_status)"
bad_resp="$(printf '%s\n' "${bad_raw}" | extract_body)"
if [[ "${bad_st}" == "400" ]]; then
  record_ok "user update profile rejects invalid avatar_url (400)"
else
  record_mismatch "ms-go-user" "${wiki_ref} (валидация avatar_url)" "HTTP 400" "HTTP ${bad_st}" "PATCH ${USER_API}/me (invalid avatar_url) resp=${bad_resp}" "minor" "ms-go-user"
fi

return 0

