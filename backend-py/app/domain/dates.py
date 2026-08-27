"""Calendar arithmetic that matches .NET's DateTime, and the YYYY-MM month key."""

from __future__ import annotations

import calendar
from datetime import UTC, datetime
from typing import Final

from app.core.errors import validation
from app.domain.money import as_utc

MONTH_FORMAT: Final = "%Y-%m"
MONTH_FORMAT_MESSAGE: Final = "Month must be in YYYY-MM format."

MIN_REPORT_YEAR: Final = 1900
MAX_REPORT_YEAR: Final = 9999
MIN_WINDOW_MONTH: Final = 1
MAX_WINDOW_MONTH: Final = 120

YEAR_RANGE_MESSAGE: Final = "Year must be between 1900 and 9999."


def add_months(moment: datetime, months: int) -> datetime:
    """.NET's DateTime.AddMonths: clamps the day, it does not roll over.

    31 Jan + 1 month is 28 Feb, and adding another month gives 28 Mar -- not
    31 Mar. Clock time and zone are preserved.
    """
    total = (moment.month - 1) + months
    year = moment.year + total // 12
    month = total % 12 + 1
    day = min(moment.day, calendar.monthrange(year, month)[1])
    return moment.replace(year=year, month=month, day=day)


def add_years(moment: datetime, years: int) -> datetime:
    """Also clamping, so 29 Feb becomes 28 Feb in a common year."""
    return add_months(moment, years * 12)


def start_of_month(moment: datetime) -> datetime:
    """Midnight UTC on the first of the given moment's UTC month."""
    utc = as_utc(moment)
    return utc.replace(day=1, hour=0, minute=0, second=0, microsecond=0)


def first_day_utc(year: int, month: int) -> datetime:
    return datetime(year, month, 1, tzinfo=UTC)


def month_from(moment: datetime) -> str:
    """The YYYY-MM key for a moment's UTC month."""
    return as_utc(moment).strftime(MONTH_FORMAT)


def try_parse_month(value: str | None) -> datetime | None:
    """Midnight UTC on day 1 of that month, or None when it is not a real month."""
    text = (value or "").strip()
    if not text:
        return None
    try:
        parsed = datetime.strptime(text, MONTH_FORMAT)
    except ValueError:
        return None
    return parsed.replace(tzinfo=UTC)


def parse_month(value: str | None) -> datetime:
    """try_parse_month, or the 400 the other backends render."""
    parsed = try_parse_month(value)
    if parsed is None:
        raise validation(MONTH_FORMAT_MESSAGE)
    return parsed


def trailing_months(reference: datetime, count: int) -> list[datetime]:
    """`count` month starts ending with the reference's own month, oldest first."""
    anchor = start_of_month(reference)
    return [add_months(anchor, offset - count + 1) for offset in range(count)]


def clamp_months(months: int) -> int:
    return max(MIN_WINDOW_MONTH, min(MAX_WINDOW_MONTH, months))
