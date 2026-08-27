"""The FastAPI application.

The service document at GET / names this backend so an operator can tell which
of the three answered a request.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import APIRouter, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import PlainTextResponse, Response

from app.api.rendering import ApiResponse
from app.api.routers import (
    accounts,
    analytics,
    auth,
    budgets,
    categories,
    goals,
    recurring,
    transactions,
)
from app.core.config import Settings, load_settings
from app.core.errors import (
    AppError,
    ValidationError,
    app_error_handler,
    validation_error_handler,
    validation_response,
)
from app.core.security import TokenService
from app.db import create_pool
from app.domain.validation import JSON_BODY_FIELD


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings: Settings = app.state.settings
    app.state.tokens = TokenService(settings.jwt_secret)

    # A pool supplied by the caller (the test harness binds one to a throwaway
    # schema) is borrowed, not owned: whoever opened it closes it.
    borrowed = getattr(app.state, "pool", None)
    if borrowed is None:
        app.state.pool = await create_pool(settings.database_url)
    try:
        yield
    finally:
        if borrowed is None:
            await app.state.pool.close()


def create_app(settings: Settings | None = None, pool: object | None = None) -> FastAPI:
    resolved = settings or load_settings()

    app = FastAPI(
        title="FinanceTracker API (Python)",
        docs_url="/swagger",
        openapi_url="/swagger/openapi.json",
        lifespan=lifespan,
        default_response_class=ApiResponse,
    )
    app.state.settings = resolved
    if pool is not None:
        app.state.pool = pool

    # Exact-match origins, any header, any method, no credentials -- the same
    # policy the other two backends apply.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=resolved.allowed_origins,
        allow_credentials=False,
        allow_methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"],
        allow_headers=["*"],
        max_age=86400,
    )

    app.add_exception_handler(AppError, app_error_handler)
    app.add_exception_handler(ValidationError, validation_error_handler)

    @app.get("/health", response_class=PlainTextResponse)
    async def health() -> str:
        return "ok"

    @app.get("/", response_class=ApiResponse)
    async def service_document() -> ApiResponse:
        return ApiResponse(
            {
                "service": "FinanceTracker API (Python)",
                "status": "ok",
                "docs": "/swagger",
            }
        )

    api = APIRouter(prefix="/api")
    api.include_router(auth.router)
    api.include_router(accounts.router)
    api.include_router(categories.router)
    api.include_router(transactions.router)
    api.include_router(recurring.router)
    api.include_router(budgets.router)
    api.include_router(goals.router)
    api.include_router(analytics.router)
    app.include_router(api)

    app.add_exception_handler(RequestValidationError, _request_validation_handler)
    return app


async def _request_validation_handler(request: Request, exc: Exception) -> Response:
    """Keeps FastAPI's own 422 shape off the wire.

    A non-uuid {id} segment answers a bare 404 with no body: the .NET routes
    constrain it with {id:guid}, so such a request matches no route at all.
    Anything else becomes the validation document the other backends write.
    """
    assert isinstance(exc, RequestValidationError)

    errors: dict[str, list[str]] = {}
    for detail in exc.errors():
        location = detail.get("loc", ())
        if location and location[0] == "path":
            return Response(status_code=404)
        field = str(location[-1]) if location else JSON_BODY_FIELD
        errors.setdefault(field, []).append(str(detail.get("msg", "")))

    return validation_response(errors or {JSON_BODY_FIELD: ["The request body is invalid."]})


def build() -> FastAPI:
    """Entry point for `uvicorn app.main:build --factory`."""
    return create_app()
