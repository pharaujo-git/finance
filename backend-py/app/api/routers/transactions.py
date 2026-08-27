"""/api/transactions, including the CSV export and import.

Route order matters: /export and /import are declared before /{id} so the
literal segments are not swallowed by the parameter.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from typing import Annotated

from fastapi import APIRouter, Request, Response, UploadFile
from fastapi import File as FileParam

from app.api.query import QueryReader
from app.api.rendering import ApiResponse
from app.api.routers.auth import read_json
from app.api.schemas import parse_transaction
from app.core.deps import CallerId, Connection
from app.core.errors import validation
from app.domain.enums import TransactionType
from app.repositories.accounts import AccountRepository
from app.repositories.categories import CategoryRepository
from app.repositories.transactions import TransactionRepository
from app.services.categories import CategoryService
from app.services.csv_io import (
    MAX_UPLOAD_BYTES,
    MISSING_FILE_MESSAGE,
    OVERSIZE_MESSAGE,
    CsvService,
    export_file_name,
)
from app.services.transactions import (
    DEFAULT_PAGE,
    DEFAULT_PAGE_SIZE,
    TransactionQuery,
    TransactionService,
)

router = APIRouter(prefix="/transactions", tags=["transactions"])


def _service(conn: Connection) -> TransactionService:
    return TransactionService(
        TransactionRepository(conn),
        AccountRepository(conn),
        CategoryService(CategoryRepository(conn)),
    )


def _csv(conn: Connection) -> CsvService:
    return CsvService(
        TransactionRepository(conn), AccountRepository(conn), CategoryRepository(conn)
    )


@router.get("", response_class=ApiResponse)
async def search(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    reader = QueryReader(request)
    query = TransactionQuery(
        page=reader.number_or("page", "Page", DEFAULT_PAGE),
        page_size=reader.number_or("pageSize", "PageSize", DEFAULT_PAGE_SIZE),
        account_id=reader.identifier("accountId", "AccountId"),
        category_id=reader.identifier("categoryId", "CategoryId"),
        type=reader.enum("type", "Type", TransactionType),  # type: ignore[arg-type]
        date_from=reader.moment("from", "From"),
        date_to=reader.moment("to", "To"),
        search=reader.text("search") or "",
    )
    reader.done()
    return ApiResponse(await _service(conn).search(caller, query))


@router.post("", response_class=ApiResponse)
async def create_transaction(request: Request, caller: CallerId, conn: Connection) -> ApiResponse:
    body = parse_transaction(await read_json(request))
    return ApiResponse(await _service(conn).create(caller, body))


@router.get("/export")
async def export_transactions(request: Request, caller: CallerId, conn: Connection) -> Response:
    reader = QueryReader(request)
    # Lowercase field keys here, unlike the search endpoint above -- that
    # asymmetry comes from the .NET action signatures and is asserted by tests.
    date_from = reader.moment("from", "from")
    date_to = reader.moment("to", "to")
    reader.done()

    body = await _csv(conn).export(caller, date_from, date_to)
    name = export_file_name(datetime.now(UTC))
    return Response(
        content=body,
        media_type="text/csv",
        headers={"Content-Disposition": (f"attachment; filename={name}; filename*=UTF-8''{name}")},
    )


# The multipart part is named "file", matching the other two backends. The
# default goes on the parameter, not in the Annotated -- FastAPI rejects it here.
UploadedFile = Annotated[UploadFile | None, FileParam()]


@router.post("/import", response_class=ApiResponse)
async def import_transactions(
    caller: CallerId, conn: Connection, file: UploadedFile = None
) -> ApiResponse:
    if file is None:
        raise validation(MISSING_FILE_MESSAGE)

    content = await file.read()
    if not content:
        raise validation(MISSING_FILE_MESSAGE)
    if len(content) > MAX_UPLOAD_BYTES:
        raise validation(OVERSIZE_MESSAGE)

    return ApiResponse(await _csv(conn).import_csv(caller, content))


@router.get("/{transaction_id}", response_class=ApiResponse)
async def get_transaction(
    transaction_id: uuid.UUID, caller: CallerId, conn: Connection
) -> ApiResponse:
    return ApiResponse(await _service(conn).get(caller, transaction_id))


@router.put("/{transaction_id}", response_class=ApiResponse)
async def update_transaction(
    transaction_id: uuid.UUID, request: Request, caller: CallerId, conn: Connection
) -> ApiResponse:
    body = parse_transaction(await read_json(request))
    return ApiResponse(await _service(conn).update(caller, transaction_id, body))


@router.delete("/{transaction_id}", status_code=204)
async def delete_transaction(
    transaction_id: uuid.UUID, caller: CallerId, conn: Connection
) -> Response:
    await _service(conn).delete(caller, transaction_id)
    return Response(status_code=204)
