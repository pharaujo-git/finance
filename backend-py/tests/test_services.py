"""Balance, recurrence and CSV logic -- all pure, so no database is needed."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal

import pytest

from app.domain.enums import Frequency, TransactionType
from app.repositories.accounts import Account
from app.repositories.recurring import RecurringRule
from app.repositories.transactions import TransactionSlice
from app.services.balance import balance_of, delta_for, net_worth_delta
from app.services.csv_io import (
    escape_csv_field,
    parse_csv,
    parse_csv_date,
    parse_currency_amount,
)
from app.services.recurring import advance, materialize

CHECKING = uuid.UUID(int=1)
SAVINGS = uuid.UUID(int=2)


def _slice(kind: TransactionType, amount: str, *, account=CHECKING, transfer=None):
    return TransactionSlice(
        account_id=account,
        transfer_account_id=transfer,
        category_id=None,
        type=kind,
        amount=Decimal(amount),
        date=datetime(2026, 8, 1, tzinfo=UTC),
    )


class TestBalance:
    def test_income_credits_the_account(self) -> None:
        assert delta_for(CHECKING, _slice(TransactionType.INCOME, "100")) == Decimal("100")

    def test_expense_debits_the_account(self) -> None:
        assert delta_for(CHECKING, _slice(TransactionType.EXPENSE, "40")) == Decimal("-40")

    def test_a_transfer_debits_the_source(self) -> None:
        item = _slice(TransactionType.TRANSFER, "25", transfer=SAVINGS)
        assert delta_for(CHECKING, item) == Decimal("-25")

    def test_a_transfer_credits_the_destination(self) -> None:
        item = _slice(TransactionType.TRANSFER, "25", transfer=SAVINGS)
        assert delta_for(SAVINGS, item) == Decimal("25")

    def test_an_unrelated_account_is_untouched(self) -> None:
        item = _slice(TransactionType.INCOME, "100")
        assert delta_for(SAVINGS, item) == Decimal("0")

    def test_a_transfer_is_net_worth_neutral(self) -> None:
        item = _slice(TransactionType.TRANSFER, "25", transfer=SAVINGS)
        assert net_worth_delta(item) == Decimal("0")

    def test_net_worth_follows_income_and_expense(self) -> None:
        assert net_worth_delta(_slice(TransactionType.INCOME, "100")) == Decimal("100")
        assert net_worth_delta(_slice(TransactionType.EXPENSE, "40")) == Decimal("-40")

    def test_balance_starts_from_the_opening_amount(self) -> None:
        account = Account(
            id=CHECKING,
            user_id=uuid.uuid4(),
            name="Checking",
            type=0,  # type: ignore[arg-type]
            initial_balance=Decimal("1000.00"),
            currency="USD",
            is_archived=False,
            created_at=datetime(2026, 1, 1, tzinfo=UTC),
        )
        slices = [
            _slice(TransactionType.INCOME, "3000"),
            _slice(TransactionType.EXPENSE, "42.50"),
            _slice(TransactionType.TRANSFER, "250.75", transfer=SAVINGS),
        ]
        assert balance_of(account, slices) == Decimal("3706.75")


class TestAdvance:
    @pytest.mark.parametrize(
        ("frequency", "expected"),
        [
            (Frequency.DAILY, datetime(2026, 1, 2, tzinfo=UTC)),
            (Frequency.WEEKLY, datetime(2026, 1, 8, tzinfo=UTC)),
            (Frequency.MONTHLY, datetime(2026, 2, 1, tzinfo=UTC)),
            (Frequency.YEARLY, datetime(2027, 1, 1, tzinfo=UTC)),
        ],
    )
    def test_steps_by_the_frequency(self, frequency: Frequency, expected: datetime) -> None:
        assert advance(datetime(2026, 1, 1, tzinfo=UTC), frequency) == expected

    def test_monthly_clamps_the_day(self) -> None:
        assert advance(datetime(2026, 1, 31, tzinfo=UTC), Frequency.MONTHLY) == datetime(
            2026, 2, 28, tzinfo=UTC
        )


def _rule(**overrides) -> RecurringRule:
    base = {
        "id": uuid.uuid4(),
        "user_id": uuid.uuid4(),
        "account_id": CHECKING,
        "category_id": None,
        "type": TransactionType.EXPENSE,
        "amount": Decimal("9.99"),
        "description": "Streaming",
        "frequency": Frequency.MONTHLY,
        "start_date": datetime(2026, 1, 1, tzinfo=UTC),
        "end_date": None,
        "next_run_date": datetime(2026, 1, 1, tzinfo=UTC),
        "is_active": True,
    }
    return RecurringRule(**{**base, **overrides})


class TestMaterialize:
    def test_creates_one_per_period_up_to_the_cutoff(self) -> None:
        rule = _rule()
        created = materialize(rule, datetime(2026, 3, 15, tzinfo=UTC))
        assert [t.date.strftime("%Y-%m-%d") for t in created] == [
            "2026-01-01",
            "2026-02-01",
            "2026-03-01",
        ]
        assert rule.next_run_date == datetime(2026, 4, 1, tzinfo=UTC)

    def test_creates_nothing_before_the_first_run(self) -> None:
        rule = _rule(next_run_date=datetime(2026, 6, 1, tzinfo=UTC))
        assert materialize(rule, datetime(2026, 1, 1, tzinfo=UTC)) == []

    def test_tags_what_it_creates(self) -> None:
        created = materialize(_rule(), datetime(2026, 1, 2, tzinfo=UTC))
        assert created[0].tags == ["recurring"]
        assert created[0].notes is None
        assert created[0].transfer_account_id is None

    def test_an_occurrence_on_the_end_date_still_runs(self) -> None:
        rule = _rule(end_date=datetime(2026, 2, 1, tzinfo=UTC))
        created = materialize(rule, datetime(2026, 6, 1, tzinfo=UTC))
        assert len(created) == 2  # Jan 1 and Feb 1
        assert rule.is_active is False

    def test_retires_a_rule_past_its_end(self) -> None:
        rule = _rule(end_date=datetime(2025, 12, 1, tzinfo=UTC))
        assert materialize(rule, datetime(2026, 6, 1, tzinfo=UTC)) == []
        assert rule.is_active is False

    def test_caps_a_long_dormant_rule(self) -> None:
        rule = _rule(frequency=Frequency.DAILY)
        created = materialize(rule, datetime(2030, 1, 1, tzinfo=UTC))
        assert len(created) == 500
        # Still active, so the next pass carries on where this one stopped.
        assert rule.is_active is True

    def test_an_inactive_rule_produces_nothing(self) -> None:
        assert materialize(_rule(is_active=False), datetime(2030, 1, 1, tzinfo=UTC)) == []


class TestCsvReader:
    def test_reads_plain_rows(self) -> None:
        assert parse_csv("a,b\n1,2\n") == [["a", "b"], ["1", "2"]]

    def test_a_trailing_newline_adds_no_empty_row(self) -> None:
        assert len(parse_csv("a,b\n1,2\n\n")) == 2

    def test_handles_quoted_commas_and_quotes(self) -> None:
        assert parse_csv('"a,b","say ""hi"""\n') == [["a,b", 'say "hi"']]

    def test_a_quoted_field_may_span_lines(self) -> None:
        assert parse_csv('"line1\nline2",x\n') == [["line1\nline2", "x"]]

    def test_ignores_carriage_returns_outside_quotes(self) -> None:
        assert parse_csv("a,b\r\n1,2\r\n") == [["a", "b"], ["1", "2"]]

    def test_empty_input_is_no_rows(self) -> None:
        assert parse_csv("") == []


class TestCsvWriter:
    @pytest.mark.parametrize(
        ("value", "expected"),
        [
            ("plain", "plain"),
            ("has,comma", '"has,comma"'),
            ('has"quote', '"has""quote"'),
            ("has\nnewline", '"has\nnewline"'),
        ],
    )
    def test_quotes_only_when_needed(self, value: str, expected: str) -> None:
        assert escape_csv_field(value) == expected


class TestCsvValues:
    @pytest.mark.parametrize(
        ("value", "expected"),
        [
            ("12.50", "12.50"),
            ("$12.50", "12.50"),
            ("1,234.56", "1234.56"),
            ("(12.50)", "-12.50"),
            ("12.50-", "-12.50"),
            ("€9.99", "9.99"),
        ],
    )
    def test_reads_currency_shapes(self, value: str, expected: str) -> None:
        assert parse_currency_amount(value) == Decimal(expected)

    def test_rejects_nonsense(self) -> None:
        assert parse_currency_amount("abc") is None

    @pytest.mark.parametrize(
        "value",
        [
            "2026-08-26",
            "2026-08-26T10:00:00",
            "2026-08-26 10:00:00",
            "08/26/2026",
            "8/26/2026",
            "8/26/2026 3:04:05 PM",
        ],
    )
    def test_reads_the_import_date_layouts(self, value: str) -> None:
        assert parse_csv_date(value) is not None

    def test_rejects_a_bad_date(self) -> None:
        assert parse_csv_date("26/08/2026") is None
