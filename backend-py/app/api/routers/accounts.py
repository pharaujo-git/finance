"""/api/accounts."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Request, Response

from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_account
from app.core.deps import CallerId, Connection
from app.repositories.accounts import AccountRepository
from app.repositories.transactions import TransactionRepository
from app.services.accounts import AccountService

router = APIRouter(prefix="/accounts", tags=["accounts"])


def _service(conn: Connection) -> AccountService:
    return AccountService(AccountRepository(conn), TransactionRepository(conn))


@router.get("", response_class=ApiResponse)
async def list_accounts(caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).list_all(caller))


@router.post("", response_class=ApiResponse)
async def create_account(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_account(await read_json(request))
    assert body.type is not None
    result = await _service(conn).create(
        caller,
        name=body.name,
        account_type=body.type,
        initial_balance=body.initial_balance,
        currency=body.currency,
    )
    return ApiResponse(result)


@router.get("/{account_id}", response_class=ApiResponse)
async def get_account(account_id: uuid.UUID, caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).get(caller, account_id))


@router.put("/{account_id}", response_class=ApiResponse)
async def update_account(
    account_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_account(await read_json(request))
    assert body.type is not None
    result = await _service(conn).update(
        caller,
        account_id,
        name=body.name,
        account_type=body.type,
        currency=body.currency,
        is_archived=body.is_archived,
    )
    return ApiResponse(result)


@router.delete("/{account_id}", status_code=204)
async def archive_account(account_id: uuid.UUID, caller: CallerId, conn: Connection) -> Response:
    await _service(conn).archive(caller, account_id)
    return Response(status_code=204)
