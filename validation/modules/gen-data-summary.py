#!/usr/bin/env python3
# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Generate LaTeX table rows (or full table) for Bavarian LoD2 descriptive stats,
computed from the `thematic_value` column.

Usage:
  python gen_bavaria_summary.py \
    --building /mnt/data/building_validation.csv \
    --roof /mnt/data/roof_validation.csv \
    --full-table

Or just rows:
  python gen_bavaria_summary.py --building ... --roof ...

Optional:
  python gen_bavaria_summary.py --building ... --roof ... --output bavaria_summary.tex
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path
import sys
import pandas as pd


@dataclass(frozen=True)
class RowSpec:
    category: str
    label: str
    attribute_name: str
    decimals: int = 2


# Map your LaTeX rows to attribute_name values in the CSVs
ROW_SPECS: list[RowSpec] = [
    RowSpec("Building", "Footprint area (m$^2$)", "footprint_area", decimals=2),
    RowSpec("Building", "Minimum height (m)", "min_height", decimals=3),
    RowSpec("Roof", "Surface area (m$^2$)", "surface_area", decimals=2),
    RowSpec("Roof", "Tilt ($^\\circ$)", "tilt", decimals=2),
    RowSpec("Roof", "Azimuth ($^\\circ$)", "azimuth", decimals=2),
]


def _require_cols(df: pd.DataFrame, path: Path, cols: set[str]) -> None:
    missing = cols - set(df.columns)
    if missing:
        raise ValueError(
            f"{path.name}: missing columns {sorted(missing)}. Found columns: {list(df.columns)}"
        )


def _load_and_clean(path: Path) -> pd.DataFrame:
    df = pd.read_csv(path)
    _require_cols(df, path, {"attribute_name", "thematic_value"})
    df = df.copy()
    df["thematic_value"] = pd.to_numeric(df["thematic_value"], errors="coerce")
    df = df.dropna(subset=["attribute_name", "thematic_value"])
    return df


def _stats_by_attribute(df: pd.DataFrame) -> dict[str, dict]:
    stats = (
        df.groupby("attribute_name")["thematic_value"]
        .agg(count="count", mean="mean", std="std", min="min", max="max")
        .reset_index()
    )
    return {row["attribute_name"]: row.to_dict() for _, row in stats.iterrows()}


def _fmt(x: object, decimals: int) -> str:
    if x is None or pd.isna(x):
        return "--"
    try:
        return f"{float(x):.{decimals}f}"
    except Exception:
        return "--"


def _row_to_latex(spec: RowSpec, stats_map: dict[str, dict]) -> str:
    s = stats_map.get(spec.attribute_name)
    if not s:
        return f"{spec.category} & {spec.label} & -- & -- & -- & -- & -- \\\\"

    return (
        f"{spec.category} & {spec.label} & "
        f"{int(s['count'])} & "
        f"{_fmt(s['mean'], spec.decimals)} & "
        f"{_fmt(s['std'], spec.decimals)} & "
        f"{_fmt(s['min'], spec.decimals)} & "
        f"{_fmt(s['max'], spec.decimals)} \\\\"
    )


def _full_table(rows: list[str]) -> str:
    return "\n".join(
        [
            r"\begin{table}[t]",
            r"\centering",
            r"\caption{Descriptive statistics of building- and roof-level attributes for the Bavarian LoD2 dataset.}",
            r"\label{tab:bavaria_summary}",
            r"\begin{tabular}{llrrrrr}",
            r"\hline",
            r"Category & Attribute & Count & Mean & Std. Dev. & Min & Max \\",
            r"\hline",
            *rows,
            r"\hline",
            r"\end{tabular}",
            r"\end{table}",
        ]
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--building", type=Path, required=True, help="building_validation.csv (must include min_height etc.)")
    ap.add_argument("--roof", type=Path, required=True, help="roof_validation.csv (must include surface_area/tilt/azimuth etc.)")
    ap.add_argument("--full-table", action="store_true", help="Output full LaTeX table wrapper (not just rows).")
    ap.add_argument("--output", "-o", type=Path, default=None, help="Write output to a file instead of stdout.")
    args = ap.parse_args()

    for p in [args.building, args.roof]:
        if not p.exists():
            print(f"ERROR: file not found: {p}", file=sys.stderr)
            return 2

    bldg = _load_and_clean(args.building)
    roof = _load_and_clean(args.roof)

    combined = pd.concat([bldg, roof], ignore_index=True)
    stats_map = _stats_by_attribute(combined)

    rows = [_row_to_latex(spec, stats_map) for spec in ROW_SPECS]
    out = _full_table(rows) if args.full_table else "\n".join(rows)

    if args.output:
        args.output.write_text(out, encoding="utf-8")
    else:
        print(out)

    # Diagnostics: show what's available vs missing
    available = sorted(stats_map.keys())
    wanted = [s.attribute_name for s in ROW_SPECS]
    missing = [w for w in wanted if w not in stats_map]
    if missing:
        print(
            "\nNOTE: These attribute_name values were not found, so placeholders '--' were used:\n  "
            + "\n  ".join(missing),
            file=sys.stderr,
        )
        print("\nAvailable attribute_name values across both files:\n  " + ", ".join(available), file=sys.stderr)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
