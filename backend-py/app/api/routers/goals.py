"""/api/goals."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Request, Response

from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_contribute, parse_goal
from app.core.deps import CallerId, Connection
from app.repositories.goals import GoalRepository
from app.services.goals import GoalService

router = APIRouter(prefix="/goals", tags=["goals"])


def _service(conn: Connection) -> GoalService:
    return GoalService(GoalRepository(conn))


@router.get("", response_class=ApiResponse)
async def list_goals(caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).list_all(caller))


@router.post("", response_class=ApiResponse)
async def create_goal(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_goal(await read_json(request))
    assert body.target_amount is not None
    result = await _service(conn).create(
        caller,
        name=body.name,
        target_amount=body.target_amount,
        current_amount=body.current_amount,
        target_date=body.target_date,
        color=body.color,
    )
    return ApiResponse(result)


@router.put("/{goal_id}", response_class=ApiResponse)
async def update_goal(
    goal_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_goal(await read_json(request))
    assert body.target_amount is not None
    result = await _service(conn).update(
        caller,
        goal_id,
        name=body.name,
        target_amount=body.target_amount,
        current_amount=body.current_amount,
        target_date=body.target_date,
        color=body.color,
    )
    return ApiResponse(result)


@router.delete("/{goal_id}", status_code=204)
async def delete_goal(goal_id: uuid.UUID, caller: CallerId, conn: Connection) -> Response:
    await _service(conn).delete(caller, goal_id)
    return Response(status_code=204)


@router.post("/{goal_id}/contribute", response_class=ApiResponse)
async def contribute(
    goal_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_contribute(await read_json(request))
    assert body.amount is not None
    return ApiResponse(await _service(conn).contribute(caller, goal_id, body.amount))
