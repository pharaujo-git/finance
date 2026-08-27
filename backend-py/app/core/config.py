"""Reads the process environment into a validated Settings object.

Variable names and fallbacks are identical to the .NET and Go APIs', so one
deployment environment can drive any of the three backends.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Final

DATABASE_URL_VARIABLE: Final = "DATABASE_URL"
JWT_SECRET_VARIABLE: Final = "JWT_SECRET"
PORT_VARIABLE: Final = "PORT"
ALLOWED_ORIGINS_VARIABLE: Final = "ALLOWED_ORIGINS"

# Must stay byte-identical to JwtOptions.LocalDevelopmentSecret in the .NET API
# and config.LocalDevelopmentSecret in the Go one, or locally issued tokens stop
# crossing backends.
LOCAL_DEVELOPMENT_SECRET: Final = "finance-tracker-local-development-signing-key-please-override"

DEFAULT_ALLOWED_ORIGINS: Final = "http://localhost:5173"

# Keeps this API off the .NET (5000) and Go (8081) ports during parity runs.
DEFAULT_PORT: Final = 8082


class ConfigError(Exception):
    """Raised when the environment cannot produce a usable configuration."""


@dataclass(frozen=True, slots=True)
class Settings:
    """The fully resolved runtime configuration."""

    database_url: str
    jwt_secret: str
    port: int
    allowed_origins: list[str]


def parse_origins(raw: str | None) -> list[str]:
    """Splits the comma-separated origin list, falling back to the default."""
    value = raw if raw and raw.strip() else DEFAULT_ALLOWED_ORIGINS
    origins = [part.strip() for part in value.split(",") if part.strip()]
    return origins or [DEFAULT_ALLOWED_ORIGINS]


def load_settings(env: dict[str, str] | None = None) -> Settings:
    """Reads the environment. DATABASE_URL is required; the rest have defaults."""
    source = env if env is not None else dict(os.environ)

    database_url = (source.get(DATABASE_URL_VARIABLE) or "").strip()
    if not database_url:
        raise ConfigError(
            f"config: {DATABASE_URL_VARIABLE} is required (postgres:// connection string)"
        )

    secret = (source.get(JWT_SECRET_VARIABLE) or "").strip() or LOCAL_DEVELOPMENT_SECRET

    port = DEFAULT_PORT
    raw_port = (source.get(PORT_VARIABLE) or "").strip()
    if raw_port:
        try:
            port = int(raw_port)
        except ValueError as exc:
            raise ConfigError(
                f"config: {PORT_VARIABLE} must be a TCP port number, got {raw_port}"
            ) from exc
        if not 0 < port <= 65535:
            raise ConfigError(f"config: {PORT_VARIABLE} must be a TCP port number, got {raw_port}")

    return Settings(
        database_url=database_url,
        jwt_secret=secret,
        port=port,
        allowed_origins=parse_origins(source.get(ALLOWED_ORIGINS_VARIABLE)),
    )
