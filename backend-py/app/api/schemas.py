"""Request bodies and the rules that guard them.

Pydantic's own validation is deliberately not used for request bodies. The .NET
pipeline fails *deserialisation* before model validation runs, so a payload
missing a `required` member reports one error under "$" and nothing else -- a
short-circuit Pydantic cannot express. Everything here is hand-rolled to keep
the three backends byte-identical on error responses.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import datetime
from decimal import Decimal
from typing import Any

from app.core.errors import ValidationError
from app.domain import validation as rules
from app.domain.enums import (
    AccountType,
    CategoryType,
    Frequency,
    TransactionType,
    _WireEnum,
)
from app.domain.money import (
    MONEY_MIN_POSITIVE,
    MONEY_MIN_ZERO,
    parse_money,
    parse_wire_date,
)
from app.domain.validation import JSON_BODY_FIELD


class Body:
    """A decoded JSON object, tracking which members the caller actually sent."""

    def __init__(self, raw: Any) -> None:
        if not isinstance(raw, dict):
            errs = ValidationError()
            errs.add(JSON_BODY_FIELD, "the JSON value could not be converted to an object")
            raise errs
        self._raw: dict[str, Any] = raw

    def text(self, key: str) -> str:
        value = self._raw.get(key)
        return value if isinstance(value, str) else ""

    def optional_text(self, key: str) -> str | None:
        value = self._raw.get(key)
        return value if isinstance(value, str) else None

    def present(self, key: str) -> bool:
        """A member counts as supplied only when it is there and not null."""
        return self._raw.get(key) is not None

    def money(self, key: str) -> Decimal | None:
        return parse_money(self._raw.get(key))

    def uuid(self, key: str) -> uuid.UUID | None:
        value = self._raw.get(key)
        if not isinstance(value, str):
            return None
        try:
            return uuid.UUID(value)
        except ValueError:
            return None

    def moment(self, key: str) -> datetime | None:
        value = self._raw.get(key)
        return parse_wire_date(value) if isinstance(value, str) else None

    def flag(self, key: str) -> bool | None:
        value = self._raw.get(key)
        return value if isinstance(value, bool) else None

    def tags(self, key: str) -> list[str]:
        value = self._raw.get(key)
        if not isinstance(value, list):
            return []
        return [item for item in value if isinstance(item, str)]

    def enum(self, key: str, kind: type[_WireEnum]) -> Any:
        """Reads an enum member.

        A string that names no member is a *deserialisation* failure, reported
        under "$" like the other JSON-reader errors -- a number is not, because
        any ordinal is accepted.
        """
        raw = self._raw.get(key)
        parsed = kind.parse(raw)
        if parsed is None and raw is not None:
            errs = ValidationError()
            if isinstance(raw, str):
                errs.add(
                    JSON_BODY_FIELD,
                    f'the JSON value "{raw}" could not be converted to {kind.__name__}',
                )
            else:
                errs.add(
                    JSON_BODY_FIELD,
                    f"the JSON value could not be converted to {kind.__name__}",
                )
            raise errs
        return parsed

    def missing(self, *keys: str) -> list[str]:
        """The `required` members the caller left out, in declaration order."""
        return [key for key in keys if not self.present(key)]


def _guard(body: Body, *required: str) -> ValidationError:
    """Runs the required-member check, raising on the first failure."""
    errs = ValidationError()
    if rules.required_members(errs, body.missing(*required)):
        raise errs
    return errs


# --- auth -------------------------------------------------------------------


@dataclass(slots=True)
class RegisterRequest:
    email: str
    password: str
    name: str


def parse_register(raw: Any) -> RegisterRequest:
    body = Body(raw)
    request = RegisterRequest(body.text("email"), body.text("password"), body.text("name"))

    errs = ValidationError()
    rules.required(errs, rules.FIELD_EMAIL, request.email)
    rules.email_address(errs, rules.FIELD_EMAIL, request.email)
    rules.max_length(errs, rules.FIELD_EMAIL, request.email, rules.EMAIL_MAX_LENGTH)
    rules.required(errs, rules.FIELD_PASSWORD, request.password)
    rules.min_length(errs, rules.FIELD_PASSWORD, request.password, rules.PASSWORD_MIN_LENGTH)
    rules.max_length(errs, rules.FIELD_PASSWORD, request.password, rules.PASSWORD_MAX_LENGTH)
    rules.required(errs, rules.FIELD_NAME, request.name)
    rules.max_length(errs, rules.FIELD_NAME, request.name, rules.NAME_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


@dataclass(slots=True)
class LoginRequest:
    email: str
    password: str


def parse_login(raw: Any) -> LoginRequest:
    body = Body(raw)
    request = LoginRequest(body.text("email"), body.text("password"))

    # No email-shape rule here: sign-in must fail as bad credentials, not as a
    # validation error, so a wrong address cannot be told from a wrong password.
    errs = ValidationError()
    rules.required(errs, rules.FIELD_EMAIL, request.email)
    rules.max_length(errs, rules.FIELD_EMAIL, request.email, rules.EMAIL_MAX_LENGTH)
    rules.required(errs, rules.FIELD_PASSWORD, request.password)
    rules.max_length(errs, rules.FIELD_PASSWORD, request.password, rules.PASSWORD_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


@dataclass(slots=True)
class UpdateProfileRequest:
    name: str
    currency: str


def parse_update_profile(raw: Any) -> UpdateProfileRequest:
    body = Body(raw)
    request = UpdateProfileRequest(body.text("name"), body.text("currency"))

    errs = ValidationError()
    rules.required(errs, rules.FIELD_NAME, request.name)
    rules.max_length(errs, rules.FIELD_NAME, request.name, rules.NAME_MAX_LENGTH)
    rules.required(errs, rules.FIELD_CURRENCY, request.currency)
    rules.max_length(errs, rules.FIELD_CURRENCY, request.currency, rules.CURRENCY_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


# --- accounts ---------------------------------------------------------------


@dataclass(slots=True)
class AccountRequest:
    name: str
    type: AccountType | None
    initial_balance: Decimal | None
    currency: str
    is_archived: bool | None


def parse_account(raw: Any) -> AccountRequest:
    body = Body(raw)
    _guard(body, "type")

    # An omitted currency and an explicit null both mean USD. The Go API cannot
    # tell them apart, so neither does this one.
    currency = body.optional_text("currency")
    request = AccountRequest(
        name=body.text("name"),
        type=body.enum("type", AccountType),
        initial_balance=body.money("initialBalance"),
        currency="USD" if currency is None else currency,
        is_archived=body.flag("isArchived"),
    )

    errs = ValidationError()
    rules.required(errs, rules.FIELD_NAME, request.name)
    rules.max_length(errs, rules.FIELD_NAME, request.name, rules.NAME_MAX_LENGTH)
    rules.required(errs, rules.FIELD_CURRENCY, request.currency)
    rules.max_length(errs, rules.FIELD_CURRENCY, request.currency, rules.CURRENCY_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


# --- categories -------------------------------------------------------------


@dataclass(slots=True)
class CategoryRequest:
    name: str
    type: CategoryType | None
    icon: str
    color: str


def parse_category(raw: Any) -> CategoryRequest:
    body = Body(raw)
    _guard(body, "type")

    request = CategoryRequest(
        name=body.text("name"),
        type=body.enum("type", CategoryType),
        icon=body.text("icon"),
        color=body.text("color"),
    )

    errs = ValidationError()
    rules.required(errs, rules.FIELD_NAME, request.name)
    rules.max_length(errs, rules.FIELD_NAME, request.name, rules.NAME_MAX_LENGTH)
    rules.max_length(errs, rules.FIELD_ICON, request.icon, rules.ICON_MAX_LENGTH)
    rules.max_length(errs, rules.FIELD_COLOR, request.color, rules.COLOR_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


# --- transactions -----------------------------------------------------------


@dataclass(slots=True)
class TransactionRequest:
    account_id: uuid.UUID | None
    category_id: uuid.UUID | None
    type: TransactionType | None
    amount: Decimal | None
    date: datetime | None
    description: str
    notes: str | None
    tags: list[str] = field(default_factory=list)
    transfer_account_id: uuid.UUID | None = None


def parse_transaction(raw: Any) -> TransactionRequest:
    body = Body(raw)
    _guard(body, "accountId", "type", "amount", "date")

    request = TransactionRequest(
        account_id=body.uuid("accountId"),
        category_id=body.uuid("categoryId"),
        type=body.enum("type", TransactionType),
        amount=body.money("amount"),
        date=body.moment("date"),
        description=body.text("description"),
        notes=body.optional_text("notes"),
        tags=body.tags("tags"),
        transfer_account_id=body.uuid("transferAccountId"),
    )

    errs = ValidationError()
    rules.decimal_range(errs, rules.FIELD_AMOUNT, request.amount, MONEY_MIN_POSITIVE)
    rules.required(errs, rules.FIELD_DESCRIPTION, request.description)
    rules.max_length(
        errs, rules.FIELD_DESCRIPTION, request.description, rules.DESCRIPTION_MAX_LENGTH
    )
    if request.notes is not None:
        rules.max_length(errs, rules.FIELD_NOTES, request.notes, rules.NOTES_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


# --- budgets ----------------------------------------------------------------


@dataclass(slots=True)
class CreateBudgetRequest:
    category_id: uuid.UUID | None
    month: str
    limit: Decimal | None


def parse_create_budget(raw: Any) -> CreateBudgetRequest:
    body = Body(raw)
    _guard(body, "categoryId", "limit")

    request = CreateBudgetRequest(
        category_id=body.uuid("categoryId"),
        month=body.text("month"),
        limit=body.money("limit"),
    )

    errs = ValidationError()
    rules.required(errs, rules.FIELD_MONTH, request.month)
    if request.month and not rules.MONTH_PATTERN.match(request.month):
        errs.add(rules.FIELD_MONTH, rules.MONTH_FORMAT_MESSAGE)
    rules.decimal_range(errs, rules.FIELD_LIMIT, request.limit, MONEY_MIN_ZERO)
    rules.raise_if_any(errs)
    return request


@dataclass(slots=True)
class UpdateBudgetRequest:
    limit: Decimal | None


def parse_update_budget(raw: Any) -> UpdateBudgetRequest:
    body = Body(raw)
    _guard(body, "limit")

    request = UpdateBudgetRequest(limit=body.money("limit"))
    errs = ValidationError()
    rules.decimal_range(errs, rules.FIELD_LIMIT, request.limit, MONEY_MIN_ZERO)
    rules.raise_if_any(errs)
    return request


# --- goals ------------------------------------------------------------------


@dataclass(slots=True)
class GoalRequest:
    name: str
    target_amount: Decimal | None
    current_amount: Decimal | None
    target_date: datetime | None
    color: str


def parse_goal(raw: Any) -> GoalRequest:
    body = Body(raw)
    _guard(body, "targetAmount")

    request = GoalRequest(
        name=body.text("name"),
        target_amount=body.money("targetAmount"),
        current_amount=body.money("currentAmount"),
        target_date=body.moment("targetDate"),
        color=body.text("color"),
    )

    errs = ValidationError()
    rules.required(errs, rules.FIELD_NAME, request.name)
    rules.max_length(errs, rules.FIELD_NAME, request.name, rules.NAME_MAX_LENGTH)
    rules.decimal_range(errs, rules.FIELD_TARGET, request.target_amount, MONEY_MIN_POSITIVE)
    rules.decimal_range(errs, rules.FIELD_CURRENT, request.current_amount, MONEY_MIN_ZERO)
    rules.max_length(errs, rules.FIELD_COLOR, request.color, rules.COLOR_MAX_LENGTH)
    rules.raise_if_any(errs)
    return request


@dataclass(slots=True)
class ContributeRequest:
    amount: Decimal | None


def parse_contribute(raw: Any) -> ContributeRequest:
    body = Body(raw)
    _guard(body, "amount")

    request = ContributeRequest(amount=body.money("amount"))
    errs = ValidationError()
    rules.decimal_range(errs, rules.FIELD_AMOUNT, request.amount, MONEY_MIN_POSITIVE)
    rules.raise_if_any(errs)
    return request


# --- recurring --------------------------------------------------------------


@dataclass(slots=True)
class RecurringRequest:
    account_id: uuid.UUID | None
    category_id: uuid.UUID | None
    type: TransactionType | None
    amount: Decimal | None
    description: str
    frequency: Frequency | None
    start_date: datetime | None
    end_date: datetime | None
    is_active: bool | None


def parse_recurring(raw: Any) -> RecurringRequest:
    body = Body(raw)
    _guard(body, "accountId", "type", "amount", "frequency", "startDate")

    request = RecurringRequest(
        account_id=body.uuid("accountId"),
        category_id=body.uuid("categoryId"),
        type=body.enum("type", TransactionType),
        amount=body.money("amount"),
        description=body.text("description"),
        frequency=body.enum("frequency", Frequency),
        start_date=body.moment("startDate"),
        end_date=body.moment("endDate"),
        is_active=body.flag("isActive"),
    )

    errs = ValidationError()
    rules.decimal_range(errs, rules.FIELD_AMOUNT, request.amount, MONEY_MIN_POSITIVE)
    rules.required(errs, rules.FIELD_DESCRIPTION, request.description)
    rules.max_length(
        errs, rules.FIELD_DESCRIPTION, request.description, rules.DESCRIPTION_MAX_LENGTH
    )
    rules.raise_if_any(errs)
    return request
