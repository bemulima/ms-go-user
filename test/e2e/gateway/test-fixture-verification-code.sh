#!/usr/bin/env bash
set -euo pipefail
set +x
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 - "${script_dir}" <<'PYTEST'
import os
from pathlib import Path
import subprocess
import sys
import tempfile

root = Path(sys.argv[1])
base = {key: value for key, value in os.environ.items() if not key.startswith('USER_E2E_')}
helper = root / 'fixture-verification-code.sh'
script = 'source "$1"; fixture_verification_code fixture@example.test'
with tempfile.TemporaryDirectory(prefix='verification-observer-test-') as directory:
    directory = Path(directory)
    marker = directory / 'called'
    command = directory / 'mailbox-observer'
    def fixture(body):
        command.write_text('#!/bin/sh\n' + body)
        command.chmod(0o700)
    def invoke(extra):
        return subprocess.run(['bash', '-c', script, 'test', str(helper)], env={**base, **extra}, text=True, capture_output=True)
    config = {'USER_E2E_ISOLATED_FIXTURE': 'true', 'USER_E2E_VERIFICATION_CODE_COMMAND': str(command)}
    fixture(f'touch "{marker}"\nprintf 0427\n')
    result = invoke({'USER_E2E_VERIFICATION_CODE_COMMAND': str(command)})
    assert result.returncode != 0 and not marker.exists() and not result.stdout, 'observer ran without isolated fixture attestation'
    result = invoke({'USER_E2E_ISOLATED_FIXTURE': 'true'})
    assert result.returncode != 0 and not marker.exists() and not result.stdout, 'missing command accepted'
    fixture('test "$1" = fixture@example.test || exit 1\nprintf 0427\n')
    result = invoke(config)
    assert result.returncode == 0 and result.stdout == '0427', 'observer input or leading-zero proof changed'
    fixture('printf synthetic-private-diagnostic >&2\nprintf invalid-private-output\n')
    result = invoke(config)
    assert result.returncode != 0 and not result.stdout and 'private' not in result.stderr, 'invalid output or hook stderr leaked'
    fixture('printf synthetic-private-diagnostic >&2\nexit 1\n')
    result = invoke(config)
    assert result.returncode != 0 and not result.stdout and 'private' not in result.stderr, 'failing hook stderr leaked'
    result = subprocess.run(['bash', str(root/'run-tests.sh')], env=base, text=True, capture_output=True)
    assert result.returncode != 0 and 'USER_E2E_ISOLATED_FIXTURE=true is required' in result.stderr, 'runner failed to preflight before Gateway access'
for path in (root/'scenarios').glob('*.sh'):
    text = path.read_text()
    assert '/api/tarantool' not in text and 'TARANTOOL_API' not in text, f'{path.name}: retired dependency'
    assert 'resp=${' not in text, f'{path.name}: response logging'
print('isolated verification delivery observer regression passed')
PYTEST
