"""Money, tags and the wire formats for dates.

Amounts are numeric(18,2) in Postgres. They are never floats: every value that
crosses the wire is a Decimal, and it renders with the scale it carries, so
1250.00 stays 1250.00 and 12.5 stays 12.5 -- matching the other two backends.
"""

from __future__ import annotations

from datetime import UTC, date, datetime
from decimal import ROUND_HALF_UP, Decimal, InvalidOperation
from typing import Final

# The bounds every [Range(typeof(decimal), ...)] in the DTOs uses. They are
# strings because that is how the attribute spells them, and because the
# validation message quotes them verbatim.
MONEY_MIN_POSITIVE: Final = "0.01"
MONEY_MIN_ZERO: Final = "0.00"
MONEY_MAX: Final = "999999999999.99"

MONEY_SCALE: Final = 2
_CENTS: Final = Decimal("0.01")
ZERO: Final = Decimal(0)

# The separator the TagsRaw column uses. A unit separator cannot appear in a tag.
TAG_SEPARATOR: Final = "\x1f"


def round_money(value: Decimal) -> Decimal:
    """Rounds to two places, half away from zero -- and never lengthens the scale.

    A value already at or above two places is returned untouched, so 10 stays
    10 rather than becoming 10.00. That asymmetry is deliberate: it is what
    keeps the JSON identical across the three backends.
    """
    if -value.as_tuple().exponent <= MONEY_SCALE:  # type: ignore[operator]
        return value
    rounded = value.quantize(_CENTS, rounding=ROUND_HALF_UP)
    # A small negative rounds to -0.00 in Decimal; no backend should ever put a
    # signed zero on the wire.
    return rounded.copy_abs() if rounded == 0 else rounded


def trim_money(value: Decimal) -> Decimal:
    """Drops trailing fractional zeros. Used only on the savings rate."""
    trimmed = value.normalize()
    # normalize() turns 100 into 1E+2; put integers back into plain form.
    if -trimmed.as_tuple().exponent < 0:  # type: ignore[operator]
        return trimmed.quantize(Decimal(1))
    return trimmed


def render_amount(value: Decimal) -> str:
    """Renders an amount as a bare JSON number, keeping its scale."""
    return format(value, "f") if value.as_tuple().exponent < 0 else str(value)  # type: ignore[operator]


def parse_money(raw: object) -> Decimal | None:
    """Reads a decimal from a JSON number or a quoted number."""
    if isinstance(raw, bool) or raw is None:
        return None
    if isinstance(raw, Decimal):
        return raw
    if isinstance(raw, int):
        return Decimal(raw)
    if isinstance(raw, float):
        return Decimal(str(raw))
    if isinstance(raw, str):
        try:
            return Decimal(raw.strip())
        except (InvalidOperation, ValueError):
            return None
    return None


def parse_wire_date(raw: str) -> datetime | None:
    """Accepts the layouts the model binder does, assuming UTC when no zone is given.

    Most specific first: RFC3339, then second- and minute-precision local
    timestamps, then the bare date the frontend's <input type="date"> posts.
    """
    text = (raw or "").strip()
    if not text:
        return None

    candidate = text[:-1] + "+00:00" if text.endswith(("Z", "z")) else text
    try:
        parsed = datetime.fromisoformat(candidate)
    except ValueError:
        try:
            return datetime.combine(date.fromisoformat(text), datetime.min.time(), tzinfo=UTC)
        except ValueError:
            return None

    return parsed.replace(tzinfo=UTC) if parsed.tzinfo is None else parsed.astimezone(UTC)


def as_utc(value: datetime) -> datetime:
    """Normalises a timestamp to UTC; naive values are read as UTC wall clock."""
    return value.replace(tzinfo=UTC) if value.tzinfo is None else value.astimezone(UTC)


def render_datetime(value: datetime) -> str:
    """RFC3339 in UTC, with trailing zeros trimmed off the fraction.

    Go marshals time.Time as RFC3339Nano, which drops trailing zeros and the
    decimal point entirely when the fraction is zero. Python's isoformat keeps
    all six digits, so 22:27:26.516550Z would not match the Go API's
    22:27:26.51655Z.
    """
    moment = as_utc(value)
    base = moment.strftime("%Y-%m-%dT%H:%M:%S")
    fraction = f"{moment.microsecond:06d}".rstrip("0")
    return f"{base}.{fraction}Z" if fraction else f"{base}Z"


def join_tags(tags: list[str] | None) -> str:
    """Trims, drops blanks, and packs into the storage column."""
    return TAG_SEPARATOR.join(part for part in (t.strip() for t in tags or []) if part)


def split_tags(raw: str | None) -> list[str]:
    """Unpacks the storage column. Always a list, never None."""
    if not raw:
        return []
    return [part for part in raw.split(TAG_SEPARATOR) if part]


def trimmed_or_none(value: str | None) -> str | None:
    """Whitespace-only collapses to None, as the other backends do for Notes."""
    text = (value or "").strip()
    return text or None
