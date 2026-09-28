# SonarQube quality gate conditions for C# and React

A **quality gate** is a set of conditions on project metrics. If any condition fails, the scan fails. In CI that means a red job and a blocked merge.

Gate conditions are **not language-specific**. The same metrics are available for C# and for React (TypeScript/JavaScript). What differs per language is the **rules** that produce the issues those metrics count (see [Rules per language](#rules-per-language)).

*Source: SonarQube Community Build 26.8, `api/metrics/search`. 126 metrics are usable as gate conditions.*

## Overall code vs. new code

Every condition exists in two versions:

- **Overall code**, e.g. `coverage`: measured on the whole project.
- **New code**, e.g. `new_coverage`: measured only on code changed since a baseline.

Our pipeline starts a **fresh SonarQube inside every CI run**, so there is never an earlier scan to compare against. New-code conditions never trigger there, so **use the overall-code versions**.

✅ = used by our `ci-gate` today.

---

## Issues

| Condition | Metric key |
|---|---|
| Blocker severity issues | `software_quality_blocker_issues` |
| High severity issues | `software_quality_high_issues` |
| Medium severity issues | `software_quality_medium_issues` |
| Low severity issues | `software_quality_low_issues` |
| Info severity issues | `software_quality_info_issues` |
| All issues | `violations` |
| Open issues | `open_issues` |
| Confirmed issues | `confirmed_issues` |
| Reopened issues | `reopened_issues` |
| Accepted issues | `accepted_issues` |
| Blocker and High severity accepted issues | `high_impact_accepted_issues` |
| Issues from prioritized rules | `prioritized_rule_issues` |
| Legacy severities (Blocker / Critical / Major / Minor / Info) | `blocker_violations`, `critical_violations`, `major_violations`, `minor_violations`, `info_violations` |

## Reliability (bugs)

| Condition | Metric key |
|---|---|
| ✅ Reliability issues | `software_quality_reliability_issues` (legacy: `bugs`) |
| ✅ Reliability rating (A–E) | `software_quality_reliability_rating` (legacy: `reliability_rating`) |
| Reliability remediation effort | `software_quality_reliability_remediation_effort` |

## Security

| Condition | Metric key |
|---|---|
| ✅ Security issues | `software_quality_security_issues` (legacy: `vulnerabilities`) |
| ✅ Security rating (A–E) | `software_quality_security_rating` (legacy: `security_rating`) |
| Security remediation effort | `software_quality_security_remediation_effort` |
| Security hotspots | `security_hotspots` |
| Security hotspots reviewed (%) | `security_hotspots_reviewed` |
| Security review rating (A–E) | `security_review_rating` |

## Maintainability (code smells)

| Condition | Metric key |
|---|---|
| Maintainability issues | `software_quality_maintainability_issues` (legacy: `code_smells`) |
| Maintainability rating (A–E) | `software_quality_maintainability_rating` (legacy: `sqale_rating`) |
| Technical debt (time) | `software_quality_maintainability_remediation_effort` (legacy: `sqale_index`) |
| Technical debt ratio (%) | `software_quality_maintainability_debt_ratio` (legacy: `sqale_debt_ratio`) |
| Effort to reach maintainability rating A | `effort_to_reach_software_quality_maintainability_rating_a` |

## Coverage and tests

| Condition | Metric key |
|---|---|
| ✅ Coverage (%) | `coverage` |
| Line coverage (%) | `line_coverage` |
| Branch / condition coverage (%) | `branch_coverage` |
| Uncovered lines | `uncovered_lines` |
| Uncovered conditions | `uncovered_conditions` |
| Unit tests (count) | `tests` |
| Unit test failures | `test_failures` |
| Unit test errors | `test_errors` |
| Skipped unit tests | `skipped_tests` |
| Unit test success (%) | `test_success_density` |
| Unit test duration | `test_execution_time` |

## Duplication

| Condition | Metric key |
|---|---|
| ✅ Duplicated lines (%) | `duplicated_lines_density` |
| Duplicated lines | `duplicated_lines` |
| Duplicated blocks | `duplicated_blocks` |
| Duplicated files | `duplicated_files` |

## Size and complexity

These can be gate conditions, but they measure how big the project is rather than whether the code is good, so they rarely make useful gates.

| Condition | Metric key |
|---|---|
| Cyclomatic complexity | `complexity` |
| Cognitive complexity | `cognitive_complexity` |
| Comments (%) | `comment_lines_density` |
| Lines of code | `ncloc` |
| Files / classes / functions / statements | `files`, `classes`, `functions`, `statements` |

---

## Our current gate (`ci-gate`)

| Condition | Fails when |
|---|---|
| Reliability issues | > 0 |
| Security issues | > 0 |
| Reliability rating | worse than A |
| Security rating | worse than A |
| Coverage | < 60% |
| Duplicated lines | > 5% |
| Blocker severity issues *(proposed, see PR #1)* | > 0 |

## Rules per language

| Language | Rules available | Enabled in "Sonar way" (default profile) |
|---|---|---|
| C# | 456 | 326 |
| TypeScript | 533 | 428 |
| JavaScript | 515 | 415 |

134 of the TypeScript/JavaScript rules are tagged `react` (hooks, JSX, props and so on).

## Making any rule break the build

1. Copy the built-in **Sonar way** profile for the language. Built-in profiles are read-only.
2. Raise the rule's severity to **Blocker** in the copy, e.g. `csharpsquid:S1481` (unused local variable).
3. Keep the gate condition **Blocker severity issues > 0**.

The same condition then covers C# and React. SonarQube lets you change a rule's **severity** but not its **category**, so an unused variable stays a Maintainability issue and can't be relabelled a Bug. Blocker severity is what makes it fail the gate.

The demo PR ([#1](https://github.com/pharaujo-git/finance/pull/1)) turns this into one variable in `.github/workflows/ci.yml`:

```yaml
UNUSED_VARIABLE_SEVERITY: BLOCKER   # LOW = report only, BLOCKER = fail the build
```

> **React note:** coverage conditions only work if the frontend job sends its coverage report to the scan (for us, `lcov` from Vitest).
