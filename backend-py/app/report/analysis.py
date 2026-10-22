"""Turns a Profile into the numbers and suggestions the report shows.

Everything is derived from the profile on each run, so the report follows the
household as it changes: pay off a loan, add a goal or cancel a subscription
and the totals, timelines and suggestions all move with it.

Amounts stay Decimal. Rates are percentages (6.75 means 6.75% APR).
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import date
from decimal import ROUND_HALF_UP, Decimal
from typing import Final

from app.report.profile import Debt, Expense, Goal, Profile, Vehicle

ZERO: Final = Decimal(0)
HUNDRED: Final = Decimal(100)
CENTS: Final = Decimal("0.01")

# A loan that would take longer than this never pays off at its current payment.
MAX_TERM_MONTHS: Final = 600

# How many months the cash projection runs for.
PROJECTION_MONTHS: Final = 24

# The extra monthly principal the payoff scenarios try.
EXTRA_PAYMENTS: Final = (Decimal(100), Decimal(250))

# Thresholds the rules use. Rough, widely quoted rules of thumb -- not advice
# tailored to any one household, which is why the report shows its working.
TARGET_SAVINGS_RATE: Final = Decimal("0.20")
HOUSING_SHARE_WARNING: Final = Decimal("0.30")  # of take-home pay
TRANSPORT_SHARE_WARNING: Final = Decimal("0.15")
HIGH_INTEREST_APR: Final = Decimal(10)
REFINANCE_WATCH_APR: Final = Decimal(6)
OLD_CAR_YEARS: Final = 8
LOW_CAR_VALUE: Final = Decimal(6000)
DINING_SHARE_WARNING: Final = Decimal("0.05")
PHONE_INTERNET_WARNING: Final = Decimal(150)

LIQUID_ACCOUNT_TYPES: Final = frozenset({"checking", "savings", "cash"})

# Debt payments land in the spending breakdown under these categories.
_DEBT_CATEGORY: Final = {"mortgage": "housing", "auto": "transportation"}


def money(value: Decimal) -> Decimal:
    return value.quantize(CENTS, rounding=ROUND_HALF_UP)


def share(part: Decimal, whole: Decimal) -> Decimal:
    """part / whole as a fraction, 0 when there is no whole."""
    return part / whole if whole > 0 else ZERO


def add_months(day: date, months: int) -> date:
    total = day.month - 1 + months
    return date(day.year + total // 12, total % 12 + 1, 1)


def months_between(start: date, end: date) -> int:
    return (end.year - start.year) * 12 + (end.month - start.month)


@dataclass(slots=True)
class Payoff:
    """How a loan runs out at a given monthly principal-and-interest payment."""

    months: int | None  # None: the payment never covers the interest
    interest: Decimal

    @property
    def pays_off(self) -> bool:
        return self.months is not None


def amortize(balance: Decimal, apr: Decimal, payment: Decimal) -> Payoff:
    """Month-by-month amortization; the final payment is only what is owed."""
    rate = apr / HUNDRED / 12
    remaining = balance
    interest_total = ZERO
    months = 0
    while remaining > 0:
        if months >= MAX_TERM_MONTHS:
            return Payoff(None, money(interest_total))
        interest = money(remaining * rate)
        if payment <= interest:
            return Payoff(None, money(interest_total))
        interest_total += interest
        remaining = remaining + interest - payment
        months += 1
    return Payoff(months, money(interest_total))


def level_payment(balance: Decimal, apr: Decimal, months: int) -> Decimal:
    """The fixed payment that clears `balance` in `months` at `apr`."""
    if months <= 0:
        return balance
    rate = apr / HUNDRED / 12
    if rate == 0:
        return money(balance / months)
    factor = (1 + rate) ** months
    return money(balance * rate * factor / (factor - 1))


@dataclass(slots=True)
class Scenario:
    extra: Decimal
    months: int
    months_saved: int
    interest_saved: Decimal


@dataclass(slots=True)
class DebtView:
    debt: Debt
    principal_payment: Decimal  # monthly payment less escrow
    payoff: Payoff | None  # None when balance or APR is unknown
    payoff_date: date | None
    scenarios: list[Scenario] = field(default_factory=list)


@dataclass(slots=True)
class VehicleView:
    vehicle: Vehicle
    age: int | None
    payment: Decimal
    payoff_date: date | None


@dataclass(slots=True)
class GoalView:
    goal: Goal
    progress: Decimal  # 0..1
    remaining: Decimal
    months_left: int | None  # until target date
    required_monthly: Decimal | None
    projected_date: date | None  # at the planned contribution
    on_track: bool | None


@dataclass(slots=True)
class Suggestion:
    area: str
    title: str
    detail: str
    priority: str  # high, medium, low
    # A rough estimate of the money a month the idea frees up, when the profile
    # holds enough to say. Ideas that move money around (rather than freeing
    # it) leave this empty so the "potential savings" total stays honest.
    monthly_impact: Decimal | None = None


@dataclass(slots=True)
class ProjectionPoint:
    month: date
    saved: Decimal  # cumulative, from today
    freed: Decimal  # debt payments that have ended by this month


@dataclass(slots=True)
class Analysis:
    profile: Profile
    income: Decimal
    expenses: Decimal
    debt_payments: Decimal
    committed_savings: Decimal
    surplus: Decimal  # what is left unassigned each month
    savings_rate: Decimal
    by_category: list[tuple[str, Decimal]]
    needs: Decimal
    wants: Decimal
    savings_bucket: Decimal
    liquid: Decimal
    emergency_months: Decimal
    emergency_target: Decimal
    assets: Decimal
    liabilities: Decimal
    net_worth: Decimal
    debts: list[DebtView]
    vehicles: list[VehicleView]
    goals: list[GoalView]
    suggestions: list[Suggestion]
    projection: list[ProjectionPoint]
    unknown_balances: list[str]


def analyze(profile: Profile) -> Analysis:
    income = sum((item.monthly for item in profile.income), ZERO)
    expenses = sum((item.monthly for item in profile.expenses), ZERO)
    debt_payments = sum((debt.monthly_payment for debt in profile.debts), ZERO)
    committed = profile.monthly_savings + profile.monthly_retirement
    surplus = income - expenses - debt_payments - committed
    savings_rate = share(committed + max(surplus, ZERO), income)

    needs = sum((e.monthly for e in profile.expenses if e.essential), ZERO) + debt_payments
    wants = sum((e.monthly for e in profile.expenses if not e.essential), ZERO)

    liquid = sum((a.balance for a in profile.accounts if a.type in LIQUID_ACCOUNT_TYPES), ZERO)
    emergency_target = needs * profile.emergency_fund_months_target

    debts = [_debt_view(debt, profile.as_of) for debt in profile.debts]
    by_name = {view.debt.name: view for view in debts}
    vehicles = [_vehicle_view(v, by_name, profile.as_of) for v in profile.vehicles]

    assets = (
        sum((a.balance for a in profile.accounts), ZERO)
        + sum((a.value for a in profile.assets), ZERO)
        + sum((v.value for v in profile.vehicles if v.value is not None), ZERO)
    )
    liabilities = sum((d.balance for d in profile.debts if d.balance is not None), ZERO)

    analysis = Analysis(
        profile=profile,
        income=income,
        expenses=expenses,
        debt_payments=debt_payments,
        committed_savings=committed,
        surplus=surplus,
        savings_rate=savings_rate,
        by_category=_by_category(profile.expenses, profile.debts),
        needs=needs,
        wants=wants,
        savings_bucket=committed + max(surplus, ZERO),
        liquid=liquid,
        emergency_months=share(liquid, needs),
        emergency_target=emergency_target,
        assets=assets,
        liabilities=liabilities,
        net_worth=assets - liabilities,
        debts=debts,
        vehicles=vehicles,
        goals=[_goal_view(goal, profile.as_of) for goal in profile.goals],
        suggestions=[],
        projection=_projection(profile.as_of, committed, surplus, debts),
        unknown_balances=[d.name for d in profile.debts if d.balance is None or d.apr is None],
    )
    analysis.suggestions = suggest(analysis)
    return analysis


def _by_category(expenses: list[Expense], debts: list[Debt]) -> list[tuple[str, Decimal]]:
    totals: dict[str, Decimal] = {}
    for expense in expenses:
        totals[expense.category] = totals.get(expense.category, ZERO) + expense.monthly
    for debt in debts:
        category = _DEBT_CATEGORY.get(debt.kind, "debt")
        totals[category] = totals.get(category, ZERO) + debt.monthly_payment
    return sorted(totals.items(), key=lambda item: item[1], reverse=True)


def _debt_view(debt: Debt, today: date) -> DebtView:
    principal = max(debt.monthly_payment - debt.escrow, ZERO)
    if debt.balance is None or debt.apr is None:
        return DebtView(debt, principal, None, None)

    payoff = amortize(debt.balance, debt.apr, principal)
    view = DebtView(
        debt,
        principal,
        payoff,
        add_months(today, payoff.months) if payoff.months is not None else None,
    )
    if payoff.months is not None:
        for extra in EXTRA_PAYMENTS:
            faster = amortize(debt.balance, debt.apr, principal + extra)
            if faster.months is None:
                continue
            view.scenarios.append(
                Scenario(
                    extra=extra,
                    months=faster.months,
                    months_saved=payoff.months - faster.months,
                    interest_saved=payoff.interest - faster.interest,
                )
            )
    return view


def _vehicle_view(vehicle: Vehicle, debts: dict[str, DebtView], today: date) -> VehicleView:
    age = today.year - vehicle.year if vehicle.year is not None else None
    loan = debts.get(vehicle.loan) if vehicle.loan is not None else None
    return VehicleView(
        vehicle=vehicle,
        age=age,
        payment=loan.debt.monthly_payment if loan else ZERO,
        payoff_date=loan.payoff_date if loan else None,
    )


def _goal_view(goal: Goal, today: date) -> GoalView:
    remaining = max(goal.target - goal.current, ZERO)
    progress = min(share(goal.current, goal.target), Decimal(1))

    months_left = None
    required = None
    if goal.target_date is not None:
        months_left = max(months_between(today, goal.target_date), 0)
        required = money(remaining / months_left) if months_left > 0 else remaining

    projected = None
    if remaining == 0:
        projected = today
    elif goal.monthly_contribution:
        months = int((remaining / goal.monthly_contribution).to_integral_value("ROUND_CEILING"))
        projected = add_months(today, months)

    on_track = None
    if remaining == 0:
        on_track = True
    elif required is not None and goal.monthly_contribution is not None:
        on_track = goal.monthly_contribution >= required

    return GoalView(goal, progress, remaining, months_left, required, projected, on_track)


def _projection(
    today: date, committed: Decimal, surplus: Decimal, debts: list[DebtView]
) -> list[ProjectionPoint]:
    """Cash set aside month by month, if every finished loan's payment is saved too."""
    points = []
    saved = ZERO
    for i in range(1, PROJECTION_MONTHS + 1):
        freed = sum(
            (
                view.debt.monthly_payment
                for view in debts
                if view.payoff is not None
                and view.payoff.months is not None
                and view.payoff.months < i
            ),
            ZERO,
        )
        saved += committed + max(surplus, ZERO) + freed
        points.append(ProjectionPoint(add_months(today, i), saved, freed))
    return points


# --- suggestions ------------------------------------------------------------

Rule = Callable[[Analysis], list[Suggestion]]


def suggest(analysis: Analysis) -> list[Suggestion]:
    found: list[Suggestion] = []
    for rule in RULES:
        found.extend(rule(analysis))
    order = {"high": 0, "medium": 1, "low": 2}
    # Stable: within a priority, rules keep the order they are declared in.
    return sorted(found, key=lambda s: order[s.priority])


def _fmt(a: Analysis, value: Decimal) -> str:
    return f"{a.profile.currency_symbol}{money(value):,.0f}"


def _pct(value: Decimal) -> str:
    return f"{(value * 100).quantize(Decimal(1), rounding=ROUND_HALF_UP)}%"


def _category_total(analysis: Analysis, category: str) -> Decimal:
    return sum((e.monthly for e in analysis.profile.expenses if e.category == category), ZERO)


def rule_cash_flow(a: Analysis) -> list[Suggestion]:
    if a.income == 0:
        return [
            Suggestion(
                "Cash flow",
                "Add your income to the profile",
                "Without take-home pay the report cannot work out a savings rate or a budget.",
                "high",
            )
        ]
    if a.surplus < 0:
        return [
            Suggestion(
                "Cash flow",
                f"Close a monthly gap of {_fmt(a, -a.surplus)}",
                "Bills, loan payments and planned savings add up to more than take-home pay. "
                "Start with the wants in the 50/30/20 table, then the ideas below.",
                "high",
            )
        ]
    if a.surplus > 0:
        return [
            Suggestion(
                "Cash flow",
                f"Give the unassigned {_fmt(a, a.surplus)} a month a job",
                "Set an automatic transfer on payday into savings or a goal, so it is saved "
                "before it is spent.",
                "medium",
            )
        ]
    return []


def rule_savings_rate(a: Analysis) -> list[Suggestion]:
    if a.income == 0 or a.savings_rate >= TARGET_SAVINGS_RATE:
        return []
    gap = a.income * TARGET_SAVINGS_RATE - a.savings_bucket
    return [
        Suggestion(
            "Saving",
            f"Raise the savings rate from {_pct(a.savings_rate)} towards 20%",
            f"Another {_fmt(a, gap)} a month would get there. Raising it 1% each quarter is a "
            "pace most budgets absorb without noticing.",
            "high",
        )
    ]


def rule_emergency_fund(a: Analysis) -> list[Suggestion]:
    target_months = a.profile.emergency_fund_months_target
    if a.needs == 0 or a.emergency_months >= target_months:
        return []
    gap = a.emergency_target - a.liquid
    per_month = gap / 12
    return [
        Suggestion(
            "Safety net",
            f"Build the emergency fund to {target_months} months of needs",
            f"Cash covers {a.emergency_months.quantize(Decimal('0.1'))} months of essential "
            f"costs today. {_fmt(a, gap)} more closes the gap — about {_fmt(a, per_month)} a month "
            "for a year, in a high-yield savings account.",
            "high" if a.emergency_months < 3 else "medium",
        )
    ]


def rule_high_interest(a: Analysis) -> list[Suggestion]:
    costly = [v for v in a.debts if v.debt.apr is not None and v.debt.apr >= HIGH_INTEREST_APR]
    if not costly:
        return []
    costly.sort(key=lambda v: v.debt.apr or ZERO, reverse=True)
    names = ", ".join(f"{v.debt.name} ({v.debt.apr}%)" for v in costly)
    return [
        Suggestion(
            "Debt",
            "Pay down high-interest debt first (avalanche)",
            f"In this order: {names}. Pay minimums elsewhere and send every extra dollar to the "
            "top of the list; a 0% balance-transfer card can buy time on card debt.",
            "high",
        )
    ]


def rule_employer_match(a: Analysis) -> list[Suggestion]:
    if a.profile.employer_match_captured is not False:
        return []
    return [
        Suggestion(
            "Retirement",
            "Capture the full employer retirement match",
            "A match is an instant 50-100% return. Raise the contribution to at least the "
            "match before any extra debt payments.",
            "high",
        )
    ]


def rule_auto_loans(a: Analysis) -> list[Suggestion]:
    found = []
    for view in a.debts:
        debt = view.debt
        if debt.kind != "auto":
            continue
        if view.payoff_date is not None:
            found.append(
                Suggestion(
                    "Cars",
                    f"When {debt.name} is paid off, keep paying it — to yourself",
                    f"It ends around {view.payoff_date:%b %Y}. Moving the same "
                    f"{_fmt(a, debt.monthly_payment)} into a car-replacement fund means the next "
                    f"car is bought in cash: {_fmt(a, debt.monthly_payment * 12)} a year.",
                    "medium",
                )
            )
        if view.scenarios:
            best = view.scenarios[0]
            found.append(
                Suggestion(
                    "Cars",
                    f"Add {_fmt(a, best.extra)} a month to {debt.name}",
                    f"Clears it {best.months_saved} months sooner and saves "
                    f"{_fmt(a, best.interest_saved)} in interest.",
                    "low",
                )
            )
        if debt.apr is not None and debt.apr >= REFINANCE_WATCH_APR and debt.balance:
            found.append(
                Suggestion(
                    "Cars",
                    f"Get a refinance quote on {debt.name} ({debt.apr}% APR)",
                    "Credit unions often beat dealer financing. Only worth it with no fees "
                    "and without stretching the remaining term.",
                    "low",
                )
            )
        if view.debt.balance is None:
            found.append(
                Suggestion(
                    "Cars",
                    f"Add the balance and APR of {debt.name}",
                    "With them the report can show the payoff date and what extra payments "
                    "would save.",
                    "low",
                )
            )
    return found


def rule_vehicles(a: Analysis) -> list[Suggestion]:
    found = []
    for view in a.vehicles:
        vehicle = view.vehicle
        if not vehicle.paid_off or not vehicle.full_coverage:
            continue
        old = view.age is not None and view.age >= OLD_CAR_YEARS
        cheap = vehicle.value is not None and vehicle.value <= LOW_CAR_VALUE
        if old or cheap:
            found.append(
                Suggestion(
                    "Cars",
                    f"Review collision/comprehensive on {vehicle.name}",
                    "It is paid off, so no lender requires full coverage. If a year of that "
                    "coverage costs more than ~10% of the car's value, dropping it usually wins "
                    "— keep liability and the emergency fund as the backstop.",
                    "medium",
                )
            )
    if len(a.vehicles) >= 2:
        found.append(
            Suggestion(
                "Cars",
                f"Re-shop insurance for all {len(a.vehicles)} cars together",
                "Multi-car and home+auto bundles, a higher deductible and a usage-based "
                "discount are the usual levers; quotes take an hour once a year.",
                "medium",
            )
        )
    transport = dict(a.by_category).get("transportation", ZERO)
    if a.income and share(transport, a.income) > TRANSPORT_SHARE_WARNING:
        found.append(
            Suggestion(
                "Cars",
                f"Transportation is {_pct(share(transport, a.income))} of take-home pay",
                "Above ~15% is heavy. Keeping the paid-off car longer is the cheapest car "
                "there is; combine errands and compare fuel cards.",
                "medium",
            )
        )
    return found


def rule_housing(a: Analysis) -> list[Suggestion]:
    found = []
    housing = dict(a.by_category).get("housing", ZERO)
    if a.income and share(housing, a.income) > HOUSING_SHARE_WARNING:
        found.append(
            Suggestion(
                "Home",
                f"Housing takes {_pct(share(housing, a.income))} of take-home pay",
                "Over ~30% leaves little room elsewhere. Appeal the property-tax assessment, "
                "re-shop homeowner's insurance, and drop PMI once you reach 20% equity.",
                "medium",
            )
        )
    for view in a.debts:
        debt = view.debt
        if debt.kind != "mortgage" or view.payoff is None or view.payoff.months is None:
            continue
        if debt.apr is not None and debt.balance and debt.apr >= REFINANCE_WATCH_APR:
            lower = level_payment(debt.balance, debt.apr - 1, view.payoff.months)
            saving = view.principal_payment - lower
            if saving > 0:
                found.append(
                    Suggestion(
                        "Home",
                        f"Watch refinance rates on {debt.name} ({debt.apr}%)",
                        f"A rate one point lower over the same remaining term would cut the "
                        f"payment by about {_fmt(a, saving)} a month. Worth it once closing costs "
                        "are recovered within a few years.",
                        "medium",
                        saving,
                    )
                )
        if view.scenarios:
            best = view.scenarios[-1]
            found.append(
                Suggestion(
                    "Home",
                    f"An extra {_fmt(a, best.extra)} a month on {debt.name}",
                    f"Takes {best.months_saved // 12} years {best.months_saved % 12} months off "
                    f"the loan and saves {_fmt(a, best.interest_saved)} in interest. Do this only "
                    "after the emergency fund and any high-interest debt.",
                    "low",
                )
            )
    return found


def rule_subscriptions(a: Analysis) -> list[Suggestion]:
    subs = [e for e in a.profile.expenses if e.category == "subscriptions"]
    total = sum((e.monthly for e in subs), ZERO)
    if not subs or total < 30:
        return []
    return [
        Suggestion(
            "Everyday spending",
            f"Audit {len(subs)} subscriptions ({_fmt(a, total)} a month)",
            "Cancel what was not used last month and rotate streaming services instead of "
            "holding them all. Cutting a third is typical.",
            "medium",
            total / 3,
        )
    ]


def rule_dining(a: Analysis) -> list[Suggestion]:
    dining = _category_total(a, "dining")
    if not a.income or share(dining, a.income) <= DINING_SHARE_WARNING:
        return []
    return [
        Suggestion(
            "Everyday spending",
            f"Bring dining out ({_fmt(a, dining)} a month) down by a third",
            "Plan the week's dinners, batch-cook on Sunday and keep restaurants for set "
            "nights. Groceries for the same meals cost a fraction.",
            "medium",
            dining / 3,
        )
    ]


def rule_phone_internet(a: Analysis) -> list[Suggestion]:
    bill = _category_total(a, "phone_internet")
    if bill <= PHONE_INTERNET_WARNING:
        return []
    return [
        Suggestion(
            "Bills",
            f"Renegotiate phone and internet ({_fmt(a, bill)} a month)",
            "Call retention with a competitor's offer, or move phones to a prepaid/MVNO plan "
            "on the same network. Savings of a quarter are common.",
            "medium",
            bill / 4,
        )
    ]


def rule_insurance(a: Analysis) -> list[Suggestion]:
    premium = _category_total(a, "insurance")
    if premium == 0:
        return []
    return [
        Suggestion(
            "Bills",
            f"Compare insurance quotes ({_fmt(a, premium)} a month)",
            "Premiums creep up at renewal. Shopping home and auto every year typically "
            "saves around 10%.",
            "low",
            premium / 10,
        )
    ]


def rule_utilities(a: Analysis) -> list[Suggestion]:
    bill = _category_total(a, "utilities")
    if bill == 0:
        return []
    return [
        Suggestion(
            "Bills",
            "Trim utilities with a home energy check",
            "A smart thermostat, LED bulbs, sealing drafts and off-peak laundry each take a "
            "few percent off; many utilities offer the audit free.",
            "low",
            bill / 10,
        )
    ]


def rule_goals(a: Analysis) -> list[Suggestion]:
    found = []
    for view in a.goals:
        if view.on_track is False and view.required_monthly is not None:
            planned = view.goal.monthly_contribution or ZERO
            found.append(
                Suggestion(
                    "Goals",
                    f"{view.goal.name} is behind schedule",
                    f"It needs {_fmt(a, view.required_monthly)} a month to land on time; "
                    f"{_fmt(a, planned)} is planned. Raise it, move the date, or point the "
                    "savings from the ideas above at it.",
                    "medium",
                )
            )
        elif view.goal.monthly_contribution is None and view.remaining > 0:
            found.append(
                Suggestion(
                    "Goals",
                    f"Set a monthly amount for {view.goal.name}",
                    "A goal with an automatic monthly transfer gets funded; one without "
                    "tends to wait.",
                    "low",
                )
            )
    if not a.goals:
        found.append(
            Suggestion(
                "Goals",
                "Add a goal or two",
                "Emergency fund, next car, vacation, home projects — name it, price it, "
                "date it, and the report will track it.",
                "medium",
            )
        )
    return found


def rule_habits(a: Analysis) -> list[Suggestion]:
    return [
        Suggestion(
            "Habits",
            "Automate on payday",
            "Savings, retirement and goal transfers go out the day pay lands; bills on "
            "autopay. What is left is safe to spend.",
            "low",
        ),
        Suggestion(
            "Habits",
            "Use a 48-hour rule for non-essential buys over 100",
            "Put it on a list and wait two days. Most impulse purchases do not survive it.",
            "low",
        ),
        Suggestion(
            "Habits",
            "Review this report monthly",
            "Update the profile, regenerate, and check goals and the savings rate. A 20-minute "
            "money date once a month is enough.",
            "low",
        ),
    ]


RULES: Final[tuple[Rule, ...]] = (
    rule_cash_flow,
    rule_savings_rate,
    rule_emergency_fund,
    rule_high_interest,
    rule_employer_match,
    rule_auto_loans,
    rule_vehicles,
    rule_housing,
    rule_subscriptions,
    rule_dining,
    rule_phone_internet,
    rule_insurance,
    rule_utilities,
    rule_goals,
    rule_habits,
)
