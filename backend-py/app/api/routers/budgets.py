"""/api/budgets."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Request, Response

from app.api.query import QueryReader
from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_create_budget, parse_update_budget
from app.core.deps import CallerId, Connection
from app.repositories.budgets import BudgetRepository
from app.repositories.categories import CategoryRepository
from app.repositories.transactions import TransactionRepository
from app.services.budgets import BudgetService
from app.services.categories import CategoryService

router = APIRouter(prefix="/budgets", tags=["budgets"])


def _service(conn: Connection) -> BudgetService:
    return BudgetService(
        BudgetRepository(conn),
        TransactionRepository(conn),
        CategoryService(CategoryRepository(conn)),
    )


@router.get("", response_class=ApiResponse)
async def list_budgets(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    # An absent key means "this month"; an empty one is a validation failure.
    month = QueryReader(request).text("month")
    return ApiResponse(await _service(conn).list_all(caller, month))


@router.post("", response_class=ApiResponse)
async def create_budget(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_create_budget(await read_json(request))
    assert body.category_id is not None
    assert body.limit is not None
    result = await _service(conn).create(
        caller, category_id=body.category_id, month=body.month, limit=body.limit
    )
    return ApiResponse(result)


@router.put("/{budget_id}", response_class=ApiResponse)
async def update_budget(
    budget_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_update_budget(await read_json(request))
    assert body.limit is not None
    return ApiResponse(await _service(conn).update(caller, budget_id, body.limit))


@router.delete("/{budget_id}", status_code=204)
async def delete_budget(budget_id: uuid.UUID, caller: CallerId, conn: Connection) -> Response:
    await _service(conn).delete(caller, budget_id)
    return Response(status_code=204)
