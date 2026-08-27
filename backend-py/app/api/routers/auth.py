"""/api/auth. Register and login are the only anonymous routes in the API."""

from __future__ import annotations

from fastapi import APIRouter, Request

from app.api.rendering import ApiResponse
from app.api.schemas import parse_login, parse_register, parse_update_profile
from app.core.deps import CallerId, Connection, Tokens
from app.core.errors import ValidationError
from app.domain.validation import JSON_BODY_FIELD
from app.repositories.users import UserRepository
from app.services.auth import AuthService

router = APIRouter(prefix="/auth", tags=["auth"])


async def read_json(request: Request) -> object:
    """Decodes the body, reporting a parse failure the way MVC's reader does."""
    try:
        return await request.json()
    except Exception as exc:  # noqa: BLE001 - any decode failure is one 400
        errs = ValidationError()
        errs.add(JSON_BODY_FIELD, str(exc))
        raise errs from exc


def _service(conn: Connection, tokens: Tokens) -> AuthService:
    return AuthService(UserRepository(conn), tokens)


@router.post("/register", response_class=ApiResponse)
async def register(request: Request, conn: Connection, tokens: Tokens) -> ApiResponse:
    body = parse_register(await read_json(request))
    result = await _service(conn, tokens).register(body.email, body.password, body.name)
    return ApiResponse(result)


@router.post("/login", response_class=ApiResponse)
async def login(request: Request, conn: Connection, tokens: Tokens) -> ApiResponse:
    body = parse_login(await read_json(request))
    result = await _service(conn, tokens).login(body.email, body.password)
    return ApiResponse(result)


@router.get("/me", response_class=ApiResponse)
async def profile(caller: CallerId, conn: Connection, tokens: Tokens) -> ApiResponse:
    return ApiResponse(await _service(conn, tokens).profile(caller))


@router.put("/me", response_class=ApiResponse)
async def update_profile(
    request: Request, caller: CallerId, conn: Connection, tokens: Tokens
) -> ApiResponse:
    body = parse_update_profile(await read_json(request))
    result = await _service(conn, tokens).update_profile(caller, body.name, body.currency)
    return ApiResponse(result)
