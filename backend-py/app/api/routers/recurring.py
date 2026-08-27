"""/api/recurring."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Request, Response

from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_recurring
from app.core.deps import CallerId, Connection
from app.repositories.accounts import AccountRepository
from app.repositories.categories import CategoryRepository
from app.repositories.recurring import RecurringRepository
from app.repositories.transactions import TransactionRepository
from app.services.categories import CategoryService
from app.services.recurring import RecurringService

router = APIRouter(prefix="/recurring", tags=["recurring"])


def _service(conn: Connection) -> RecurringService:
    return RecurringService(
        RecurringRepository(conn),
        TransactionRepository(conn),
        AccountRepository(conn),
        CategoryService(CategoryRepository(conn)),
    )


@router.get("", response_class=ApiResponse)
async def list_rules(caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).list_all(caller))


@router.post("", response_class=ApiResponse)
async def create_rule(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_recurring(await read_json(request))
    return ApiResponse(await _service(conn).create(caller, body))


@router.put("/{rule_id}", response_class=ApiResponse)
async def update_rule(
    rule_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_recurring(await read_json(request))
    return ApiResponse(await _service(conn).update(caller, rule_id, body))


@router.delete("/{rule_id}", status_code=204)
async def delete_rule(rule_id: uuid.UUID, caller: CallerId, conn: Connection) -> Response:
    await _service(conn).delete(caller, rule_id)
    return Response(status_code=204)
