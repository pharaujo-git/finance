"""Dependency-injection wiring: the pool, the token service, the caller."""

from __future__ import annotations

import uuid
from collections.abc import AsyncIterator
from typing import Annotated

import asyncpg
from fastapi import Depends, Request

from app.core.errors import unauthorized
from app.core.security import InvalidTokenError, Principal, TokenService

# The 401 details the Go API renders, kept identical so a client cannot tell the
# backends apart by their error text.
_MISSING_TOKEN_DETAIL = "Authentication is required."
_BAD_TOKEN_DETAIL = "The access token is invalid or has expired."

_BEARER_PREFIX = "Bearer "


def get_pool(request: Request) -> asyncpg.Pool:
    return request.app.state.pool


def get_tokens(request: Request) -> TokenService:
    return request.app.state.tokens


async def get_connection(
    pool: Annotated[asyncpg.Pool, Depends(get_pool)],
) -> AsyncIterator[asyncpg.Connection]:
    """Borrows a connection for the lifetime of one request."""
    async with pool.acquire() as connection:
        yield connection


def current_principal(
    request: Request,
    tokens: Annotated[TokenService, Depends(get_tokens)],
) -> Principal:
    """Validates the bearer token, raising the 401 problem on any failure."""
    header = request.headers.get("Authorization", "")
    if not header.startswith(_BEARER_PREFIX):
        raise unauthorized(_MISSING_TOKEN_DETAIL)

    raw = header[len(_BEARER_PREFIX) :].strip()
    if not raw:
        raise unauthorized(_MISSING_TOKEN_DETAIL)

    try:
        return tokens.validate(raw)
    except InvalidTokenError as exc:
        raise unauthorized(_BAD_TOKEN_DETAIL) from exc


def current_user_id(
    principal: Annotated[Principal, Depends(current_principal)],
) -> uuid.UUID:
    """The authenticated caller's id, which is all most handlers need."""
    return principal.user_id


Connection = Annotated[asyncpg.Connection, Depends(get_connection)]
CallerId = Annotated[uuid.UUID, Depends(current_user_id)]
Tokens = Annotated[TokenService, Depends(get_tokens)]
