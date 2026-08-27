"""The DataAnnotations rules the .NET API enforces, reproduced message for message.

The frontend renders these strings verbatim, and the other two backends emit
them byte-identically, so the wording below is fixed. Note the deliberate
asymmetry copied from .NET's resources: `required` and `emailAddress` put the
field name first ("The Email field ..."), while the length and range rules put
"The field" first ("The field Email ...").
"""

from __future__ import annotations

import re
from decimal import Decimal
from typing import Final

from app.core.errors import ValidationError
from app.domain.money import MONEY_MAX

# ModelState keys are the PascalCase property names of the .NET request DTOs,
# not the camelCase JSON names, because that is what the .NET API emits.
FIELD_EMAIL: Final = "Email"
FIELD_PASSWORD: Final = "Password"
FIELD_NAME: Final = "Name"
FIELD_CURRENCY: Final = "Currency"
FIELD_ICON: Final = "Icon"
FIELD_COLOR: Final = "Color"
FIELD_DESCRIPTION: Final = "Description"
FIELD_NOTES: Final = "Notes"
FIELD_AMOUNT: Final = "Amount"
FIELD_MONTH: Final = "Month"
FIELD_LIMIT: Final = "Limit"
FIELD_TARGET: Final = "TargetAmount"
FIELD_CURRENT: Final = "CurrentAmount"
FIELD_PAGE: Final = "Page"
FIELD_PAGE_SIZE: Final = "PageSize"
FIELD_SEARCH: Final = "Search"

# The key MVC's JSON reader uses for a body-level failure.
JSON_BODY_FIELD: Final = "$"

EMAIL_MAX_LENGTH: Final = 256
PASSWORD_MIN_LENGTH: Final = 8
PASSWORD_MAX_LENGTH: Final = 128
NAME_MAX_LENGTH: Final = 200
CURRENCY_MAX_LENGTH: Final = 8
ICON_MAX_LENGTH: Final = 64
COLOR_MAX_LENGTH: Final = 32
DESCRIPTION_MAX_LENGTH: Final = 500
NOTES_MAX_LENGTH: Final = 2000
SEARCH_MAX_LENGTH: Final = 200

REQUIRED_MESSAGE: Final = "The {field} field is required."
EMAIL_ADDRESS_MESSAGE: Final = "The {field} field is not a valid e-mail address."
MIN_LENGTH_MESSAGE: Final = (
    "The field {field} must be a string or array type with a minimum length of '{limit}'."
)
MAX_LENGTH_MESSAGE: Final = (
    "The field {field} must be a string or array type with a maximum length of '{limit}'."
)
RANGE_MESSAGE: Final = "The field {field} must be between {minimum} and {maximum}."
MISSING_MEMBERS_MESSAGE: Final = (
    "The JSON payload was missing required properties, including the following: {members}"
)
MONTH_FORMAT_MESSAGE: Final = "Month must be in YYYY-MM format."
INVALID_VALUE_MESSAGE: Final = "The value '{value}' is not valid for {field}."

# Looser than a real month parse on purpose: "2026-13" passes here and fails later.
MONTH_PATTERN: Final = re.compile(r"^\d{4}-\d{2}$")


def required(errs: ValidationError, field: str, value: str | None) -> None:
    """Whitespace-only counts as missing, as [Required] does."""
    if not (value or "").strip():
        errs.add(field, REQUIRED_MESSAGE.format(field=field))


def email_address(errs: ValidationError, field: str, value: str | None) -> None:
    """Exactly one '@', neither first nor last character, on the raw value."""
    text = value or ""
    if text.count("@") != 1 or text.startswith("@") or text.endswith("@"):
        errs.add(field, EMAIL_ADDRESS_MESSAGE.format(field=field))


def min_length(errs: ValidationError, field: str, value: str | None, limit: int) -> None:
    if len(value or "") < limit:
        errs.add(field, MIN_LENGTH_MESSAGE.format(field=field, limit=limit))


def max_length(errs: ValidationError, field: str, value: str | None, limit: int) -> None:
    if len(value or "") > limit:
        errs.add(field, MAX_LENGTH_MESSAGE.format(field=field, limit=limit))


def decimal_range(errs: ValidationError, field: str, value: Decimal | None, minimum: str) -> None:
    """A missing value is not this rule's business; the required-member check owns it."""
    if value is None:
        return
    if value < Decimal(minimum) or value > Decimal(MONEY_MAX):
        errs.add(field, RANGE_MESSAGE.format(field=field, minimum=minimum, maximum=MONEY_MAX))


def int_range(
    errs: ValidationError, field: str, value: int | None, minimum: int, maximum: int
) -> None:
    if value is None:
        return
    if value < minimum or value > maximum:
        errs.add(field, RANGE_MESSAGE.format(field=field, minimum=minimum, maximum=maximum))


def required_members(errs: ValidationError, missing: list[str]) -> bool:
    """Reports absent `required` members under "$".

    Returns True when something was missing, and every caller returns
    immediately on True — so no other rule runs. That short-circuit is the
    single most important ordering behaviour to keep: the .NET pipeline fails
    deserialisation before model validation ever starts.
    """
    if not missing:
        return False
    errs.add(JSON_BODY_FIELD, MISSING_MEMBERS_MESSAGE.format(members=", ".join(missing)))
    return True


def invalid_value(errs: ValidationError, value: str, field: str) -> None:
    errs.add(field, INVALID_VALUE_MESSAGE.format(value=value, field=field))


def raise_if_any(errs: ValidationError) -> None:
    if not errs.empty:
        raise errs
