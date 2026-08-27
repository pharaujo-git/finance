"""/api/categories. There is deliberately no GET by id here."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Request, Response

from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_category
from app.core.deps import CallerId, Connection
from app.repositories.categories import CategoryRepository
from app.services.categories import CategoryService

router = APIRouter(prefix="/categories", tags=["categories"])


def _service(conn: Connection) -> CategoryService:
    return CategoryService(CategoryRepository(conn))


@router.get("", response_class=ApiResponse)
async def list_categories(caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).list_all(caller))


@router.post("", response_class=ApiResponse)
async def create_category(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_category(await read_json(request))
    assert body.type is not None
    result = await _service(conn).create(
        caller, name=body.name, category_type=body.type, icon=body.icon, color=body.color
    )
    return ApiResponse(result)


@router.put("/{category_id}", response_class=ApiResponse)
async def update_category(
    category_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_category(await read_json(request))
    assert body.type is not None
    result = await _service(conn).update(
        caller,
        category_id,
        name=body.name,
        category_type=body.type,
        icon=body.icon,
        color=body.color,
    )
    return ApiResponse(result)


@router.delete("/{category_id}", status_code=204)
async def delete_category(category_id: uuid.UUID, caller: CallerId, conn: Connection) -> Response:
    await _service(conn).delete(caller, category_id)
    return Response(status_code=204)
