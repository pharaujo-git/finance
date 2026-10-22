"""The household finance report: profile parsing, the arithmetic, the rules, the page."""

from __future__ import annotations

import json
import re
from datetime import date
from decimal import Decimal
from pathlib import Path
from typing import Any

import pytest

from app.report.__main__ import main
from app.report.analysis import add_months, amortize, analyze, level_payment
from app.report.profile import ProfileError, load_profile, parse_profile
from app.report.render import potential_savings, render

TODAY = date(2026, 10, 22)
EXAMPLE = Path(__file__).resolve().parents[1] / "reports" / "profile.example.json"


def household(**overrides: Any) -> dict[str, Any]:
    raw: dict[str, Any] = {
        "household": "Test",
        "as_of": "2026-10-22",
        "income": [{"name": "Pay", "monthly": 9000}],
        "debts": [
            {
                "name": "Mortgage",
                "kind": "mortgage",
                "monthly_payment": 3100,
                "balance": 400000,
                "apr": 6.75,
                "escrow": 500,
            },
            {
                "name": "Car loan",
                "kind": "auto",
                "monthly_payment": 450,
                "balance": 10000,
                "apr": 7,
            },
        ],
        "vehicles": [
            {"name": "Old car", "year": 2015, "value": 5000},
            {"name": "New car", "year": 2023, "value": 25000, "loan": "Car loan"},
        ],
        "expenses": [
            {"name": "Groceries", "category": "groceries", "monthly": 900},
            {"name": "Eating out", "category": "dining", "monthly": 600},
        ],
        "accounts": [{"name": "Savings", "type": "savings", "balance": 5000}],
    }
    raw.update(overrides)
    return raw


def titles(raw: dict[str, Any]) -> list[str]:
    return [s.title for s in analyze(parse_profile(raw)).suggestions]


class TestProfile:
    def test_reads_money_as_decimal_and_months_as_dates(self) -> None:
        profile = parse_profile(
            household(goals=[{"name": "Trip", "target": "5000.50", "target_date": "2027-07"}])
        )
        assert profile.goals[0].target == Decimal("5000.50")
        assert profile.goals[0].target_date == date(2027, 7, 1)
        assert profile.as_of == TODAY

    def test_a_vehicle_with_no_loan_is_paid_off(self) -> None:
        profile = parse_profile(household())
        assert [v.paid_off for v in profile.vehicles] == [True, False]

    def test_categories_default_to_needs_or_wants(self) -> None:
        profile = parse_profile(household())
        assert [e.essential for e in profile.expenses] == [True, False]

    @pytest.mark.parametrize(
        ("change", "message"),
        [
            ({"income": [{"name": "Pay"}]}, "income[0].monthly is required"),
            ({"income": [{"name": "Pay", "monthly": -1}]}, "cannot be negative"),
            ({"income": [{"name": "Pay", "monthly": "lots"}]}, "must be a number"),
            ({"vehicles": [{"name": "Car", "loan": "Nope"}]}, "names no debt"),
            ({"debts": [{"name": "X", "kind": "boat", "monthly_payment": 1}]}, "kind must be"),
            ({"as_of": "soon"}, "YYYY-MM"),
            ({"expenses": {"name": "x"}}, "must be a list"),
        ],
    )
    def test_rejects_bad_values_naming_the_field(
        self, change: dict[str, Any], message: str
    ) -> None:
        with pytest.raises(ProfileError, match=re.escape(message)):
            parse_profile(household(**change))

    def test_the_example_profile_loads(self) -> None:
        profile = load_profile(EXAMPLE, today=TODAY)
        assert {d.monthly_payment for d in profile.debts} == {Decimal(3100), Decimal(450)}

    def test_a_missing_file_is_a_profile_error(self, tmp_path: Path) -> None:
        with pytest.raises(ProfileError, match="not found"):
            load_profile(tmp_path / "nope.json")


class TestArithmetic:
    def test_amortize_matches_a_level_payment(self) -> None:
        payment = level_payment(Decimal(20000), Decimal(6), 60)
        assert payment == Decimal("386.66")
        payoff = amortize(Decimal(20000), Decimal(6), payment)
        assert payoff.months == 60

    def test_a_payment_below_the_interest_never_pays_off(self) -> None:
        assert amortize(Decimal(100000), Decimal(12), Decimal(500)).months is None

    def test_zero_interest(self) -> None:
        assert amortize(Decimal(1200), Decimal(0), Decimal(100)).months == 12
        assert level_payment(Decimal(1200), Decimal(0), 12) == Decimal(100)

    def test_add_months_crosses_years(self) -> None:
        assert add_months(date(2026, 11, 22), 3) == date(2027, 2, 1)


class TestAnalysis:
    def test_totals(self) -> None:
        a = analyze(parse_profile(household(monthly_savings=400, monthly_retirement=500)))
        assert a.income == 9000
        assert a.debt_payments == 3550
        assert a.surplus == 9000 - 1500 - 3550 - 900
        assert a.needs == 900 + 3550
        assert a.wants == 600

    def test_loan_payments_join_their_category(self) -> None:
        a = analyze(parse_profile(household()))
        totals = dict(a.by_category)
        assert totals["housing"] == 3100
        assert totals["transportation"] == 450

    def test_escrow_does_not_pay_down_the_mortgage(self) -> None:
        a = analyze(parse_profile(household()))
        mortgage = a.debts[0]
        assert mortgage.principal_payment == 2600
        assert mortgage.payoff is not None and mortgage.payoff.months is not None
        # Extra principal always finishes sooner and costs less.
        assert all(s.months_saved > 0 and s.interest_saved > 0 for s in mortgage.scenarios)

    def test_the_car_loan_ending_frees_its_payment_in_the_projection(self) -> None:
        a = analyze(parse_profile(household()))
        car = a.debts[1]
        assert car.payoff is not None and car.payoff.months == 24
        assert a.projection[23].freed == 0
        a = analyze(
            parse_profile(
                household(
                    debts=[
                        {
                            "name": "Car loan",
                            "kind": "auto",
                            "monthly_payment": 450,
                            "balance": 2000,
                            "apr": 7,
                        },
                    ]
                )
            )
        )
        assert a.projection[-1].freed == 450

    def test_net_worth_counts_property_and_cars(self) -> None:
        a = analyze(parse_profile(household(assets=[{"name": "Home", "value": 500000}])))
        assert a.net_worth == 5000 + 500000 + 5000 + 25000 - 400000 - 10000

    def test_goal_on_track(self) -> None:
        a = analyze(
            parse_profile(
                household(
                    goals=[
                        {
                            "name": "Trip",
                            "target": 1200,
                            "target_date": "2027-10",
                            "monthly_contribution": 100,
                        },
                        {
                            "name": "Car",
                            "target": 12000,
                            "target_date": "2027-10",
                            "monthly_contribution": 100,
                        },
                        {"name": "Done", "target": 10, "current": 10},
                    ]
                )
            )
        )
        trip, car, done = a.goals
        assert trip.required_monthly == 100 and trip.on_track is True
        assert car.on_track is False
        assert done.on_track is True and done.progress == 1


class TestSuggestions:
    def test_the_two_car_household_gets_car_ideas(self) -> None:
        found = titles(household())
        assert "When Car loan is paid off, keep paying it — to yourself" in found
        assert "Review collision/comprehensive on Old car" in found
        assert "Re-shop insurance for all 2 cars together" in found

    def test_follows_the_profile_when_the_loan_is_gone(self) -> None:
        raw = household(
            debts=[household()["debts"][0]],
            vehicles=[{"name": "Old car", "year": 2015}, {"name": "New car", "year": 2023}],
        )
        found = titles(raw)
        assert not any("Car loan" in t for t in found)

    def test_overspending_comes_first(self) -> None:
        found = analyze(parse_profile(household(income=[{"name": "Pay", "monthly": 4000}])))
        assert found.suggestions[0].title.startswith("Close a monthly gap")

    def test_high_interest_debt_is_ordered_by_rate(self) -> None:
        debts = [
            {
                "name": "Card A",
                "kind": "credit_card",
                "monthly_payment": 50,
                "balance": 1000,
                "apr": 19,
            },
            {
                "name": "Card B",
                "kind": "credit_card",
                "monthly_payment": 50,
                "balance": 1000,
                "apr": 27,
            },
        ]
        a = analyze(parse_profile(household(debts=debts, vehicles=[])))
        avalanche = next(s for s in a.suggestions if "avalanche" in s.title)
        assert avalanche.priority == "high"
        assert avalanche.detail.index("Card B") < avalanche.detail.index("Card A")

    def test_missed_match_is_flagged(self) -> None:
        assert "Capture the full employer retirement match" in titles(
            household(employer_match_captured=False)
        )

    def test_priorities_are_sorted(self) -> None:
        order = {"high": 0, "medium": 1, "low": 2}
        a = analyze(parse_profile(household()))
        ranks = [order[s.priority] for s in a.suggestions]
        assert ranks == sorted(ranks)

    def test_an_empty_profile_still_reports(self) -> None:
        a = analyze(parse_profile({"as_of": "2026-10-01"}))
        assert a.suggestions[0].title == "Add your income to the profile"
        assert "<html" in render(a)


class TestRender:
    def test_every_section_is_on_the_page(self) -> None:
        page = render(analyze(load_profile(EXAMPLE, today=TODAY)))
        for anchor in (
            "summary",
            "cash-flow",
            "spending",
            "ideas",
            "debts",
            "cars",
            "safety-net",
            "goals",
            "projection",
            "plan",
            "net-worth",
        ):
            assert f'id="{anchor}"' in page

    def test_text_from_the_profile_is_escaped(self) -> None:
        raw = household(
            household="<script>x</script>",
            expenses=[{"name": "<b>Gym</b>", "category": "subscriptions", "monthly": 60}],
        )
        page = render(analyze(parse_profile(raw)))
        assert "<b>Gym</b>" not in page and "&lt;b&gt;Gym&lt;/b&gt;" in page
        assert "<script>x</script>" not in page

    def test_potential_savings_only_counts_estimates(self) -> None:
        a = analyze(parse_profile(household()))
        assert potential_savings(a) == sum(
            s.monthly_impact for s in a.suggestions if s.monthly_impact is not None
        )


class TestCommandLine:
    def test_writes_the_page(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        profile = tmp_path / "me.json"
        profile.write_text(json.dumps(household()), encoding="utf-8")
        assert main([str(profile)]) == 0
        assert (tmp_path / "me.html").read_text(encoding="utf-8").startswith("<!doctype html>")
        assert "ideas" in capsys.readouterr().out

    def test_a_bad_profile_exits_2(self, tmp_path: Path) -> None:
        profile = tmp_path / "me.json"
        profile.write_text("{", encoding="utf-8")
        assert main([str(profile)]) == 2
