#!/usr/bin/env bash
# Test-only delivery observer. This executable reads the isolated fixture mailbox.
set +x

check_verification_fixture() {
  if [[ "${USER_E2E_ISOLATED_FIXTURE:-}" != true ]]; then
    printf '%s\n' 'USER_E2E_ISOLATED_FIXTURE=true is required for signup fixtures' >&2
    return 1
  fi
  local command_path="${USER_E2E_VERIFICATION_CODE_COMMAND:-}"
  if [[ "$command_path" != /* || ! -f "$command_path" || ! -x "$command_path" ]]; then
    printf '%s\n' 'USER_E2E_VERIFICATION_CODE_COMMAND must be an absolute executable path' >&2
    return 1
  fi
}

fixture_verification_code() {
  check_verification_fixture || return 1
  local code
  if ! code="$("${USER_E2E_VERIFICATION_CODE_COMMAND}" "$1" 2>/dev/null)"; then
    printf '%s\n' 'isolated verification delivery observer failed' >&2
    return 1
  fi
  if [[ ! "$code" =~ ^[0-9]{4}$ ]]; then
    printf '%s\n' 'isolated verification delivery observer returned an invalid code' >&2
    return 1
  fi
  printf '%s' "$code"
}
