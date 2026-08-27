"""/api/dashboard and /api/reports.

The query field keys here are lowercase (months, year, from, to), unlike the
transaction search's PascalCase ones. That difference is inherited from the
.NET action signatures and the parity tests assert it.
"""

from __future__ import annotations

from datetime import UTC, datetime

from fastapi import APIRouter, Request

from app.api.query import QueryReader
from app.api.rendering import ApiResponse
from app.core.deps import CallerId, Connection
from app.repositories.accounts import AccountRepository
from app.repositories.categories import CategoryRepository
from app.repositories.transactions import TransactionRepository
from app.services.analytics import (
    DEFAULT_CASHFLOW_MONTHS,
    DEFAULT_NET_WORTH_MONTHS,
    AnalyticsService,
)
from app.services.categories import CategoryService

router = APIRouter(tags=["analytics"])


def _service(conn: Connection) -> AnalyticsService:
    return AnalyticsService(
        TransactionRepository(conn),
        AccountRepository(conn),
        CategoryService(CategoryRepository(conn)),
    )


@router.get("/dashboard/summary", response_class=ApiResponse)
async def summary(caller: CallerId, conn: Connection) -> ApiResponse:
    return ApiResponse(await _service(conn).summary(caller, datetime.now(UTC)))


@router.get("/dashboard/networth", response_class=ApiResponse)
async def net_worth(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    reader = QueryReader(request)
    months = reader.number_or("months", "months", DEFAULT_NET_WORTH_MONTHS)
    reader.done()
    return ApiResponse(await _service(conn).net_worth(caller, datetime.now(UTC), months))


@router.get("/dashboard/cashflow", response_class=ApiResponse)
async def cashflow(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    reader = QueryReader(request)
    months = reader.number_or("months", "months", DEFAULT_CASHFLOW_MONTHS)
    reader.done()
    return ApiResponse(await _service(conn).cashflow(caller, datetime.now(UTC), months))


@router.get("/dashboard/spending", response_class=ApiResponse)
async def spending(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    month = QueryReader(request).text("month")
    return ApiResponse(await _service(conn).spending(caller, datetime.now(UTC), month))


@router.get("/reports/monthly", response_class=ApiResponse)
async def monthly_report(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    reader = QueryReader(request)
    year = reader.number_or("year", "year", datetime.now(UTC).year)
    reader.done()
    return ApiResponse(await _service(conn).monthly_report(caller, year))


@router.get("/reports/categories", response_class=ApiResponse)
async def category_report(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    reader = QueryReader(request)
    date_from = reader.moment("from", "from")
    date_to = reader.moment("to", "to")
    reader.done()
    return ApiResponse(await _service(conn).category_report(caller, date_from, date_to))
