# Household finance report

A one-page HTML report built from a JSON profile of your finances: cash flow,
where the money goes, a 50/30/20 check, debts and payoff dates, cars, the
emergency fund, goals, a 24-month savings projection, an action-plan checklist,
net worth, and savings ideas worked out from your own numbers.

Nothing is hard-coded about one household. When something changes (a loan is
paid off, a bill goes up, a third car, a new goal), edit the profile and run it
again. Every section is recomputed.

## Run it

```bash
cd backend-py
cp reports/profile.example.json reports/profile.json   # then put your numbers in
python -m app.report reports/profile.json               # writes reports/profile.html
python -m app.report reports/profile.json -o ~/Desktop/finances.html
```

It uses the standard library only, so any Python 3.12+ will do. `profile.json`
and the generated HTML are git-ignored so your real numbers stay on your machine.

## The profile

| Field | What goes in it |
|---|---|
| `household`, `currency`, `as_of` | Title, currency code (`USD`, `EUR`, `BRL`...), and the report date (defaults to today) |
| `income[]` | `name`, `monthly` take-home pay |
| `expenses[]` | `name`, `category`, `monthly`, optional `essential` (need vs. want). Do **not** list loan payments here |
| `debts[]` | `name`, `kind` (`mortgage`, `auto`, `credit_card`, `student`, `personal`, `other`), `monthly_payment`, and optionally `balance`, `apr` (percent) and `escrow` (the tax/insurance part of a mortgage payment) |
| `vehicles[]` | `name`, `year`, `value`, `full_coverage`, and `loan`: the name of its debt. No `loan` means paid off |
| `accounts[]` | `name`, `type` (`checking`, `savings`, `cash`, `investment`, `retirement`), `balance` |
| `assets[]` | Things you own outside accounts: `name`, `value` (the house, for example) |
| `goals[]` | `name`, `target`, `current`, optional `target_date` (`YYYY-MM`) and `monthly_contribution` |
| `monthly_savings`, `monthly_retirement` | What already goes to savings and retirement each month |
| `employer_match_captured` | `true`/`false`. If `false`, the report flags it |
| `emergency_fund_months_target` | Months of needs to keep in cash (default 6) |
| `notes[]` | Free text shown at the end of the report |

Categories with tailored ideas: `housing`, `utilities`, `transportation`,
`insurance`, `groceries`, `dining`, `subscriptions`, `phone_internet`, `health`,
`childcare`, `education`, `shopping`, `entertainment`, `travel`, `personal`,
`gifts`, `pets`, `other`.

The ideas come from widely used rules of thumb (20% savings rate, housing under
~30% of take-home pay, 3–6 months of emergency cash, avalanche order for debt).
The report shows how it got each number. It is not personal financial advice.
