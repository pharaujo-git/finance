"""Money, dates, enums and the validation rules."""

from __future__ import annotations

from datetime import UTC, datetime
from decimal import Decimal

import pytest

from app.api.rendering import dumps
from app.core.errors import ValidationError
from app.domain import validation as rules
from app.domain.dates import (
    add_months,
    add_years,
    month_from,
    start_of_month,
    trailing_months,
    try_parse_month,
)
from app.domain.enums import AccountType, Frequency, TransactionType
from app.domain.money import (
    join_tags,
    parse_wire_date,
    render_amount,
    render_datetime,
    round_money,
    split_tags,
    trim_money,
)


class TestRoundMoney:
    @pytest.mark.parametrize(
        ("value", "expected"),
        [
            ("1.005", "1.01"),
            ("-1.005", "-1.01"),
            ("2.344", "2.34"),
            ("2.345", "2.35"),
            ("10", "10"),  # scale is never lengthened
            ("0.1", "0.1"),
            # Rounds to zero, and the sign goes with it: never "-0.00".
            ("-0.0049", "0.00"),
        ],
    )
    def test_matches_the_shared_vectors(self, value: str, expected: str) -> None:
        assert str(round_money(Decimal(value))) == expected


class TestRendering:
    @pytest.mark.parametrize(
        ("value", "expected"),
        [("1250.00", "1250.00"), ("12.5", "12.5"), ("10", "10"), ("0", "0"), ("-0.50", "-0.50")],
    )
    def test_keeps_the_scale_it_carries(self, value: str, expected: str) -> None:
        assert render_amount(Decimal(value)) == expected

    def test_money_is_a_bare_json_number(self) -> None:
        assert dumps({"balance": Decimal("1250.00")}) == '{"balance":1250.00}'

    def test_a_defined_enum_is_its_camel_case_name(self) -> None:
        assert dumps({"type": AccountType.CREDIT_CARD}) == '{"type":"creditCard"}'

    def test_an_undefined_ordinal_is_a_bare_number(self) -> None:
        # The .NET converter round-trips a value naming no member.
        assert dumps({"type": AccountType(99)}) == '{"type":99}'

    def test_trims_trailing_zeros_from_a_timestamp(self) -> None:
        moment = datetime(2026, 8, 26, 22, 27, 26, 516550, tzinfo=UTC)
        assert render_datetime(moment) == "2026-08-26T22:27:26.51655Z"

    def test_omits_the_fraction_when_it_is_zero(self) -> None:
        moment = datetime(2026, 8, 26, 22, 27, 26, tzinfo=UTC)
        assert render_datetime(moment) == "2026-08-26T22:27:26Z"

    def test_text_cannot_forge_a_number(self) -> None:
        import json

        rendered = dumps({"text": '1,"balance":999'})
        assert json.loads(rendered) == {"text": '1,"balance":999'}


class TestTrimMoney:
    @pytest.mark.parametrize(
        ("value", "expected"), [("0.7500", "0.75"), ("0.5000", "0.5"), ("0", "0"), ("100", "100")]
    )
    def test_drops_trailing_zeros(self, value: str, expected: str) -> None:
        assert str(trim_money(Decimal(value))) == expected


class TestAddMonths:
    def test_clamps_rather_than_rolling_over(self) -> None:
        jan31 = datetime(2026, 1, 31, tzinfo=UTC)
        assert add_months(jan31, 1) == datetime(2026, 2, 28, tzinfo=UTC)
        # Clamping is not remembered: Feb 28 + 1 month is Mar 28, not Mar 31.
        assert add_months(add_months(jan31, 1), 1) == datetime(2026, 3, 28, tzinfo=UTC)

    def test_walks_backwards(self) -> None:
        assert add_months(datetime(2026, 1, 15, tzinfo=UTC), -1) == datetime(
            2025, 12, 15, tzinfo=UTC
        )

    def test_leap_day_clamps_in_a_common_year(self) -> None:
        assert add_years(datetime(2028, 2, 29, tzinfo=UTC), 1) == datetime(2029, 2, 28, tzinfo=UTC)

    def test_preserves_the_clock(self) -> None:
        moment = datetime(2026, 3, 10, 14, 30, 45, 123456, tzinfo=UTC)
        assert add_months(moment, 1).timetz() == moment.timetz()


class TestMonths:
    def test_start_of_month(self) -> None:
        assert start_of_month(datetime(2026, 8, 26, 13, 5, tzinfo=UTC)) == datetime(
            2026, 8, 1, tzinfo=UTC
        )

    def test_month_from(self) -> None:
        assert month_from(datetime(2026, 8, 26, tzinfo=UTC)) == "2026-08"

    def test_trailing_months_ends_on_the_reference(self) -> None:
        window = trailing_months(datetime(2026, 3, 15, tzinfo=UTC), 3)
        assert [month_from(m) for m in window] == ["2026-01", "2026-02", "2026-03"]

    @pytest.mark.parametrize("value", ["2026-13", "nope", "", "2026", "2026-1-1"])
    def test_rejects_a_bad_month(self, value: str) -> None:
        assert try_parse_month(value) is None


class TestWireDates:
    @pytest.mark.parametrize(
        "value",
        ["2026-08-26", "2026-08-26T10:00:00", "2026-08-26T10:00", "2026-08-26T10:00:00Z"],
    )
    def test_accepts_the_supported_layouts(self, value: str) -> None:
        assert parse_wire_date(value) is not None

    def test_reads_a_naive_value_as_utc(self) -> None:
        assert parse_wire_date("2026-08-26T10:00:00").tzinfo == UTC  # type: ignore[union-attr]

    def test_rejects_nonsense(self) -> None:
        assert parse_wire_date("last tuesday") is None


class TestEnums:
    @pytest.mark.parametrize("value", ["creditCard", "CREDITCARD", "creditcard", " creditCard "])
    def test_parses_a_name_in_any_casing(self, value: str) -> None:
        assert AccountType.parse(value) is AccountType.CREDIT_CARD

    def test_parses_an_ordinal(self) -> None:
        assert TransactionType.parse(2) is TransactionType.TRANSFER
        assert TransactionType.parse("2") is TransactionType.TRANSFER

    def test_preserves_an_undefined_ordinal(self) -> None:
        parsed = AccountType.parse(99)
        assert int(parsed) == 99  # type: ignore[arg-type]
        assert not parsed.is_defined  # type: ignore[union-attr]

    def test_rejects_an_unknown_name(self) -> None:
        assert AccountType.parse("rust") is None

    def test_wire_names_are_stable(self) -> None:
        assert [t.wire_name for t in Frequency] == ["daily", "weekly", "monthly", "yearly"]
        assert AccountType.CREDIT_CARD.wire_name == "creditCard"


class TestTags:
    def test_round_trips(self) -> None:
        assert split_tags(join_tags(["food", "out"])) == ["food", "out"]

    def test_drops_blanks_and_trims(self) -> None:
        assert split_tags(join_tags([" food ", "  ", "", "out"])) == ["food", "out"]

    def test_empty_is_a_list_not_none(self) -> None:
        assert split_tags("") == []
        assert split_tags(None) == []


class TestValidationMessages:
    def test_required_wording(self) -> None:
        errs = ValidationError()
        rules.required(errs, "Email", "   ")
        assert errs.errors == {"Email": ["The Email field is required."]}

    def test_max_length_wording_puts_the_field_second(self) -> None:
        errs = ValidationError()
        rules.max_length(errs, "Name", "n" * 201, 200)
        assert errs.errors["Name"] == [
            "The field Name must be a string or array type with a maximum length of '200'."
        ]

    def test_range_wording_quotes_the_bounds_verbatim(self) -> None:
        errs = ValidationError()
        rules.decimal_range(errs, "Amount", Decimal("0"), "0.01")
        assert errs.errors["Amount"] == [
            "The field Amount must be between 0.01 and 999999999999.99."
        ]

    @pytest.mark.parametrize(
        ("value", "valid"),
        [("a@b.c", True), ("bad", False), ("@b.c", False), ("a@", False), ("a@b@c", False)],
    )
    def test_email_shape(self, value: str, valid: bool) -> None:
        errs = ValidationError()
        rules.email_address(errs, "Email", value)
        assert errs.empty is valid

    def test_required_members_short_circuits(self) -> None:
        errs = ValidationError()
        assert rules.required_members(errs, ["accountId", "type"]) is True
        assert errs.errors == {
            "$": [
                "The JSON payload was missing required properties, "
                "including the following: accountId, type"
            ]
        }
