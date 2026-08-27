"""The four enums, twins of FinanceTracker.Domain.Enums.

Two wire formats have to agree at once:

  - the database stores the ordinal in an integer column, so the members must
    keep the declaration order of the C# enums;
  - JSON carries the camelCase member name, because the .NET API registers
    JsonStringEnumConverter(JsonNamingPolicy.CamelCase).

Reading accepts a member name in any casing or the ordinal as digits, matching
the .NET converter (AllowIntegerValues defaults to true).
"""

from __future__ import annotations

import enum
from typing import Self


class _WireEnum(enum.IntEnum):
    """An IntEnum that serialises as its camelCase name.

    An ordinal outside the declared set is preserved rather than rejected: the
    .NET converter accepts any number on read (AllowIntegerValues defaults to
    true) and writes an undefined value straight back out as a number. Rows
    already carrying such a value must round-trip, not crash the reader.
    """

    @classmethod
    def _missing_(cls, value: object) -> Self | None:
        if not isinstance(value, int) or isinstance(value, bool):
            return None
        # An unregistered pseudo-member: it compares and stores as its ordinal
        # but never appears in __members__, which is how the renderer knows to
        # write it as a bare number.
        pseudo = int.__new__(cls, value)
        pseudo._name_ = str(value)
        pseudo._value_ = value
        return pseudo

    @property
    def is_defined(self) -> bool:
        return 0 <= int(self) < len(_WIRE_NAMES[type(self)])

    @property
    def wire_name(self) -> str:
        """The camelCase name, or the bare ordinal for an undefined value."""
        if self.is_defined:
            return _WIRE_NAMES[type(self)][int(self)]
        return str(int(self))

    def __str__(self) -> str:
        return self.wire_name

    @classmethod
    def parse(cls, value: str | int | None) -> Self | None:
        """Reads a member name in any casing, or an ordinal. None if unusable."""
        if value is None or isinstance(value, bool):
            return None
        if isinstance(value, int):
            return cls(value)

        text = value.strip()
        if not text:
            return None
        for ordinal, name in enumerate(_WIRE_NAMES[cls]):
            if name.casefold() == text.casefold():
                return cls(ordinal)
        try:
            return cls(int(text))
        except ValueError:
            return None


class AccountType(_WireEnum):
    CHECKING = 0
    SAVINGS = 1
    CREDIT_CARD = 2
    CASH = 3
    INVESTMENT = 4


class CategoryType(_WireEnum):
    INCOME = 0
    EXPENSE = 1


class TransactionType(_WireEnum):
    INCOME = 0
    EXPENSE = 1
    TRANSFER = 2


class Frequency(_WireEnum):
    DAILY = 0
    WEEKLY = 1
    MONTHLY = 2
    YEARLY = 3


# Wire names in ordinal order. "creditCard" is the one member whose camelCase
# form differs from a plain lowercase of the C# name.
_WIRE_NAMES: dict[type[_WireEnum], tuple[str, ...]] = {
    AccountType: ("checking", "savings", "creditCard", "cash", "investment"),
    CategoryType: ("income", "expense"),
    TransactionType: ("income", "expense", "transfer"),
    Frequency: ("daily", "weekly", "monthly", "yearly"),
}
