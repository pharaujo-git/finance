"""Query-string binding that mirrors MVC's model binder.

Conversion failures are collected rather than raised one at a time, so a
request with three bad values reports all three. An *absent* key is not the
same as an empty one: "?month=" is a failure, not "this month".
"""

from __future__ import annotations

import uuid
from datetime import datetime

from fastapi import Request

from app.core.errors import ValidationError
from app.domain import validation as rules
from app.domain.money import parse_wire_date


class QueryReader:
    def __init__(self, request: Request) -> None:
        self._params = request.query_params
        self._errs = ValidationError()

    def text(self, key: str) -> str | None:
        """None when the caller omitted the key entirely."""
        return self._params.get(key) if key in self._params else None

    def number(self, key: str, field: str) -> int | None:
        raw = self.text(key)
        if raw is None:
            return None
        try:
            return int(raw)
        except ValueError:
            rules.invalid_value(self._errs, raw, field)
            return None

    def number_or(self, key: str, field: str, fallback: int) -> int:
        parsed = self.number(key, field)
        return fallback if parsed is None else parsed

    def identifier(self, key: str, field: str) -> uuid.UUID | None:
        raw = self.text(key)
        if raw is None:
            return None
        try:
            return uuid.UUID(raw)
        except ValueError:
            rules.invalid_value(self._errs, raw, field)
            return None

    def moment(self, key: str, field: str) -> datetime | None:
        raw = self.text(key)
        if raw is None:
            return None
        parsed = parse_wire_date(raw)
        if parsed is None:
            rules.invalid_value(self._errs, raw, field)
        return parsed

    def enum(self, key: str, field: str, kind: type) -> object | None:
        raw = self.text(key)
        if raw is None:
            return None
        parsed = kind.parse(raw)  # type: ignore[attr-defined]
        if parsed is None:
            rules.invalid_value(self._errs, raw, field)
        return parsed

    def done(self) -> None:
        """Raises the accumulated conversion failures, if any."""
        rules.raise_if_any(self._errs)
