"""JSON rendering with exact control over how each scalar reaches the wire.

The stock encoders will not do here. Money must arrive as a *bare* JSON number
that keeps the scale it carries (1250.00, not 1250.0 and not "1250.00"), which
rules out float conversion and rules out quoting. Enums must arrive as their
camelCase name, or as a bare ordinal when the value names no member.

So the document is written directly rather than post-processed: no placeholder
can be spoofed by user-supplied text that way.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from fastapi import Response

from app.domain.enums import _WireEnum
from app.domain.money import render_amount, render_datetime


def _write(value: Any, out: list[str]) -> None:
    """Appends the JSON text for one value."""
    if value is None:
        out.append("null")
    elif isinstance(value, bool):
        # Before int: bool is a subclass of int.
        out.append("true" if value else "false")
    elif isinstance(value, _WireEnum):
        if value.is_defined:
            out.append(json.dumps(value.wire_name, ensure_ascii=False))
        else:
            # An ordinal naming no member is written back as a bare number,
            # exactly as JsonStringEnumConverter does.
            out.append(str(int(value)))
    elif isinstance(value, Decimal):
        out.append(render_amount(value))
    elif isinstance(value, int):
        out.append(str(value))
    elif isinstance(value, float):
        out.append(repr(value))
    elif isinstance(value, str):
        out.append(json.dumps(value, ensure_ascii=False))
    elif isinstance(value, uuid.UUID):
        out.append(json.dumps(str(value)))
    elif isinstance(value, datetime):
        out.append(json.dumps(render_datetime(value)))
    elif isinstance(value, dict):
        out.append("{")
        for index, (key, item) in enumerate(value.items()):
            if index:
                out.append(",")
            out.append(json.dumps(str(key), ensure_ascii=False))
            out.append(":")
            _write(item, out)
        out.append("}")
    elif isinstance(value, (list, tuple)):
        out.append("[")
        for index, item in enumerate(value):
            if index:
                out.append(",")
            _write(item, out)
        out.append("]")
    else:
        out.append(json.dumps(value, ensure_ascii=False))


def dumps(payload: Any) -> str:
    out: list[str] = []
    _write(payload, out)
    return "".join(out)


class ApiResponse(Response):
    """A JSON response that renders money and enums the way the API promises."""

    # Spelled out rather than left to Starlette, which only appends the charset
    # for text/* -- and the other two backends send it on JSON.
    media_type = "application/json; charset=utf-8"

    def render(self, content: Any) -> bytes:
        return dumps(content).encode()
