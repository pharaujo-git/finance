"""Command line: python -m app.report reports/profile.json -o reports/report.html"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from app.report.analysis import analyze
from app.report.profile import ProfileError, load_profile
from app.report.render import potential_savings, render


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="python -m app.report", description="Build the household finance report."
    )
    parser.add_argument("profile", type=Path, help="the household profile (JSON)")
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="where to write the HTML (default: next to the profile, as .html)",
    )
    args = parser.parse_args(argv)

    try:
        profile = load_profile(args.profile)
    except ProfileError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    analysis = analyze(profile)
    output = args.output or args.profile.with_suffix(".html")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(render(analysis), encoding="utf-8")

    print(f"Wrote {output}")
    print(
        f"  savings rate {analysis.savings_rate:.0%}, unassigned {analysis.surplus:,.2f}/mo, "
        f"{len(analysis.suggestions)} ideas worth ~{potential_savings(analysis):,.0f}/mo"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
