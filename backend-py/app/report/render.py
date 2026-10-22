"""Renders an Analysis as one self-contained HTML page.

No external scripts, fonts or images: the file opens from disk, prints cleanly,
and follows the system's light or dark theme. Charts are inline SVG. The only
script remembers which action-plan items are ticked, in the browser's own
storage, so the page works the same without it.
"""

from __future__ import annotations

import json
from datetime import date
from decimal import ROUND_HALF_UP, Decimal
from html import escape
from typing import Final

from app.report.analysis import (
    TARGET_SAVINGS_RATE,
    ZERO,
    Analysis,
    DebtView,
    GoalView,
    Suggestion,
    share,
)

_CATEGORY_LABELS: Final = {
    "phone_internet": "Phone & internet",
    "debt": "Other debt payments",
}

_PRIORITY_LABELS: Final = {"high": "Do first", "medium": "Next", "low": "When you can"}
_PLAN_HORIZONS: Final = {"high": "This month", "medium": "Next 3 months", "low": "This year"}


class _Fmt:
    def __init__(self, symbol: str) -> None:
        self._symbol = symbol

    def money(self, value: Decimal, *, cents: bool = False) -> str:
        sign = "-" if value < 0 else ""
        places = Decimal("0.01") if cents else Decimal(1)
        amount = abs(value).quantize(places, rounding=ROUND_HALF_UP)
        return f"{sign}{self._symbol}{amount:,}"


def pct(value: Decimal) -> str:
    return f"{(value * 100).quantize(Decimal(1), rounding=ROUND_HALF_UP)}%"


def _label(category: str) -> str:
    return _CATEGORY_LABELS.get(category, category.replace("_", " ").capitalize())


def _month(day: date | None) -> str:
    return f"{day:%b %Y}" if day else "—"


def potential_savings(analysis: Analysis) -> Decimal:
    return sum(
        (s.monthly_impact for s in analysis.suggestions if s.monthly_impact is not None), ZERO
    )


def render(analysis: Analysis) -> str:
    fmt = _Fmt(analysis.profile.currency_symbol)
    p = analysis.profile
    sections = [
        _summary(analysis, fmt),
        _cash_flow(analysis, fmt),
        _spending(analysis, fmt),
        _ideas(analysis, fmt),
        _debts(analysis, fmt),
        _cars(analysis, fmt),
        _safety_net(analysis, fmt),
        _goals(analysis, fmt),
        _projection(analysis, fmt),
        _plan(analysis),
        _net_worth(analysis, fmt),
        _notes(analysis),
    ]
    nav = "".join(
        f'<a href="#{anchor}">{escape(title)}</a>'
        for anchor, title in (
            ("summary", "Summary"),
            ("cash-flow", "Cash flow"),
            ("spending", "Spending"),
            ("ideas", "Savings ideas"),
            ("debts", "Debts"),
            ("cars", "Cars"),
            ("safety-net", "Emergency fund"),
            ("goals", "Goals"),
            ("projection", "Projection"),
            ("plan", "Action plan"),
            ("net-worth", "Net worth"),
        )
    )
    return _PAGE.format(
        title=escape(f"{p.household} — Finance Report"),
        household=escape(p.household),
        as_of=f"{p.as_of:%B %-d, %Y}",
        nav=nav,
        body="\n".join(sections),
        # json.dumps, not escape(): this lands inside a <script>, where entities stay literal.
        storage_key=json.dumps(f"finance-report-plan:{p.household}").replace("<", "\\u003c"),
    )


def _section(anchor: str, title: str, lede: str, body: str) -> str:
    return (
        f'<section id="{anchor}"><h2>{escape(title)}</h2><p class="lede">{lede}</p>{body}</section>'
    )


def _tile(label: str, value: str, note: str = "", tone: str = "") -> str:
    return (
        f'<div class="tile {tone}"><div class="tile-label">{escape(label)}</div>'
        f'<div class="tile-value">{value}</div><div class="tile-note">{note}</div></div>'
    )


def _bar(fraction: Decimal, tone: str = "") -> str:
    width = max(min(fraction, Decimal(1)), ZERO) * 100
    return f'<div class="bar"><span class="{tone}" style="width:{width:.1f}%"></span></div>'


def _summary(a: Analysis, fmt: _Fmt) -> str:
    rate_tone = "good" if a.savings_rate >= TARGET_SAVINGS_RATE else "warn"
    target = a.profile.emergency_fund_months_target
    ef_tone = "good" if a.emergency_months >= target else "warn"
    surplus_tone = "bad" if a.surplus < 0 else ""
    tiles = "".join(
        [
            _tile("Take-home pay", fmt.money(a.income), "per month"),
            _tile(
                "Spending",
                fmt.money(a.expenses + a.debt_payments),
                f"bills {fmt.money(a.expenses)} · loans {fmt.money(a.debt_payments)}",
            ),
            _tile("Savings rate", pct(a.savings_rate), "target 20%+", rate_tone),
            _tile(
                "Unassigned",
                fmt.money(a.surplus),
                "left after bills, loans and savings",
                surplus_tone,
            ),
            _tile(
                "Emergency fund",
                f"{a.emergency_months.quantize(Decimal('0.1'))} mo",
                f"target {target} months of needs",
                ef_tone,
            ),
            _tile("Net worth", fmt.money(a.net_worth), "what you own − what you owe"),
            _tile(
                "Potential savings",
                fmt.money(potential_savings(a)),
                "a month, from the ideas below",
                "good",
            ),
        ]
    )
    return _section(
        "summary",
        "Summary",
        "Where things stand this month, at a glance.",
        f'<div class="tiles">{tiles}</div>',
    )


def _cash_flow(a: Analysis, fmt: _Fmt) -> str:
    rows = "".join(
        f"<tr><td>{escape(i.name)}</td><td class='num'>{fmt.money(i.monthly)}</td></tr>"
        for i in a.profile.income
    )
    out = [
        ("Bills and living costs", a.expenses),
        ("Loan payments", a.debt_payments),
        ("Savings", a.profile.monthly_savings),
        ("Retirement", a.profile.monthly_retirement),
    ]
    rows += "".join(
        f"<tr><td>− {label}</td><td class='num'>{fmt.money(value)}</td></tr>"
        for label, value in out
    )
    tone = "bad" if a.surplus < 0 else "good"
    rows += (
        f"<tr class='total'><td>= Unassigned</td>"
        f"<td class='num {tone}-text'>{fmt.money(a.surplus)}</td></tr>"
    )

    split = [
        ("Needs", a.needs, Decimal("0.50"), "c1"),
        ("Wants", a.wants, Decimal("0.30"), "c2"),
        ("Savings & extra", a.savings_bucket, Decimal("0.20"), "c3"),
    ]
    stack = "".join(
        f'<span class="{tone}" style="width:{share(value, a.income) * 100:.1f}%" '
        f'title="{label}"></span>'
        for label, value, _, tone in split
        if a.income > 0
    )
    split_rows = "".join(
        f"<tr><td><i class='dot {tone}'></i>{label}</td>"
        f"<td class='num'>{fmt.money(value)}</td>"
        f"<td class='num'>{pct(share(value, a.income))}</td>"
        f"<td class='num muted'>{pct(target)}</td></tr>"
        for label, value, target, tone in split
    )
    return _section(
        "cash-flow",
        "Monthly cash flow",
        "Take-home pay in, everything else out. The 50/30/20 check compares the split "
        "with the common rule of thumb: half on needs, under a third on wants, a fifth saved.",
        f'<div class="grid2"><table><tbody>{rows}</tbody></table>'
        f'<div><div class="stack">{stack}</div>'
        f"<table><thead><tr><th>50/30/20</th><th class='num'>Monthly</th>"
        f"<th class='num'>Actual</th><th class='num'>Target</th></tr></thead>"
        f"<tbody>{split_rows}</tbody></table></div></div>",
    )


def _spending(a: Analysis, fmt: _Fmt) -> str:
    total = sum((value for _, value in a.by_category), ZERO)
    biggest = a.by_category[0][1] if a.by_category else ZERO
    rows = "".join(
        f"<div class='hrow'><div>{escape(_label(cat))}</div>"
        f"{_bar(share(value, biggest))}"
        f"<div class='num'>{fmt.money(value)}</div>"
        f"<div class='num muted'>{pct(share(value, a.income))}</div></div>"
        for cat, value in a.by_category
    )
    detail = "".join(
        f"<tr><td>{escape(e.name)}</td><td>{escape(_label(e.category))}</td>"
        f"<td>{'Need' if e.essential else 'Want'}</td>"
        f"<td class='num'>{fmt.money(e.monthly)}</td></tr>"
        for e in sorted(a.profile.expenses, key=lambda e: e.monthly, reverse=True)
    )
    return _section(
        "spending",
        "Where the money goes",
        f"{fmt.money(total)} a month across {len(a.by_category)} categories, loan payments "
        "included. The right-hand column is the share of take-home pay.",
        f'<div class="hbars">{rows}</div>'
        f"<details><summary>Every line item</summary><table><thead><tr><th>Item</th>"
        f"<th>Category</th><th>Type</th><th class='num'>Monthly</th></tr></thead>"
        f"<tbody>{detail}</tbody></table></details>",
    )


def _idea(s: Suggestion, fmt: _Fmt) -> str:
    impact = (
        f"<div class='impact'>≈ {fmt.money(s.monthly_impact)}/mo</div>" if s.monthly_impact else ""
    )
    return (
        f"<article class='idea {s.priority}'><div class='idea-head'>"
        f"<span class='chip'>{escape(s.area)}</span>{impact}</div>"
        f"<h3>{escape(s.title)}</h3><p>{escape(s.detail)}</p></article>"
    )


def _ideas(a: Analysis, fmt: _Fmt) -> str:
    groups = []
    for priority in ("high", "medium", "low"):
        items = [s for s in a.suggestions if s.priority == priority]
        if items:
            cards = "".join(_idea(s, fmt) for s in items)
            groups.append(
                f"<h3 class='group'>{_PRIORITY_LABELS[priority]}</h3>"
                f"<div class='ideas'>{cards}</div>"
            )
    return _section(
        "ideas",
        "Ways to save",
        f"{len(a.suggestions)} ideas worked out from your numbers, most important first. "
        f"The estimates add up to about {fmt.money(potential_savings(a))} a month "
        f"({fmt.money(potential_savings(a) * 12)} a year).",
        "".join(groups),
    )


def _debt_row(view: DebtView, fmt: _Fmt) -> str:
    d = view.debt
    if view.payoff is None:
        timeline = "<td colspan='3' class='muted'>Add balance and APR to see payoff</td>"
    elif view.payoff.months is None:
        timeline = "<td colspan='3' class='bad-text'>Payment does not cover the interest</td>"
    else:
        years, months = divmod(view.payoff.months, 12)
        timeline = (
            f"<td>{_month(view.payoff_date)}</td><td class='num'>{years}y {months}m</td>"
            f"<td class='num'>{fmt.money(view.payoff.interest)}</td>"
        )
    return (
        f"<tr><td>{escape(d.name)}</td><td>{escape(_label(d.kind))}</td>"
        f"<td class='num'>{fmt.money(d.balance) if d.balance is not None else '—'}</td>"
        f"<td class='num'>{f'{d.apr}%' if d.apr is not None else '—'}</td>"
        f"<td class='num'>{fmt.money(d.monthly_payment)}</td>{timeline}</tr>"
    )


def _debts(a: Analysis, fmt: _Fmt) -> str:
    if not a.debts:
        return _section("debts", "Debts", "No debts in the profile. Nicely done.", "")
    rows = "".join(_debt_row(view, fmt) for view in a.debts)
    scenarios = "".join(
        f"<tr><td>{escape(v.debt.name)}</td><td class='num'>+{fmt.money(s.extra)}</td>"
        f"<td class='num'>{s.months_saved // 12}y {s.months_saved % 12}m sooner</td>"
        f"<td class='num good-text'>{fmt.money(s.interest_saved)}</td></tr>"
        for v in a.debts
        for s in v.scenarios
    )
    extra = (
        f"<h3>What extra principal does</h3><table><thead><tr><th>Loan</th>"
        f"<th class='num'>Extra / month</th><th class='num'>Paid off</th>"
        f"<th class='num'>Interest saved</th></tr></thead><tbody>{scenarios}</tbody></table>"
        if scenarios
        else ""
    )
    escrow = (
        "<p class='muted small'>Payments that include escrow (property tax and insurance) "
        "only count the loan part towards payoff.</p>"
        if any(v.debt.escrow for v in a.debts)
        else ""
    )
    return _section(
        "debts",
        "Debts and payoff",
        f"{fmt.money(a.debt_payments)} a month to {len(a.debts)} lenders; "
        f"{fmt.money(a.liabilities)} owed in all.",
        f"<table><thead><tr><th>Loan</th><th>Kind</th><th class='num'>Balance</th>"
        f"<th class='num'>APR</th><th class='num'>Payment</th><th>Paid off</th>"
        f"<th class='num'>Time left</th><th class='num'>Interest left</th></tr></thead>"
        f"<tbody>{rows}</tbody></table>{escrow}{extra}",
    )


def _cars(a: Analysis, fmt: _Fmt) -> str:
    if not a.vehicles:
        return _section("cars", "Cars", "No vehicles in the profile.", "")
    cards = []
    for view in a.vehicles:
        v = view.vehicle
        status = (
            "<span class='chip good'>Paid off</span>"
            if v.paid_off
            else f"<span class='chip'>{fmt.money(view.payment)}/mo until "
            f"{_month(view.payoff_date)}</span>"
        )
        facts = [
            f"Year {v.year} · {view.age} years old" if v.year else "",
            f"Worth about {fmt.money(v.value)}" if v.value is not None else "",
            "Full coverage" if v.full_coverage else "Liability only",
        ]
        cards.append(
            f"<article class='car'><h3>{escape(v.name)}</h3>{status}"
            f"<p>{' · '.join(escape(f) for f in facts if f)}</p></article>"
        )
    paying = sum((v.payment for v in a.vehicles), ZERO)
    return _section(
        "cars",
        "Cars",
        f"{len(a.vehicles)} cars, {sum(1 for v in a.vehicles if v.vehicle.paid_off)} paid off; "
        f"{fmt.money(paying)} a month in car payments.",
        f"<div class='cars'>{''.join(cards)}</div>",
    )


def _safety_net(a: Analysis, fmt: _Fmt) -> str:
    target = a.profile.emergency_fund_months_target
    fraction = share(a.liquid, a.emergency_target)
    tone = "good" if fraction >= 1 else "warn"
    return _section(
        "safety-net",
        "Emergency fund",
        f"Cash in checking, savings and on hand against {target} months of needs "
        f"({fmt.money(a.needs)} a month, loan payments included).",
        f"<div class='progress-head'><strong>{fmt.money(a.liquid)}</strong>"
        f"<span>of {fmt.money(a.emergency_target)} · {pct(min(fraction, Decimal(1)))}</span></div>"
        f"{_bar(fraction, tone)}",
    )


def _goal_card(view: GoalView, fmt: _Fmt) -> str:
    g = view.goal
    if view.on_track is True:
        badge = "<span class='chip good'>On track</span>"
    elif view.on_track is False:
        badge = "<span class='chip bad'>Behind</span>"
    else:
        badge = "<span class='chip'>No plan yet</span>"
    lines = [
        f"Target {fmt.money(g.target)}" + (f" by {_month(g.target_date)}" if g.target_date else "")
    ]
    if view.required_monthly is not None and view.remaining > 0:
        lines.append(f"Needs {fmt.money(view.required_monthly)}/mo")
    if g.monthly_contribution is not None:
        lines.append(f"Planned {fmt.money(g.monthly_contribution)}/mo")
    if view.projected_date is not None and view.remaining > 0:
        lines.append(f"At that pace: {_month(view.projected_date)}")
    tone = "good" if view.on_track else ("bad" if view.on_track is False else "")
    return (
        f"<article class='goal'><div class='idea-head'><h3>{escape(g.name)}</h3>{badge}</div>"
        f"<div class='progress-head'><strong>{fmt.money(g.current)}</strong>"
        f"<span>{pct(view.progress)}</span></div>{_bar(view.progress, tone)}"
        f"<p class='muted small'>{' · '.join(lines)}</p></article>"
    )


def _goals(a: Analysis, fmt: _Fmt) -> str:
    cards = "".join(_goal_card(view, fmt) for view in a.goals)
    total = sum((v.goal.monthly_contribution or ZERO for v in a.goals), ZERO)
    return _section(
        "goals",
        "Goals",
        f"{len(a.goals)} goals, {fmt.money(total)} a month planned towards them."
        if a.goals
        else "No goals yet — add some to the profile.",
        f"<div class='goals'>{cards}</div>",
    )


def _projection(a: Analysis, fmt: _Fmt) -> str:
    points = a.projection
    if not points:
        return ""
    top = max((p.saved for p in points), default=ZERO) or Decimal(1)
    width, height, pad = 720, 220, 28
    step = (width - pad) / len(points)
    bars = []
    for i, point in enumerate(points):
        h = float(point.saved / top) * (height - pad * 2)
        x = pad + i * step + 2
        y = height - pad - h
        tone = "c3" if point.freed > 0 else "c1"
        bars.append(
            f"<rect class='{tone}' x='{x:.1f}' y='{y:.1f}' width='{step - 4:.1f}' "
            f"height='{h:.1f}' rx='2'><title>{_month(point.month)}: "
            f"{fmt.money(point.saved)}</title></rect>"
        )
        if i % 3 == 0 or i == len(points) - 1:
            bars.append(
                f"<text x='{x + (step - 4) / 2:.1f}' y='{height - 8}' "
                f"text-anchor='middle'>{point.month:%b %y}</text>"
            )
    bars.append(
        f"<text x='{pad}' y='14' class='strong'>{fmt.money(top)}</text>"
        f"<line x1='{pad}' x2='{width}' y1='{pad}' y2='{pad}' class='grid'/>"
        f"<line x1='{pad}' x2='{width}' y1='{height - pad}' y2='{height - pad}' class='axis'/>"
    )
    freed = [v for v in a.debts if v.payoff_date and v.payoff_date <= points[-1].month]
    legend = (
        "<p class='muted small'><i class='dot c3'></i> Months after "
        + ", ".join(f"{escape(v.debt.name)} ends ({_month(v.payoff_date)})" for v in freed)
        + " — its payment is saved instead.</p>"
        if freed
        else ""
    )
    final = points[-1]
    return _section(
        "projection",
        f"Next {len(points)} months",
        f"If savings, retirement and the unassigned amount are all set aside — and every loan "
        f"that ends keeps its payment going to savings — about "
        f"<strong>{fmt.money(final.saved)}</strong> is put away by {_month(final.month)}.",
        f"<svg class='chart' viewBox='0 0 {width} {height}' role='img' "
        f"aria-label='Cumulative savings by month'>{''.join(bars)}</svg>{legend}",
    )


def _plan(a: Analysis) -> str:
    columns = []
    for priority, horizon in _PLAN_HORIZONS.items():
        items = [s for s in a.suggestions if s.priority == priority]
        if not items:
            continue
        lis = "".join(
            f"<li><label><input type='checkbox' data-key='{escape(s.title)}'> "
            f"{escape(s.title)}</label></li>"
            for s in items
        )
        columns.append(f"<div class='plan-col'><h3>{horizon}</h3><ul>{lis}</ul></div>")
    return _section(
        "plan",
        "Action plan",
        "The ideas above as a checklist. Ticks are remembered in this browser.",
        f"<div class='plan'>{''.join(columns)}</div>",
    )


def _net_worth(a: Analysis, fmt: _Fmt) -> str:
    rows = "".join(
        f"<tr><td>{escape(acc.name)}</td><td>{escape(_label(acc.type))}</td>"
        f"<td class='num'>{fmt.money(acc.balance)}</td></tr>"
        for acc in a.profile.accounts
    )
    rows += "".join(
        f"<tr><td>{escape(asset.name)}</td><td>Property</td>"
        f"<td class='num'>{fmt.money(asset.value)}</td></tr>"
        for asset in a.profile.assets
    )
    rows += "".join(
        f"<tr><td>{escape(v.name)}</td><td>Vehicle</td>"
        f"<td class='num'>{fmt.money(v.value)}</td></tr>"
        for v in a.profile.vehicles
        if v.value is not None
    )
    rows += "".join(
        f"<tr><td>{escape(d.name)}</td><td>Debt</td>"
        f"<td class='num bad-text'>−{fmt.money(d.balance)}</td></tr>"
        for d in a.profile.debts
        if d.balance is not None
    )
    rows += (
        f"<tr class='total'><td colspan='2'>Net worth</td>"
        f"<td class='num'>{fmt.money(a.net_worth)}</td></tr>"
    )
    return _section(
        "net-worth",
        "Net worth",
        f"{fmt.money(a.assets)} in assets, {fmt.money(a.liabilities)} owed.",
        f"<table><tbody>{rows}</tbody></table>",
    )


def _notes(a: Analysis) -> str:
    notes = [escape(n) for n in a.profile.notes]
    if a.unknown_balances:
        notes.append(
            "No balance or APR yet for: "
            + ", ".join(escape(n) for n in a.unknown_balances)
            + ". Their payments are counted; payoff dates are not."
        )
    notes.append(
        "Estimates use common rules of thumb, not personal advice. Update the profile and "
        "regenerate whenever something changes — every number here is recomputed."
    )
    items = "".join(f"<li>{n}</li>" for n in notes)
    return _section("notes", "Notes and assumptions", "", f"<ul class='notes'>{items}</ul>")


_PAGE: Final = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
:root {{
  --bg: #f6f7f9; --panel: #ffffff; --text: #1b1f24; --muted: #5f6b7a; --line: #e3e7ec;
  --accent: #2563eb; --good: #15803d; --warn: #b45309; --bad: #b91c1c;
  --c1: #2563eb; --c2: #d97706; --c3: #16a34a; --track: #eef1f4;
  --high: #b91c1c; --medium: #d97706; --low: #64748b;
}}
@media (prefers-color-scheme: dark) {{
  :root {{
    --bg: #0f1216; --panel: #171b21; --text: #e6e9ee; --muted: #94a0b0; --line: #2a313a;
    --accent: #60a5fa; --good: #4ade80; --warn: #fbbf24; --bad: #f87171;
    --c1: #60a5fa; --c2: #fbbf24; --c3: #4ade80; --track: #232a33;
    --high: #f87171; --medium: #fbbf24; --low: #94a3b8;
  }}
}}
* {{ box-sizing: border-box; }}
body {{ margin: 0; background: var(--bg); color: var(--text);
  font: 15px/1.5 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; }}
header {{ padding: 32px 16px 8px; max-width: 1100px; margin: 0 auto; }}
header h1 {{ margin: 0; font-size: 28px; }}
header p {{ margin: 4px 0 0; color: var(--muted); }}
nav {{ position: sticky; top: 0; z-index: 1; background: var(--bg);
  border-bottom: 1px solid var(--line); overflow-x: auto; white-space: nowrap; }}
nav div {{ max-width: 1100px; margin: 0 auto; padding: 8px 16px; }}
nav a {{ color: var(--muted); text-decoration: none; margin-right: 16px; font-size: 14px; }}
nav a:hover {{ color: var(--accent); }}
main {{ max-width: 1100px; margin: 0 auto; padding: 0 16px 48px; }}
section {{ background: var(--panel); border: 1px solid var(--line); border-radius: 12px;
  padding: 20px; margin-top: 20px; scroll-margin-top: 48px; }}
h2 {{ margin: 0 0 4px; font-size: 20px; }}
h3 {{ font-size: 15px; margin: 0 0 6px; }}
h3.group {{ margin: 18px 0 10px; color: var(--muted); text-transform: uppercase;
  font-size: 12px; letter-spacing: .06em; }}
.lede {{ margin: 0 0 16px; color: var(--muted); }}
.muted {{ color: var(--muted); }} .small {{ font-size: 13px; }}
.good-text {{ color: var(--good); }} .bad-text {{ color: var(--bad); }}
table {{ width: 100%; border-collapse: collapse; font-size: 14px; }}
th, td {{ text-align: left; padding: 7px 8px; border-bottom: 1px solid var(--line); }}
th {{ color: var(--muted); font-weight: 600; font-size: 12px; }}
.num {{ text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }}
tr.total td {{ font-weight: 700; border-bottom: none; }}
.tiles {{ display: grid; gap: 12px; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); }}
.tile {{ border: 1px solid var(--line); border-radius: 10px; padding: 12px; }}
.tile-label {{ color: var(--muted); font-size: 12px; }}
.tile-value {{ font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; }}
.tile-note {{ color: var(--muted); font-size: 12px; }}
.tile.good .tile-value {{ color: var(--good); }} .tile.warn .tile-value {{ color: var(--warn); }}
.tile.bad .tile-value {{ color: var(--bad); }}
.grid2 {{ display: grid; gap: 24px; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); }}
.bar {{ height: 8px; background: var(--track); border-radius: 99px; overflow: hidden; }}
.bar span {{ display: block; height: 100%; background: var(--accent); border-radius: 99px; }}
.bar span.good {{ background: var(--good); }} .bar span.warn {{ background: var(--warn); }}
.bar span.bad {{ background: var(--bad); }}
.stack {{ display: flex; height: 14px; border-radius: 99px; overflow: hidden;
  background: var(--track); margin-bottom: 12px; }}
.stack .c1, .dot.c1 {{ background: var(--c1); }} .stack .c2, .dot.c2 {{ background: var(--c2); }}
.stack .c3, .dot.c3 {{ background: var(--c3); }}
.dot {{ display: inline-block; width: 9px; height: 9px; border-radius: 50%; margin-right: 6px; }}
.hbars {{ display: grid; gap: 8px; }}
.hrow {{ display: grid; grid-template-columns: minmax(110px, 180px) 1fr 90px 50px; gap: 10px;
  align-items: center; font-size: 14px; }}
details {{ margin-top: 16px; }} summary {{ cursor: pointer; color: var(--accent); }}
.ideas, .goals, .cars {{ display: grid; gap: 12px;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); }}
.idea, .goal, .car {{ border: 1px solid var(--line); border-radius: 10px; padding: 14px; }}
.idea {{ border-left: 4px solid var(--low); }}
.idea.high {{ border-left-color: var(--high); }}
.idea.medium {{ border-left-color: var(--medium); }}
.idea p, .car p {{ margin: 0; color: var(--muted); font-size: 14px; }}
.idea-head {{ display: flex; justify-content: space-between; align-items: center; gap: 8px;
  margin-bottom: 6px; }}
.impact {{ color: var(--good); font-weight: 700; font-size: 13px; white-space: nowrap; }}
.chip {{ display: inline-block; font-size: 12px; padding: 1px 8px; border-radius: 99px;
  background: var(--track); color: var(--muted); }}
.chip.good {{ color: var(--good); }} .chip.bad {{ color: var(--bad); }}
.car .chip {{ margin-bottom: 8px; }}
.progress-head {{ display: flex; justify-content: space-between; margin: 6px 0;
  font-variant-numeric: tabular-nums; }}
.progress-head span {{ color: var(--muted); font-size: 13px; }}
.chart {{ width: 100%; height: auto; }}
.chart text {{ fill: var(--muted); font-size: 11px; }} .chart text.strong {{ fill: var(--text); }}
.chart rect.c1 {{ fill: var(--c1); }} .chart rect.c3 {{ fill: var(--c3); }}
.chart line.grid {{ stroke: var(--line); stroke-dasharray: 4 4; }}
.chart line.axis {{ stroke: var(--line); }}
.plan {{ display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }}
.plan ul {{ list-style: none; padding: 0; margin: 0; }}
.plan li {{ padding: 6px 0; border-bottom: 1px solid var(--line); font-size: 14px; }}
.plan input:checked + * , .plan label:has(input:checked) {{ color: var(--muted);
  text-decoration: line-through; }}
.notes {{ margin: 0; padding-left: 18px; color: var(--muted); }}
footer {{ text-align: center; color: var(--muted); font-size: 12px; padding: 0 16px 32px; }}
@media (max-width: 560px) {{
  .hrow {{ grid-template-columns: 1fr 80px; }}
  .hrow .bar, .hrow .muted {{ display: none; }}
  section {{ padding: 16px; }}
  table {{ display: block; overflow-x: auto; }}
}}
@media print {{
  nav {{ display: none; }} body {{ background: #fff; }}
  section {{ break-inside: avoid; border-color: #ccc; }}
}}
</style>
</head>
<body>
<header><h1>{household}</h1><p>Finance report · as of {as_of}</p></header>
<nav><div>{nav}</div></nav>
<main>
{body}
</main>
<footer>Generated from the household profile. Edit the profile and run the report again to
refresh every section.</footer>
<script>
(function () {{
  var key = {storage_key}, state = {{}};
  try {{ state = JSON.parse(localStorage.getItem(key) || "{{}}"); }} catch (e) {{}}
  document.querySelectorAll(".plan input[type=checkbox]").forEach(function (box) {{
    box.checked = !!state[box.dataset.key];
    box.addEventListener("change", function () {{
      state[box.dataset.key] = box.checked;
      try {{ localStorage.setItem(key, JSON.stringify(state)); }} catch (e) {{}}
    }});
  }});
}})();
</script>
</body>
</html>
"""
