#!/usr/bin/env bash
set -euo pipefail

wiki_ref="wiki/USER_PROFILE.md"

AUTH_API="/api/auth/v1/auth"
TARANTOOL_API="/api/tarantool/v1"
USER_API="/api/user/v1/users"

email="${E2E_EMAIL}"
password="${E2E_PASSWORD}"

# Подготовка: создаём пользователя через auth (по wiki это отдельный раздел, но для user-profile E2E нужен токен).
start_body="$(jq -nc --arg email "${email}" --arg password "${password}" '{email:$email,password:$password}')"
start_raw="$(http_json POST "${AUTH_API}/signup/start" "${start_body}")"
start_status="$(printf '%s\n' "${start_raw}" | extract_status)"
start_resp="$(printf '%s\n' "${start_raw}" | extract_body)"
if [[ "${start_status}" != "202" ]]; then
  record_mismatch "ms-go-auth" "wiki/AUTHENTICATION.md (signup/start)" "HTTP 202" "HTTP ${start_status}" "POST ${AUTH_API}/signup/start resp=${start_resp}" "blocker" "ms-go-auth"
  return 1
fi
record_ok "auth signup/start returns 202"

tara_body="$(jq -nc --arg email "${email}" '{value:{email:$email,password:""}}')"
tara_raw="$(http_json POST "${TARANTOOL_API}/set-new-user" "${tara_body}")"
tara_status="$(printf '%s\n' "${tara_raw}" | extract_status)"
tara_resp="$(printf '%s\n' "${tara_raw}" | extract_body)"
if [[ "${tara_status}" != "200" ]]; then
  record_mismatch "ms-go-tarantool" "wiki/AUTHENTICATION.md (signup/code)" "HTTP 200" "HTTP ${tara_status}" "POST ${TARANTOOL_API}/set-new-user resp=${tara_resp}" "blocker" "ms-go-tarantool"
  return 1
fi
record_ok "tarantool set-new-user returns 200"
code="$(echo "${tara_resp}" | jq -r '.code // empty')"
if [[ -z "${code}" ]]; then
  record_mismatch "ms-go-tarantool" "wiki/AUTHENTICATION.md (signup/code)" "в E2E доступен code (APP_ENV=integration)" "code отсутствует в ответе" "POST ${TARANTOOL_API}/set-new-user resp=${tara_resp}" "blocker" "ms-go-tarantool"
  return 1
fi
record_ok "tarantool provides verification code"

verify_body="$(jq -nc --arg email "${email}" --arg code "${code}" '{email:$email,code:$code}')"
verify_raw="$(http_json POST "${AUTH_API}/signup/verify" "${verify_body}")"
verify_status="$(printf '%s\n' "${verify_raw}" | extract_status)"
verify_resp="$(printf '%s\n' "${verify_raw}" | extract_body)"
if [[ "${verify_status}" != "200" ]]; then
  record_mismatch "ms-go-auth" "wiki/AUTHENTICATION.md (signup/verify)" "HTTP 200" "HTTP ${verify_status}" "POST ${AUTH_API}/signup/verify resp=${verify_resp}" "blocker" "ms-go-auth"
  return 1
fi
record_ok "auth signup/verify returns 200"

token="$(echo "${verify_resp}" | jq -r '.access_token // empty')"
if [[ -z "${token}" ]]; then
  record_mismatch "ms-go-auth" "wiki/AUTHENTICATION.md (tokens)" "access_token присутствует" "access_token отсутствует" "POST ${AUTH_API}/signup/verify resp=${verify_resp}" "blocker" "ms-go-auth"
  return 1
fi
record_ok "auth returns access token"
export E2E_USER_ACCESS_TOKEN="${token}"

# 1) GET /users/me
me_raw="$(http_json GET "${USER_API}/me" "" "${token}")"
me_status="$(printf '%s\n' "${me_raw}" | extract_status)"
me_resp="$(printf '%s\n' "${me_raw}" | extract_body)"

if [[ "${me_status}" != "200" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref} (GET /users/me)" "HTTP 200 + профиль" "HTTP ${me_status}" "GET ${USER_API}/me resp=${me_resp}" "blocker" "ms-go-user"
  return 0
fi
record_ok "user get /me returns 200"

if ! echo "${me_resp}" | jq -e '(.id? and .email?) or (.data.id? and .data.email?)' >/dev/null 2>&1; then
  record_mismatch "ms-go-user" "${wiki_ref} (GET /users/me)" "response содержит id/email" "id/email отсутствуют" "GET ${USER_API}/me → 200 resp=${me_resp}" "major" "ms-go-user"
  return 0
fi
record_ok "user /me payload contains id+email"

me_id="$(echo "${me_resp}" | jq -r '.id // .data.id // empty')"
if [[ -n "${me_id}" ]]; then
  export E2E_USER_ID="${me_id}"
fi

# 2) Негатив: без токена должно быть 401
unauth_raw="$(http_json GET "${USER_API}/me")"
unauth_status="$(printf '%s\n' "${unauth_raw}" | extract_status)"
unauth_resp="$(printf '%s\n' "${unauth_raw}" | extract_body)"
if [[ "${unauth_status}" != "401" ]]; then
  record_mismatch "ms-go-user" "${wiki_ref} (GET /users/me)" "HTTP 401 без токена" "HTTP ${unauth_status}" "GET ${USER_API}/me (no auth) resp=${unauth_resp}" "major" "ms-go-user/ms-getway"
else
  record_ok "user /me without token returns 401"
fi

return 0
