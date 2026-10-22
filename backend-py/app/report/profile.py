"""The household profile the report is built from.

The profile is a plain JSON file the owner edits by hand: income, bills, debts,
cars, savings and goals. Nothing is hard-coded about any one household -- a
second mortgage, a third car or a new goal is just another entry in a list, and
the next run of the report picks it up.

Every amount is a Decimal, read from a JSON number or a quoted number, exactly
as the API reads money (see app.domain.money.parse_money).
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import date
from decimal import Decimal
from pathlib import Path
from typing import Any, Final

from app.domain.money import ZERO, parse_money

# The expense categories the report knows how to reason about. Anything else is
# accepted and shown, it just gets no category-specific suggestion.
KNOWN_CATEGORIES: Final = (
    "housing",
    "utilities",
    "transportation",
    "insurance",
    "groceries",
    "dining",
    "subscriptions",
    "phone_internet",
    "health",
    "childcare",
    "education",
    "shopping",
    "entertainment",
    "travel",
    "personal",
    "gifts",
    "pets",
    "other",
)

CURRENCY_SYMBOLS: Final = {
    "USD": "$",
    "CAD": "$",
    "AUD": "$",
    "EUR": "€",
    "GBP": "£",
    "BRL": "R$",
}

DEBT_KINDS: Final = ("mortgage", "auto", "credit_card", "student", "personal", "other")


class ProfileError(ValueError):
    """The profile file is missing a field or carries an unusable value."""


@dataclass(slots=True)
class Income:
    name: str
    monthly: Decimal


@dataclass(slots=True)
class Expense:
    name: str
    category: str
    monthly: Decimal
    # Needs versus wants, for the 50/30/20 split. Defaults by category.
    essential: bool


@dataclass(slots=True)
class Debt:
    name: str
    kind: str
    monthly_payment: Decimal
    balance: Decimal | None = None
    apr: Decimal | None = None  # percent, e.g. 6.75
    # The part of the payment that is not the loan itself: a mortgage's escrow
    # for property tax and homeowner's insurance. It never reduces the balance.
    escrow: Decimal = ZERO


@dataclass(slots=True)
class Vehicle:
    name: str
    value: Decimal | None = None
    year: int | None = None
    # The name of the Debt that finances it; None means it is paid off.
    loan: str | None = None
    full_coverage: bool = True

    @property
    def paid_off(self) -> bool:
        return self.loan is None


@dataclass(slots=True)
class Account:
    name: str
    type: str  # checking, savings, investment, retirement, cash
    balance: Decimal


@dataclass(slots=True)
class Asset:
    """Something owned outside an account: the house, a boat, land."""

    name: str
    value: Decimal


@dataclass(slots=True)
class Goal:
    name: str
    target: Decimal
    current: Decimal = ZERO
    target_date: date | None = None
    monthly_contribution: Decimal | None = None


@dataclass(slots=True)
class Profile:
    household: str
    currency: str
    as_of: date
    income: list[Income] = field(default_factory=list)
    expenses: list[Expense] = field(default_factory=list)
    debts: list[Debt] = field(default_factory=list)
    vehicles: list[Vehicle] = field(default_factory=list)
    accounts: list[Account] = field(default_factory=list)
    assets: list[Asset] = field(default_factory=list)
    goals: list[Goal] = field(default_factory=list)
    # Monthly amounts already going to savings and retirement out of take-home pay.
    monthly_savings: Decimal = ZERO
    monthly_retirement: Decimal = ZERO
    # Whether the full employer 401(k)/pension match is being captured.
    employer_match_captured: bool | None = None
    emergency_fund_months_target: int = 6
    notes: list[str] = field(default_factory=list)

    @property
    def currency_symbol(self) -> str:
        return CURRENCY_SYMBOLS.get(self.currency, f"{self.currency} ")


# Categories that are needs by default; everything else counts as a want.
_ESSENTIAL_BY_DEFAULT: Final = frozenset(
    {
        "housing",
        "utilities",
        "transportation",
        "insurance",
        "groceries",
        "phone_internet",
        "health",
        "childcare",
        "education",
    }
)


def load_profile(path: str | Path, *, today: date | None = None) -> Profile:
    try:
        raw = json.loads(Path(path).read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ProfileError(f"Profile file not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise ProfileError(f"Profile is not valid JSON: {exc}") from exc
    return parse_profile(raw, today=today)


def parse_profile(raw: Any, *, today: date | None = None) -> Profile:
    if not isinstance(raw, dict):
        raise ProfileError("The profile must be a JSON object.")

    as_of = _date(raw.get("as_of"), "as_of") or today or date.today()
    profile = Profile(
        household=_text(raw.get("household"), "household", default="My household"),
        currency=_text(raw.get("currency"), "currency", default="USD").upper(),
        as_of=as_of,
        monthly_savings=_money(raw.get("monthly_savings"), "monthly_savings", default=ZERO),
        monthly_retirement=_money(
            raw.get("monthly_retirement"), "monthly_retirement", default=ZERO
        ),
        employer_match_captured=_flag(raw.get("employer_match_captured"), default=None),
        emergency_fund_months_target=int(
            _money(
                raw.get("emergency_fund_months_target"),
                "emergency_fund_months_target",
                default=Decimal(6),
            )
        ),
        notes=[str(note) for note in _list(raw.get("notes"), "notes")],
    )

    for i, item in enumerate(_list(raw.get("income"), "income")):
        where = f"income[{i}]"
        profile.income.append(
            Income(
                name=_text(item.get("name"), f"{where}.name"),
                monthly=_money(item.get("monthly"), f"{where}.monthly"),
            )
        )

    for i, item in enumerate(_list(raw.get("expenses"), "expenses")):
        where = f"expenses[{i}]"
        category = _text(item.get("category"), f"{where}.category", default="other").lower()
        profile.expenses.append(
            Expense(
                name=_text(item.get("name"), f"{where}.name"),
                category=category,
                monthly=_money(item.get("monthly"), f"{where}.monthly"),
                essential=_flag(item.get("essential"), default=category in _ESSENTIAL_BY_DEFAULT)
                is True,
            )
        )

    for i, item in enumerate(_list(raw.get("debts"), "debts")):
        where = f"debts[{i}]"
        kind = _text(item.get("kind"), f"{where}.kind", default="other").lower()
        if kind not in DEBT_KINDS:
            raise ProfileError(f"{where}.kind must be one of: {', '.join(DEBT_KINDS)}.")
        profile.debts.append(
            Debt(
                name=_text(item.get("name"), f"{where}.name"),
                kind=kind,
                monthly_payment=_money(item.get("monthly_payment"), f"{where}.monthly_payment"),
                balance=_optional_money(item.get("balance"), f"{where}.balance"),
                apr=_optional_money(item.get("apr"), f"{where}.apr"),
                escrow=_money(item.get("escrow"), f"{where}.escrow", default=ZERO),
            )
        )

    debt_names = {debt.name for debt in profile.debts}
    for i, item in enumerate(_list(raw.get("vehicles"), "vehicles")):
        where = f"vehicles[{i}]"
        loan = item.get("loan")
        if loan is not None and loan not in debt_names:
            raise ProfileError(f"{where}.loan names no debt: {loan!r}.")
        year = item.get("year")
        profile.vehicles.append(
            Vehicle(
                name=_text(item.get("name"), f"{where}.name"),
                value=_optional_money(item.get("value"), f"{where}.value"),
                year=int(year) if year is not None else None,
                loan=loan,
                full_coverage=_flag(item.get("full_coverage"), default=True) is True,
            )
        )

    for i, item in enumerate(_list(raw.get("accounts"), "accounts")):
        where = f"accounts[{i}]"
        profile.accounts.append(
            Account(
                name=_text(item.get("name"), f"{where}.name"),
                type=_text(item.get("type"), f"{where}.type", default="checking").lower(),
                balance=_money(item.get("balance"), f"{where}.balance"),
            )
        )

    for i, item in enumerate(_list(raw.get("assets"), "assets")):
        where = f"assets[{i}]"
        profile.assets.append(
            Asset(
                name=_text(item.get("name"), f"{where}.name"),
                value=_money(item.get("value"), f"{where}.value"),
            )
        )

    for i, item in enumerate(_list(raw.get("goals"), "goals")):
        where = f"goals[{i}]"
        profile.goals.append(
            Goal(
                name=_text(item.get("name"), f"{where}.name"),
                target=_money(item.get("target"), f"{where}.target"),
                current=_money(item.get("current"), f"{where}.current", default=ZERO),
                target_date=_date(item.get("target_date"), f"{where}.target_date"),
                monthly_contribution=_optional_money(
                    item.get("monthly_contribution"), f"{where}.monthly_contribution"
                ),
            )
        )

    return profile


def _list(value: Any, where: str) -> list[Any]:
    if value is None:
        return []
    if not isinstance(value, list):
        raise ProfileError(f"{where} must be a list.")
    for item in value:
        if where != "notes" and not isinstance(item, dict):
            raise ProfileError(f"Every entry in {where} must be an object.")
    return value


def _text(value: Any, where: str, *, default: str | None = None) -> str:
    if value is None or (isinstance(value, str) and not value.strip()):
        if default is None:
            raise ProfileError(f"{where} is required.")
        return default
    return str(value).strip()


def _money(value: Any, where: str, *, default: Decimal | None = None) -> Decimal:
    if value is None:
        if default is None:
            raise ProfileError(f"{where} is required.")
        return default
    amount = parse_money(value)
    if amount is None:
        raise ProfileError(f"{where} must be a number.")
    if amount < 0:
        raise ProfileError(f"{where} cannot be negative.")
    return amount


def _optional_money(value: Any, where: str) -> Decimal | None:
    return None if value is None else _money(value, where)


def _flag(value: Any, *, default: bool | None) -> bool | None:
    return default if value is None else bool(value)


def _date(value: Any, where: str) -> date | None:
    """A full date, or a YYYY-MM month read as its first day."""
    if value is None or value == "":
        return None
    text = str(value).strip()
    try:
        return date.fromisoformat(text if len(text) > 7 else f"{text}-01")
    except ValueError as exc:
        raise ProfileError(f"{where} must be YYYY-MM or YYYY-MM-DD.") from exc
