"""CSV export and import.

The reader is hand-written rather than using the stdlib module: it has to match
the other backends byte for byte, including treating a bad row as *skipped*
rather than fatal, and ignoring bare carriage returns outside quotes.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal, InvalidOperation
from typing import Any, Final

from app.core.errors import validation
from app.domain.enums import TransactionType
from app.domain.money import ZERO, as_utc, join_tags, round_money, split_tags, trimmed_or_none
from app.repositories.accounts import AccountRepository
from app.repositories.categories import CategoryRepository
from app.repositories.transactions import Transaction, TransactionRepository

CSV_DATE_FORMAT: Final = "%Y-%m-%d"
CSV_HEADER: Final = [
    "Date",
    "Type",
    "Amount",
    "Account",
    "Category",
    "Description",
    "Notes",
    "Tags",
]
# The column separator for tags, distinct from the unit separator used in storage.
CSV_TAG_DELIMITER: Final = ";"

EMPTY_CSV_MESSAGE: Final = "The uploaded file contains no rows."
MISSING_FILE_MESSAGE: Final = "A non-empty CSV file is required."
OVERSIZE_MESSAGE: Final = "The uploaded file is larger than 5 MB."
MAX_UPLOAD_BYTES: Final = 5 * 1024 * 1024

# Most specific first; a value with no zone is read as UTC.
_IMPORT_DATE_FORMATS: Final = (
    "%Y-%m-%d",
    "%Y-%m-%dT%H:%M:%S",
    "%Y-%m-%d %H:%M:%S",
    "%m/%d/%Y",
    "%m/%d/%Y %H:%M:%S",
    "%m/%d/%Y %I:%M:%S %p",
)

_CURRENCY_SYMBOLS: Final = "¤$£€"


def export_file_name(now: datetime) -> str:
    return f"transactions-{as_utc(now).strftime(CSV_DATE_FORMAT)}.csv"


def escape_csv_field(value: str) -> str:
    """RFC 4180: quote only when needed, and double any internal quote."""
    if any(char in value for char in (",", '"', "\n", "\r")):
        return '"' + value.replace('"', '""') + '"'
    return value


def append_csv_row(out: list[str], fields: list[str]) -> None:
    # A bare \n, never \r\n -- that is what the other backends write.
    out.append(",".join(escape_csv_field(field) for field in fields) + "\n")


def parse_csv(text: str) -> list[list[str]]:
    """A minimal RFC 4180 reader.

    Outside quotes a carriage return is dropped entirely, so CRLF files parse
    the same as LF ones. A trailing newline produces no empty final record.
    """
    rows: list[list[str]] = []
    fields: list[str] = []
    buffer: list[str] = []
    quoted = False
    touched = False

    def commit_row() -> None:
        nonlocal fields, buffer, touched
        if not touched and not buffer and not fields:
            return
        fields.append("".join(buffer))
        rows.append(fields)
        fields = []
        buffer = []
        touched = False

    index = 0
    while index < len(text):
        char = text[index]
        if quoted:
            if char == '"':
                if index + 1 < len(text) and text[index + 1] == '"':
                    buffer.append('"')
                    index += 2
                    continue
                quoted = False
            else:
                # Quoted fields may span line breaks.
                buffer.append(char)
        elif char == '"':
            quoted = True
            touched = True
        elif char == ",":
            fields.append("".join(buffer))
            buffer = []
            touched = True
        elif char == "\r":
            pass
        elif char == "\n":
            commit_row()
        else:
            buffer.append(char)
            touched = True
        index += 1

    commit_row()
    return rows


def parse_csv_date(value: str) -> datetime | None:
    text = value.strip()
    if not text:
        return None

    candidate = text[:-1] + "+00:00" if text.endswith(("Z", "z")) else text
    try:
        parsed = datetime.fromisoformat(candidate)
        return parsed.replace(tzinfo=UTC) if parsed.tzinfo is None else parsed.astimezone(UTC)
    except ValueError:
        pass

    for layout in _IMPORT_DATE_FORMATS:
        try:
            return datetime.strptime(text, layout).replace(tzinfo=UTC)
        except ValueError:
            continue
    return None


def parse_currency_amount(value: str) -> Decimal | None:
    """Reads the shapes .NET's NumberStyles.Currency accepts."""
    text = value.strip()
    negative = False

    if text.startswith("(") and text.endswith(")"):
        negative = True
        text = text[1:-1].strip()

    text = text.lstrip(_CURRENCY_SYMBOLS).replace(",", "").strip()
    if text.endswith("-"):
        negative = True
        text = text[:-1]

    try:
        amount = Decimal(text.strip())
    except (InvalidOperation, ValueError):
        return None
    return -amount if negative else amount


def _field(row: list[str], index: int) -> str:
    return row[index].strip() if index < len(row) else ""


def _is_header_row(row: list[str]) -> bool:
    return bool(row) and row[0].strip().lower() == "date"


class CsvService:
    def __init__(
        self,
        transactions: TransactionRepository,
        accounts: AccountRepository,
        categories: CategoryRepository,
    ) -> None:
        self._transactions = transactions
        self._accounts = accounts
        self._categories = categories

    async def export(
        self, user_id: uuid.UUID, date_from: datetime | None, date_to: datetime | None
    ) -> str:
        items = await self._transactions.list_range(user_id, date_from, date_to)
        account_names = {a.id: a.name for a in await self._accounts.list_all(user_id)}
        category_names = {c.id: c.name for c in await self._categories.list_visible(user_id)}

        out: list[str] = []
        append_csv_row(out, CSV_HEADER)
        for item in items:
            append_csv_row(
                out,
                [
                    as_utc(item.date).strftime(CSV_DATE_FORMAT),
                    item.type.wire_name,
                    # Always exactly two places here, unlike the JSON shape.
                    f"{item.amount:.2f}",
                    account_names.get(item.account_id, ""),
                    category_names.get(item.category_id, "") if item.category_id else "",
                    item.description,
                    item.notes or "",
                    CSV_TAG_DELIMITER.join(split_tags(join_tags(item.tags))),
                ],
            )
        return "".join(out)

    async def import_csv(self, user_id: uuid.UUID, content: bytes) -> dict[str, Any]:
        text = content.decode("utf-8-sig", errors="replace")
        rows = parse_csv(text)
        if not rows:
            raise validation(EMPTY_CSV_MESSAGE)

        # Accounts: the last name wins. Categories: the first does. That
        # asymmetry is inherited from the .NET lookups and kept deliberately.
        account_ids: dict[str, uuid.UUID] = {}
        for account in await self._accounts.list_all(user_id):
            account_ids[account.name.upper()] = account.id

        category_ids: dict[str, uuid.UUID] = {}
        for category in await self._categories.list_visible(user_id):
            category_ids.setdefault(category.name.upper(), category.id)

        if _is_header_row(rows[0]):
            rows = rows[1:]

        imported: list[Transaction] = []
        skipped = 0
        for row in rows:
            built = self._build(user_id, row, account_ids, category_ids)
            if built is None:
                skipped += 1
            else:
                imported.append(built)

        if imported:
            await self._transactions.add_many(imported)
        return {"imported": len(imported), "skipped": skipped}

    def _build(
        self,
        user_id: uuid.UUID,
        row: list[str],
        account_ids: dict[str, uuid.UUID],
        category_ids: dict[str, uuid.UUID],
    ) -> Transaction | None:
        """Returns None for any unusable row; the caller counts it as skipped."""
        if len(row) < 6:
            return None

        date = parse_csv_date(_field(row, 0))
        if date is None:
            return None

        parsed_type = TransactionType.parse(_field(row, 1))
        # Transfers carry no destination in this CSV shape, so they never import.
        if parsed_type is None or parsed_type is TransactionType.TRANSFER:
            return None

        amount = parse_currency_amount(_field(row, 2))
        if amount is None or amount <= ZERO:
            return None

        account_id = account_ids.get(_field(row, 3).upper())
        if account_id is None:
            return None

        # A missing category is fine; the transaction is simply uncategorized.
        category_id = category_ids.get(_field(row, 4).upper())

        return Transaction(
            id=uuid.uuid4(),
            user_id=user_id,
            account_id=account_id,
            category_id=category_id,
            type=parsed_type,
            amount=round_money(amount),
            date=date,
            description=_field(row, 5),
            notes=trimmed_or_none(_field(row, 6)),
            tags=_field(row, 7).split(CSV_TAG_DELIMITER) if _field(row, 7) else [],
            transfer_account_id=None,
        )
