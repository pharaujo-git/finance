"""Domain errors and the two problem shapes the API renders.

Both shapes were captured from the running .NET API and reproduced by the Go
one; the frontend's readError() parses them, so the field names, the status
titles and even the content types are fixed.
"""

from __future__ import annotations

import enum
from typing import Any, Final

from fastapi import Request
from fastapi.responses import JSONResponse

# RFC 9457, and what the .NET API writes for handled exceptions.
PROBLEM_CONTENT_TYPE: Final = "application/problem+json"

# MVC content-negotiates plain JSON (not problem+json) for validation 400s.
# Reproduced so a client switching on the media type sees no difference.
VALIDATION_CONTENT_TYPE: Final = "application/json; charset=utf-8"

VALIDATION_PROBLEM_TYPE: Final = "https://tools.ietf.org/html/rfc9110#section-15.5.1"
VALIDATION_PROBLEM_TITLE: Final = "One or more validation errors occurred."


class ErrorKind(enum.Enum):
    """The failure categories the application layer raises."""

    VALIDATION = "validation"
    UNAUTHORIZED = "unauthorized"
    NOT_FOUND = "not_found"
    CONFLICT = "conflict"


# Mirrors ApiExceptionHandler.Responses in the .NET API.
_STATUS_TITLES: Final[dict[ErrorKind, tuple[int, str]]] = {
    ErrorKind.VALIDATION: (400, "Bad Request"),
    ErrorKind.UNAUTHORIZED: (401, "Unauthorized"),
    ErrorKind.NOT_FOUND: (404, "Not Found"),
    ErrorKind.CONFLICT: (409, "Conflict"),
}


class AppError(Exception):
    """A domain failure carrying the kind that decides its HTTP status."""

    def __init__(self, kind: ErrorKind, message: str) -> None:
        super().__init__(message)
        self.kind = kind
        self.message = message


def validation(message: str) -> AppError:
    return AppError(ErrorKind.VALIDATION, message)


def unauthorized(message: str) -> AppError:
    return AppError(ErrorKind.UNAUTHORIZED, message)


def not_found(entity: str) -> AppError:
    """Takes the entity name, not the sentence: "Account" -> "Account was not found." """
    return AppError(ErrorKind.NOT_FOUND, f"{entity} was not found.")


def conflict(message: str) -> AppError:
    return AppError(ErrorKind.CONFLICT, message)


class ValidationError(Exception):
    """Field-keyed errors, rendered as MVC's validation dictionary."""

    def __init__(self, errors: dict[str, list[str]] | None = None) -> None:
        super().__init__(VALIDATION_PROBLEM_TITLE)
        self.errors: dict[str, list[str]] = errors or {}

    def add(self, field: str, message: str) -> None:
        self.errors.setdefault(field, []).append(message)

    @property
    def empty(self) -> bool:
        return not self.errors


def problem_response(
    status: int, title: str, detail: str, instance: str | None = None
) -> JSONResponse:
    """Renders an RFC 9457 problem document.

    `type` is absent because the .NET handler never sets it; the human-readable
    text lands in `detail` with `title` holding the status phrase.
    """
    body: dict[str, Any] = {"title": title, "status": status, "detail": detail}
    if instance:
        body["instance"] = instance
    return JSONResponse(body, status_code=status, media_type=PROBLEM_CONTENT_TYPE)


def validation_response(errors: dict[str, list[str]]) -> JSONResponse:
    """Renders the field-error dictionary MVC writes for a 400.

    Keys are sorted because Go marshals a map with sorted keys, and the two
    backends' bodies are compared byte for byte.
    """
    return JSONResponse(
        {
            "type": VALIDATION_PROBLEM_TYPE,
            "title": VALIDATION_PROBLEM_TITLE,
            "status": 400,
            "errors": {key: errors[key] for key in sorted(errors)},
        },
        status_code=400,
        media_type=VALIDATION_CONTENT_TYPE,
    )


async def app_error_handler(request: Request, exc: Exception) -> JSONResponse:
    """Maps a domain error onto its status and title."""
    assert isinstance(exc, AppError)
    status, title = _STATUS_TITLES.get(exc.kind, (400, "Error"))
    response = problem_response(status, title, exc.message, request.url.path)
    if exc.kind is ErrorKind.UNAUTHORIZED:
        response.headers["WWW-Authenticate"] = "Bearer"
    return response


async def validation_error_handler(request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, ValidationError)
    return validation_response(exc.errors)
