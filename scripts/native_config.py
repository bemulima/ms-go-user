#!/usr/bin/env python3
"""Run a User Service command with native-development endpoints and local secrets."""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path
from urllib.parse import quote


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_INFRA_ENV_FILE = ROOT.parents[1] / "learning-platform-infrastructure" / ".env"
KEY = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def load_dotenv(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[7:].lstrip()
        key, separator, value = line.partition("=")
        if separator != "=" or not KEY.fullmatch(key.strip()):
            raise ValueError(f"unsupported dotenv syntax at {path}:{number}")
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        values[key.strip()] = value
    return values


def required(values: dict[str, str], name: str) -> str:
    value = values.get(name, "")
    if not value:
        raise ValueError(f"{name} is required by the approved infrastructure environment")
    return value


def native_environment() -> dict[str, str]:
    configured_path = os.environ.get("LW_INFRA_ENV_FILE")
    infra_env_file = Path(configured_path).expanduser() if configured_path else DEFAULT_INFRA_ENV_FILE
    values = load_dotenv(infra_env_file)
    values.update({name: value for name, value in os.environ.items() if name.startswith("USER_NATIVE_")})

    database = values.get("USER_NATIVE_DATABASE", "lw_user")
    postgres_host = values.get("USER_NATIVE_POSTGRES_HOST", "127.0.0.1")
    postgres_port = values.get("USER_NATIVE_POSTGRES_PORT", "5432")
    native_port = values.get("USER_NATIVE_HTTP_PORT", "18082")
    db_user = values.get("LW_USER_DB_USER", "lw_user")
    db_password = values.get("LW_USER_DB_PASSWORD") or required(values, "LW_POSTGRES_PASSWORD")
    db_dsn = values.get("USER_NATIVE_DB_DSN") or "postgres://{}:{}@{}:{}/{}?sslmode=disable".format(
        quote(db_user, safe=""), quote(db_password, safe=""), postgres_host, postgres_port, database
    )

    environment = dict(os.environ)
    environment.update(
        {
            "APP_ENV": values.get("USER_NATIVE_APP_ENV", "native"),
            "APP_HOST": values.get("USER_NATIVE_HTTP_HOST", "127.0.0.1"),
            "APP_PORT": native_port,
            "APP_PUBLIC_URL": values.get("USER_NATIVE_APP_PUBLIC_URL", f"http://127.0.0.1:{native_port}"),
            "DB_HOST": postgres_host,
            "DB_PORT": postgres_port,
            "DB_USER": db_user,
            "DB_PASSWORD": db_password,
            "DB_NAME": database,
            "DB_SSLMODE": values.get("USER_NATIVE_DB_SSLMODE", "disable"),
            "DB_DSN": db_dsn,
            "NATS_URL": values.get("USER_NATIVE_NATS_URL", "nats://127.0.0.1:4222"),
            "MS_RBAC": values.get("USER_NATIVE_RBAC_URL", "http://127.0.0.1:18080/api/v1"),
            "MS_FILESTORAGE_URL": values.get("USER_NATIVE_FILESTORAGE_URL", "http://127.0.0.1:18085"),
            "FILESTORAGE_INTERNAL_TOKEN": values.get("USER_NATIVE_FILESTORAGE_INTERNAL_TOKEN") or required(values, "LW_FILESTORAGE_USER_TOKEN"),
            "IMAGE_PROCESSOR_SERVICE_BASE_URL": values.get("USER_NATIVE_IMAGE_PROCESSOR_URL", "http://127.0.0.1:18086"),
            "IMAGE_PROCESSOR_INTERNAL_TOKEN": values.get("USER_NATIVE_IMAGE_PROCESSOR_INTERNAL_TOKEN") or required(values, "LW_USER_IMAGE_PROCESSOR_TOKEN"),
            "MS_TARANTOOL_URL": values.get("USER_NATIVE_TARANTOOL_URL", ""),
            "JWT_SECRET": values.get("USER_NATIVE_JWT_SECRET") or required(values, "LW_AUTH_JWT_SECRET"),
            "JWT_ISSUER": values.get("USER_NATIVE_JWT_ISSUER", "lw-auth"),
            "JWT_AUDIENCE": values.get("USER_NATIVE_JWT_AUDIENCE", "frontend"),
            "INTERNAL_API_TOKEN": values.get("USER_NATIVE_INTERNAL_API_TOKEN") or required(values, "LW_INTERNAL_TOKEN"),
        }
    )
    print(
        "native User Service config: database={} postgres={}:{} nats={} rbac={} http={}:{} filestorage={} image_processor={}".format(
            database,
            postgres_host,
            postgres_port,
            environment["NATS_URL"],
            environment["MS_RBAC"],
            environment["APP_HOST"],
            native_port,
            environment["MS_FILESTORAGE_URL"],
            environment["IMAGE_PROCESSOR_SERVICE_BASE_URL"],
        ),
        flush=True,
    )
    return environment


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("a command is required after --")
    try:
        environment = native_environment()
    except (OSError, ValueError) as error:
        print(f"native User Service configuration failed: {error}", file=sys.stderr)
        return 2
    os.execvpe(command[0], command, environment)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
