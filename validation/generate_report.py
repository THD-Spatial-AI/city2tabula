"""
generate_report.py — paper-ready validation report from City2TABULA output folders.

Usage
-----
Single dataset:
    python generate_report.py /path/to/outputs/Germany/c2t_deggendorf

Multiple datasets (combined report):
    python generate_report.py \\
        --dataset "Deggendorf (DE):validation/outputs/Germany/c2t_deggendorf" \\
        --dataset "Freiburg (DE):validation/outputs/Germany/c2t_freiburg" \\
        --dataset "Vienna (AT):validation/outputs/Austria/c2t_vienna_130k"

The script finds the latest validation_* subfolder automatically.

Output
------
  <run_dir>/validation_report.md
  <run_dir>/figs/plots/accuracy_summary.pdf
  <run_dir>/figs/plots/accuracy_summary.png
"""

import argparse
import csv
import math
import re
import sys
from datetime import datetime
from pathlib import Path


# ─────────────────────────────────────────────────────────────────────────────
# Constants
# ─────────────────────────────────────────────────────────────────────────────

# Fixed display order for core attributes: (display_label, unit, summary_file_key, attribute_name)
CORE_ATTRS = [
    ("Tilt",        "°",  "roof",     "tilt"),
    ("Azimuth",     "°",  "roof",     "azimuth"),
    ("Roof area",   "m²", "roof",     "surface_area"),
    ("Wall area",   "m²", "wall",     "surface_area"),
    ("Floor area",  "m²", "floor",    "surface_area"),
    ("Max height",  "m",  "building", "max_height"),
    ("Min height",  "m",  "building", "min_height"),
]

ATTR_GROUP = {
    "Tilt":        "Roof geometry",
    "Azimuth":     "Roof geometry",
    "Roof area":   "Surface area",
    "Wall area":   "Surface area",
    "Floor area":  "Surface area",
    "Max height":  "Building",
    "Min height":  "Building",
}

GROUP_COLORS = {
    "Roof geometry": "#0077BB",
    "Surface area":  "#EE7733",
    "Building":      "#009988",
}

SURFACE_TYPES = ["building", "roof", "wall", "floor"]

ATTR_LABELS = {
    "min_height":   "Min height",
    "max_height":   "Max height",
    "surface_area": "Surface area",
    "tilt":         "Tilt",
    "azimuth":      "Azimuth",
}


# ─────────────────────────────────────────────────────────────────────────────
# CSV helpers
# ─────────────────────────────────────────────────────────────────────────────

def read_csv(path: Path) -> list[dict]:
    """Read a CSV file into a list of dicts. Returns [] if file missing."""
    if not path.exists():
        return []
    with path.open(newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def load_summary(run_path: Path, surface_type: str) -> list[dict]:
    p = run_path / f"{surface_type}_summary.csv"
    if not p.exists():
        print(f"WARNING: missing {p}", file=sys.stderr)
        return []
    rows = read_csv(p)
    if rows and "attribute_name" not in rows[0]:
        print(
            f"WARNING: {p} missing 'attribute_name' column. "
            f"Found: {list(rows[0].keys())}",
            file=sys.stderr,
        )
        return []
    return rows


def load_detail(run_path: Path, surface_type: str) -> list[dict]:
    p = run_path / f"{surface_type}_validation.csv"
    if not p.exists():
        print(f"WARNING: missing {p}", file=sys.stderr)
        return []
    rows = read_csv(p)
    if rows and "attribute_name" not in rows[0]:
        print(
            f"WARNING: {p} missing expected columns. "
            f"Found: {list(rows[0].keys())}",
            file=sys.stderr,
        )
        return []
    return rows


# ─────────────────────────────────────────────────────────────────────────────
# Statistics helpers
# ─────────────────────────────────────────────────────────────────────────────

def safe_float(v) -> float | None:
    if v is None or str(v).strip() == "":
        return None
    try:
        return float(v)
    except (ValueError, TypeError):
        return None


def percentile(values: list[float], p: float) -> float:
    if not values:
        return float("nan")
    s = sorted(values)
    n = len(s)
    idx = (p / 100) * (n - 1)
    lo = int(idx)
    hi = lo + 1
    if hi >= n:
        return s[-1]
    return s[lo] + (idx - lo) * (s[hi] - s[lo])


def rmse_of(rows: list[dict], col: str = "difference") -> float:
    sq = []
    for r in rows:
        v = safe_float(r.get(col))
        if v is not None:
            sq.append(v * v)
    if not sq:
        return float("nan")
    return math.sqrt(sum(sq) / len(sq))


# ─────────────────────────────────────────────────────────────────────────────
# Formatting helpers
# ─────────────────────────────────────────────────────────────────────────────

def fmt(v, decimals: int = 3) -> str:
    if v is None or (isinstance(v, float) and math.isnan(v)):
        return "—"
    return f"{v:.{decimals}f}"


def fmt_n(n) -> str:
    try:
        return f"{int(n):,}"
    except (ValueError, TypeError):
        return str(n)


def pct(n_subset: int, n_total: int) -> str:
    if n_total == 0:
        return "—"
    return f"{100 * n_subset / n_total:.1f}%"


def pipe_table(rows: list[dict]) -> str:
    if not rows:
        return "_No data._"
    cols   = list(rows[0].keys())
    header = "| " + " | ".join(cols) + " |"
    sep    = "| " + " | ".join(["---"] * len(cols)) + " |"
    lines  = [header, sep]
    for row in rows:
        lines.append("| " + " | ".join(str(row.get(c, "—")) for c in cols) + " |")
    return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
# Dataset path helpers
# ─────────────────────────────────────────────────────────────────────────────

def latest_run(dataset_path: Path) -> Path:
    runs = sorted(dataset_path.glob("validation_*"))
    if not runs:
        sys.exit(f"No validation_* folder found in {dataset_path}")
    return runs[-1]


# ─────────────────────────────────────────────────────────────────────────────
# Section 0a: Accuracy Summary Table
# ─────────────────────────────────────────────────────────────────────────────

def get_summary_row(run_path: Path, surf_type: str, attr_name: str) -> dict | None:
    rows = load_summary(run_path, surf_type)
    for r in rows:
        if r.get("attribute_name") == attr_name:
            return r
    print(
        f"WARNING: attribute '{attr_name}' not found in {surf_type}_summary.csv",
        file=sys.stderr,
    )
    return None


def accuracy_summary_table(run_path: Path) -> str:
    table_rows = []
    for label, unit, surf_type, attr_name in CORE_ATTRS:
        r = get_summary_row(run_path, surf_type, attr_name)
        if r is None:
            continue
        table_rows.append({
            "Attribute": f"{label} ({unit})",
            "n":         fmt_n(r.get("count", 0)),
            "RMSE":      fmt(safe_float(r.get("rmse"))),
            "Mean diff": fmt(safe_float(r.get("mean_difference"))),
        })
    lines = [pipe_table(table_rows), ""]
    lines.append(
        "_RMSE and mean signed difference between geometry-derived "
        "and thematic reference values._"
    )
    return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
# Section 0b: Accuracy Summary Plot
# ─────────────────────────────────────────────────────────────────────────────

def accuracy_summary_plot(
    run_path: Path,
    core_data: list[tuple],  # (label_unit, rmse, mean_d, label_base)
) -> None:
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        import matplotlib.patches as mpatches

        out_dir = run_path / "figs" / "plots"
        out_dir.mkdir(parents=True, exist_ok=True)

        # Filter to valid entries
        valid = [(lu, r, md, lb) for lu, r, md, lb in core_data
                 if r is not None and not math.isnan(r)]
        if not valid:
            print("WARNING: no valid RMSE values for plot", file=sys.stderr)
            return

        # Determine scale
        rmses_pos = [r for _, r, _, _ in valid if r > 0]
        use_log = bool(rmses_pos and (max(rmses_pos) / min(rmses_pos)) > 100)

        # Build y-positions (bottom-up so first CORE_ATTR appears at top)
        label_order = [d[0] for d in valid]  # already in CORE_ATTRS order
        y_positions: dict[str, float] = {}
        y = 0.0
        prev_grp = None
        for lu in reversed(label_order):
            lb = next((d[3] for d in valid if d[0] == lu), None)
            grp = ATTR_GROUP.get(lb or "", "Building")
            if prev_grp is not None and grp != prev_grp:
                y += 0.15
            y_positions[lu] = y
            y += 1.0
            prev_grp = grp

        fig, ax = plt.subplots(figsize=(8, 4))
        bar_height = 0.6
        colors_used: dict[str, str] = {}

        for lu, rmse_val, mean_d_val, lb in valid:
            grp   = ATTR_GROUP.get(lb, "Building")
            color = GROUP_COLORS[grp]
            yp    = y_positions.get(lu, 0)
            colors_used[grp] = color

            plot_val = abs(rmse_val) if use_log else rmse_val
            ax.barh(yp, plot_val, height=bar_height, color=color, alpha=0.85, zorder=2)

            # Mean diff diamond marker
            if mean_d_val is not None and not math.isnan(mean_d_val):
                md_plot = abs(mean_d_val) if (use_log and mean_d_val != 0) else mean_d_val
                if use_log and md_plot > 0:
                    ax.plot(md_plot, yp, marker="D", markersize=6, color="#333333", zorder=3)
                elif not use_log:
                    ax.plot(md_plot, yp, marker="D", markersize=6, color="#333333", zorder=3)

            # Bar label (value at right end)
            ax.text(plot_val, yp, f"  {rmse_val:.2f}", va="center", ha="left", fontsize=8)

        # Reference line at x=0
        ax.axvline(0, color="#888888", linewidth=0.6, linestyle="--", zorder=1)

        # y-axis ticks
        yticks  = [y_positions[lu] for lu in label_order]
        ylabels = list(label_order)
        ax.set_yticks(yticks)
        ax.set_yticklabels(ylabels, fontsize=9)

        if use_log:
            ax.set_xscale("log")
        ax.set_xlabel("RMSE", fontsize=10)
        ax.tick_params(axis="x", labelsize=8)
        ax.spines["top"].set_visible(False)
        ax.spines["right"].set_visible(False)

        group_order = ["Roof geometry", "Surface area", "Building"]
        patches = [
            mpatches.Patch(color=GROUP_COLORS[g], label=g)
            for g in group_order if g in colors_used
        ]
        ax.legend(handles=patches, loc="lower right", fontsize=8, framealpha=0.7)

        fig.tight_layout()
        pdf_path = out_dir / "accuracy_summary.pdf"
        png_path = out_dir / "accuracy_summary.png"
        fig.savefig(str(pdf_path), bbox_inches="tight", format="pdf")
        fig.savefig(str(png_path), bbox_inches="tight", dpi=150, format="png")
        plt.close(fig)
        print(f"Plot saved: {pdf_path}", file=sys.stderr)

    except Exception as exc:
        print(f"WARNING: plot generation failed: {exc}", file=sys.stderr)


# ─────────────────────────────────────────────────────────────────────────────
# Section 1: Error Distribution
# ─────────────────────────────────────────────────────────────────────────────

def error_distribution_section(run_path: Path) -> str:
    specs = [
        ("Tilt",       "°",  "roof",     "tilt",         "roof",     "tilt"),
        ("Azimuth",    "°",  "roof",     "azimuth",      "roof",     "azimuth"),
        ("Roof area",  "m²", "roof",     "surface_area", "roof",     "surface_area"),
        ("Wall area",  "m²", "wall",     "surface_area", "wall",     "surface_area"),
        ("Floor area", "m²", "floor",    "surface_area", "floor",    "surface_area"),
        ("Max height", "m",  "building", "max_height",   "building", "max_height"),
        ("Min height", "m",  "building", "min_height",   "building", "min_height"),
    ]

    table_rows = []
    for label, unit, val_type, attr_filter, sum_type, attr_name in specs:
        p = run_path / f"{val_type}_validation.csv"
        if not p.exists():
            print(f"WARNING: missing {p} for error distribution", file=sys.stderr)
            continue

        rows = read_csv(p)
        if not rows:
            continue
        if "difference" not in rows[0]:
            print(
                f"WARNING: 'difference' column absent in {p}. "
                f"Found: {list(rows[0].keys())}",
                file=sys.stderr,
            )
            continue

        diffs = []
        for r in rows:
            if r.get("attribute_name") != attr_filter:
                continue
            v = safe_float(r.get("difference"))
            if v is not None:
                diffs.append(abs(v))
        if not diffs:
            continue

        # RMSE threshold from summary
        rmse_val = None
        for sr in load_summary(run_path, sum_type):
            if sr.get("attribute_name") == attr_name:
                rmse_val = safe_float(sr.get("rmse"))
                break

        tail_str = "—"
        if rmse_val is not None and not math.isnan(rmse_val):
            tail = sum(1 for d in diffs if d > rmse_val) / len(diffs) * 100
            tail_str = f"{tail:.1f}%"

        table_rows.append({
            "Attribute":                 f"{label} ({unit})",
            "P5":                        fmt(percentile(diffs, 5)),
            "P25":                       fmt(percentile(diffs, 25)),
            "P75":                       fmt(percentile(diffs, 75)),
            "P95":                       fmt(percentile(diffs, 95)),
            "Tail fraction (> 1 RMSE)":  tail_str,
        })

    return pipe_table(table_rows) if table_rows else ""


# ─────────────────────────────────────────────────────────────────────────────
# Accuracy detail table rows
# ─────────────────────────────────────────────────────────────────────────────

def accuracy_rows(run_path: Path, label: str) -> list[dict]:
    rows = []
    for st in SURFACE_TYPES:
        sum_rows = load_summary(run_path, st)
        for r in sum_rows:
            attr = r.get("attribute_name", "")
            if st in ("roof", "wall", "floor") and attr == "surface_area":
                disp = f"{st.capitalize()} area"
            else:
                disp = ATTR_LABELS.get(attr, attr)
            med_pct = safe_float(r.get("median_percent_error"))
            rows.append({
                "Dataset":     label,
                "Level":       st.capitalize(),
                "Attribute":   disp,
                "n":           fmt_n(r.get("count", 0)),
                "RMSE":        fmt(safe_float(r.get("rmse"))),
                "Mean diff":   fmt(safe_float(r.get("mean_difference"))),
                "Median %err": fmt(med_pct) if med_pct is not None else "—",
            })
    return rows


# ─────────────────────────────────────────────────────────────────────────────
# Geometry quality table rows
# ─────────────────────────────────────────────────────────────────────────────

def geometry_quality_rows(run_path: Path, label: str) -> list[dict]:
    rows = []
    for st in ["roof", "wall", "floor"]:
        detail = load_detail(run_path, st)
        if not detail or "is_valid" not in detail[0]:
            continue
        seen: set = set()
        area_rows = []
        for r in detail:
            if r.get("attribute_name") != "surface_area":
                continue
            sid = r.get("surface_feature_id")
            if sid not in seen:
                seen.add(sid)
                area_rows.append(r)
        n = len(area_rows)
        if n == 0:
            continue
        n_invalid   = sum(1 for r in area_rows if r.get("is_valid")  == "False")
        n_nonplanar = sum(1 for r in area_rows if r.get("is_planar") == "False")
        rows.append({
            "Dataset":        label,
            "Surface type":   st.capitalize(),
            "n surfaces":     fmt_n(n),
            "Invalid (%)":    pct(n_invalid, n),
            "Non-planar (%)": pct(n_nonplanar, n),
        })
    return rows


# ─────────────────────────────────────────────────────────────────────────────
# Error stratification rows (raw floats for RMSE — used in arithmetic later)
# ─────────────────────────────────────────────────────────────────────────────

def error_stratification_rows(run_path: Path, label: str) -> list[dict]:
    rows = []
    for st in ["roof", "wall"]:
        detail = load_detail(run_path, st)
        if not detail or "is_valid" not in detail[0]:
            continue
        area_rows   = [r for r in detail if r.get("attribute_name") == "surface_area"]
        valid_plan  = [r for r in area_rows if r.get("is_valid") == "True" and r.get("is_planar") == "True"]
        problematic = [r for r in area_rows if r.get("is_valid") != "True" or r.get("is_planar") != "True"]
        if not area_rows:
            continue
        rows.append({
            "Dataset":                      label,
            "Surface type":                 st.capitalize(),
            "RMSE all (m²)":                rmse_of(area_rows),
            "RMSE valid+planar (m²)":       rmse_of(valid_plan),
            "RMSE invalid/non-planar (m²)": rmse_of(problematic),
            "n valid+planar":               len(valid_plan),
            "n invalid/non-planar":         len(problematic),
        })
    return rows


# ─────────────────────────────────────────────────────────────────────────────
# Building height rows
# ─────────────────────────────────────────────────────────────────────────────

def building_height_rows(run_path: Path, label: str) -> list[dict]:
    detail = load_detail(run_path, "building")
    if not detail or "has_attached_neighbour" not in detail[0]:
        return []
    rows = []
    for attr in ["min_height", "max_height"]:
        sub      = [r for r in detail if r.get("attribute_name") == attr]
        attached = [r for r in sub if r.get("has_attached_neighbour") == "True"]
        detached = [r for r in sub if r.get("has_attached_neighbour") == "False"]
        if not sub:
            continue
        rows.append({
            "Dataset":           label,
            "Attribute":         ATTR_LABELS.get(attr, attr) + " (m)",
            "RMSE all (m)":      fmt(rmse_of(sub)),
            "RMSE detached (m)": fmt(rmse_of(detached)),
            "RMSE attached (m)": fmt(rmse_of(attached)),
            "n detached":        fmt_n(len(detached)),
            "n attached":        fmt_n(len(attached)),
        })
    return rows


# ─────────────────────────────────────────────────────────────────────────────
# Section 2: Throughput by Surface Type
# ─────────────────────────────────────────────────────────────────────────────

def _parse_runtime(run_path: Path) -> float | None:
    for fname in ("runtime.txt", "runtime.json"):
        p = run_path / fname
        if p.exists():
            text = p.read_text(encoding="utf-8").strip()
            try:
                return float(text)
            except ValueError:
                pass
            try:
                import json
                data = json.loads(text)
                if isinstance(data, (int, float)):
                    return float(data)
                for v in data.values():
                    if isinstance(v, (int, float)):
                        return float(v)
            except Exception:
                pass

    name = run_path.name
    m = re.search(r"_(\d+)m(\d+)s\b", name)
    if m:
        return float(m.group(1)) * 60 + float(m.group(2))
    m = re.search(r"_(\d+)m\b", name)
    if m:
        return float(m.group(1)) * 60
    m = re.search(r"_(\d+)s\b", name)
    if m:
        return float(m.group(1))

    return None


def throughput_section(run_path: Path) -> str:
    runtime_s = _parse_runtime(run_path)
    if runtime_s is None:
        print("WARNING: runtime not found; skipping throughput section", file=sys.stderr)
        return ""

    surface_counts: dict[str, int] = {}
    for surf_type in ("roof", "wall", "floor"):
        p = run_path / f"{surf_type}_validation.csv"
        if not p.exists():
            continue
        rows = read_csv(p)
        cnt = sum(1 for r in rows if r.get("attribute_name") == "surface_area")
        if cnt > 0:
            surface_counts[surf_type.capitalize()] = cnt

    if not surface_counts:
        return ""

    table_rows = [
        {
            "Surface type":            st,
            "Surface count":           fmt_n(cnt),
            "Runtime (s)":             f"{runtime_s:.1f}",
            "Throughput (surfaces/s)": f"{cnt / runtime_s:.0f}",
        }
        for st, cnt in surface_counts.items()
    ]
    return pipe_table(table_rows)


# ─────────────────────────────────────────────────────────────────────────────
# Section 3: Geometry Validity Impact paragraph
# ─────────────────────────────────────────────────────────────────────────────

def geometry_validity_paragraph(run_path: Path, label: str) -> str:
    parts = []
    for st in ["wall", "roof"]:
        detail = load_detail(run_path, st)
        if not detail or "is_valid" not in detail[0]:
            continue
        area_rows   = [r for r in detail if r.get("attribute_name") == "surface_area"]
        valid_plan  = [r for r in area_rows if r.get("is_valid") == "True" and r.get("is_planar") == "True"]
        problematic = [r for r in area_rows if r.get("is_valid") != "True" or r.get("is_planar") != "True"]
        if not area_rows:
            continue

        rv = rmse_of(valid_plan)
        rb = rmse_of(problematic)
        if math.isnan(rv) or math.isnan(rb) or rv == 0:
            continue

        ratio     = rb / rv
        prop_bad  = len(problematic) / len(area_rows) * 100
        ratio_str = f"{ratio:.0f}×" if ratio >= 10 else f"{ratio:.1f}×"

        if st == "wall":
            parts.append(
                f"**{label}:** Wall surface area error is dominated by geometry quality: "
                f"surfaces that fail either the `ST_IsValid` or planarity check account for "
                f"{prop_bad:.1f}% of wall surfaces and show an RMSE of {fmt(rb)} m² — "
                f"{ratio_str} higher than the {fmt(rv)} m² RMSE of the valid+planar subset."
            )
        else:
            parts.append(
                f"**{label}:** Roof surfaces show a {ratio_str} RMSE ratio between "
                f"invalid/non-planar ({fmt(rb)} m²) and valid+planar ({fmt(rv)} m²) subsets, "
                f"with {prop_bad:.1f}% non-planar."
            )
    return "\n\n".join(parts)


# ─────────────────────────────────────────────────────────────────────────────
# Section 4: Batch Processing
# ─────────────────────────────────────────────────────────────────────────────

def _count_batch_failures(run_path: Path) -> int | None:
    """Return number of batches that failed all retries, or None if unknown."""
    for fname in ("batch_results.csv", "batch_log.csv"):
        p = run_path / fname
        if p.exists():
            rows = read_csv(p)
            return sum(
                1 for r in rows
                if r.get("status", "").lower() in ("failed", "error", "fail")
            )
    for fname in ("pipeline.log", "run.log"):
        p = run_path / fname
        if p.exists():
            text = p.read_text(encoding="utf-8", errors="replace")
            return sum(
                1 for line in text.splitlines()
                if "failed" in line.lower() and "batch" in line.lower()
            )
    return None


def batch_processing_section(run_path: Path) -> str:
    for fname in ("batch_results.csv", "batch_log.csv"):
        p = run_path / fname
        if p.exists():
            rows = read_csv(p)
            if not rows:
                continue
            total    = len(rows)
            failed   = sum(1 for r in rows if r.get("status", "").lower() in ("failed", "error", "fail"))
            retried  = sum(1 for r in rows if int(r.get("retries", 0) or 0) > 0)
            lines    = [
                f"- **Total batches:** {total:,}",
                f"- **Required at least one retry:** {retried:,}",
                f"- **Failed all retries:** {failed:,}",
            ]
            fail_rows = [r for r in rows if r.get("status", "").lower() in ("failed", "error", "fail")]
            if fail_rows:
                lines.append("\n**Failed batches:**\n")
                for r in fail_rows[:10]:
                    step  = r.get("step", r.get("sql_step", "unknown"))
                    error = r.get("error", r.get("error_type", "unknown"))
                    lines.append(f"- Step `{step}`: {error}")
            return "\n".join(lines)

    for fname in ("pipeline.log", "run.log"):
        p = run_path / fname
        if p.exists():
            lines_in = p.read_text(encoding="utf-8", errors="replace").splitlines()
            total = retried = failed = succeeded = 0
            fail_details = []
            for line in lines_in:
                ll = line.lower()
                if "batch" in ll and "start" in ll:
                    total += 1
                if "retry" in ll or "retrying" in ll:
                    retried += 1
                if ("failed" in ll or "error" in ll) and "batch" in ll:
                    failed += 1
                    fail_details.append(line.strip())
                if "batch" in ll and ("success" in ll or "complete" in ll or "done" in ll):
                    succeeded += 1
            if total == 0 and succeeded == 0 and failed == 0:
                return ""
            out = [
                f"- **Total batches detected:** {total:,}",
                f"- **Succeeded:** {succeeded:,}",
                f"- **Retried:** {retried:,}",
                f"- **Failed:** {failed:,}",
            ]
            if fail_details:
                out.append("\n**Failure lines from log:**\n")
                for d in fail_details[:10]:
                    out.append(f"- `{d}`")
            return "\n".join(out)

    print("WARNING: no batch data found; skipping batch processing section", file=sys.stderr)
    return ""


# ─────────────────────────────────────────────────────────────────────────────
# Key Takeaways (extended)
# ─────────────────────────────────────────────────────────────────────────────

def key_takeaways(run_path: Path, label: str, strat_rows: list[dict]) -> str:
    lines = [f"**{label}**\n"]

    # Existing: per surface type ratio sentences
    for r in strat_rows:
        if r["Dataset"] != label:
            continue
        st    = r["Surface type"]
        r_all = r["RMSE all (m²)"]
        r_g   = r["RMSE valid+planar (m²)"]
        r_b   = r["RMSE invalid/non-planar (m²)"]
        n_g   = fmt_n(r["n valid+planar"])
        n_b   = fmt_n(r["n invalid/non-planar"])

        if not math.isnan(r_b) and not math.isnan(r_g) and r_g > 0:
            ratio = r_b / r_g
            lines.append(
                f"- {st} area: RMSE for invalid/non-planar surfaces "
                f"({fmt(r_b)} m², n={n_b}) is "
                f"{ratio:.1f}× higher than valid+planar surfaces "
                f"({fmt(r_g)} m², n={n_g})."
            )
        else:
            lines.append(f"- {st} area RMSE (all): {fmt(r_all)} m².")

    # Rule A: Wall data quality flag
    for r in strat_rows:
        if r["Dataset"] != label or r["Surface type"] != "Wall":
            continue
        r_g = r["RMSE valid+planar (m²)"]
        r_b = r["RMSE invalid/non-planar (m²)"]
        if not math.isnan(r_b) and not math.isnan(r_g) and r_g > 0:
            ratio = r_b / r_g
            if ratio > 100:
                lines.append(
                    f"- Wall area RMSE for invalid/non-planar surfaces ({ratio:.0f}×) "
                    f"indicates a source data quality issue in the input CityGML, "
                    f"not a pipeline extraction error."
                )

    # Rule B: Max height systematic bias
    for sr in load_summary(run_path, "building"):
        if sr.get("attribute_name") == "max_height":
            mean_d = safe_float(sr.get("mean_difference"))
            rmse_v = safe_float(sr.get("rmse"))
            if mean_d is not None and mean_d > 1.0:
                lines.append(
                    f"- Max height shows a systematic positive bias of {mean_d:.2f} m "
                    f"(RMSE {fmt(rmse_v)} m), suggesting consistent overestimation "
                    f"relative to thematic values."
                )

    # Rule C: Non-planar roof majority
    roof_detail = load_detail(run_path, "roof")
    if roof_detail and "is_planar" in roof_detail[0]:
        area_rows = [r for r in roof_detail if r.get("attribute_name") == "surface_area"]
        if area_rows:
            n_nonplanar = sum(1 for r in area_rows if r.get("is_planar") != "True")
            frac = n_nonplanar / len(area_rows)
            if frac > 0.80:
                lines.append(
                    f"- {frac * 100:.1f}% of roof surfaces are non-planar — "
                    f"this reflects the LoD geometry representation, not a classification error."
                )

    # Rule D: Batch failures
    n_failures = _count_batch_failures(run_path)
    if n_failures is not None and n_failures > 0:
        lines.append(
            f"- {n_failures} batch(es) failed all retries. "
            f"Check the pipeline log for details."
        )

    # Rule E: No attached buildings
    bld_detail = load_detail(run_path, "building")
    if bld_detail and "has_attached_neighbour" in bld_detail[0]:
        has_attached = any(r.get("has_attached_neighbour") == "True" for r in bld_detail)
        if not has_attached:
            lines.append(
                "- Attachment-status analysis is unavailable for this dataset "
                "(no attached buildings detected). Height results represent detached buildings only."
            )

    return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
# Report assembly
# ─────────────────────────────────────────────────────────────────────────────

def build_report(datasets: list[tuple[str, Path]]) -> str:
    multi = len(datasets) > 1

    # Collect rows that span all datasets
    all_acc_rows: list[dict]   = []
    all_geom_rows: list[dict]  = []
    all_strat_rows: list[dict] = []
    all_height_rows: list[dict] = []

    for label, path in datasets:
        run = latest_run(path)
        all_acc_rows    += accuracy_rows(run, label)
        all_geom_rows   += geometry_quality_rows(run, label)
        all_strat_rows  += error_stratification_rows(run, label)
        all_height_rows += building_height_rows(run, label)

    sections: list[str] = []

    # ── Accuracy ─────────────────────────────────────────────────────────────
    sections.append("## Accuracy\n")

    # 0a: Summary table
    sections.append("### Summary\n")
    for label, path in datasets:
        run = latest_run(path)
        if multi:
            sections.append(f"**{label}**\n")
        sections.append(accuracy_summary_table(run))
        sections.append("")

    # Detailed accuracy table
    sections.append("### Detail\n")
    sections.append(
        "_RMSE and mean signed difference between geometry-derived and "
        "thematic reference values. n = number of matched surface comparisons._\n"
    )
    sections.append(pipe_table(all_acc_rows))

    # ── Error Distribution ────────────────────────────────────────────────────
    sections.append("\n\n## Error Distribution\n")
    sections.append(
        "_5th, 25th, 75th, 95th percentile of |difference|. "
        "Tail fraction = proportion of rows where |difference| > RMSE._\n"
    )
    for label, path in datasets:
        run = latest_run(path)
        if multi:
            sections.append(f"**{label}**\n")
        ed = error_distribution_section(run)
        if ed:
            sections.append(ed)
            sections.append("")

    # ── Geometry Quality ──────────────────────────────────────────────────────
    sections.append("\n\n## Geometry Quality\n")
    sections.append(
        "_Invalid: PostGIS `ST_IsValid` = false. "
        "Non-planar: planarity check failed during extraction. "
        "n is counted per unique *validated* surface (surfaces with a matching thematic property "
        "in the source dataset, area attribute only). Surfaces without a thematic value are excluded "
        "and do not appear in this count._\n"
    )
    sections.append(pipe_table(all_geom_rows))

    # ── Throughput ────────────────────────────────────────────────────────────
    throughput_blocks: list[tuple[str, str]] = []
    for label, path in datasets:
        run = latest_run(path)
        tp = throughput_section(run)
        if tp:
            throughput_blocks.append((label, tp))

    if throughput_blocks:
        sections.append("\n\n## Throughput by Surface Type\n")
        for label, tp in throughput_blocks:
            if multi:
                sections.append(f"**{label}**\n")
            sections.append(tp)
            sections.append("")

    # ── Surface Area RMSE by Validity ─────────────────────────────────────────
    sections.append("\n\n## Surface Area RMSE by Geometry Validity\n")
    sections.append(
        "_RMSE split by whether the surface passed both `is_valid` and `is_planar` checks. "
        "This isolates the contribution of degenerate geometry to overall error._\n"
    )
    strat_display = [
        {
            "Dataset":                      r["Dataset"],
            "Surface type":                 r["Surface type"],
            "RMSE all (m²)":                fmt(r["RMSE all (m²)"]),
            "RMSE valid+planar (m²)":       fmt(r["RMSE valid+planar (m²)"]),
            "RMSE invalid/non-planar (m²)": fmt(r["RMSE invalid/non-planar (m²)"]),
            "n valid+planar":               fmt_n(r["n valid+planar"]),
            "n invalid/non-planar":         fmt_n(r["n invalid/non-planar"]),
        }
        for r in all_strat_rows
    ]
    sections.append(pipe_table(strat_display))

    # Geometry validity impact paragraph
    validity_paras = []
    for label, path in datasets:
        run  = latest_run(path)
        para = geometry_validity_paragraph(run, label)
        if para:
            validity_paras.append(para)
    if validity_paras:
        sections.append("\n")
        sections.append("\n\n".join(validity_paras))

    # ── Building Height RMSE ──────────────────────────────────────────────────
    if all_height_rows:
        sections.append("\n\n## Building Height RMSE by Attachment Status\n")
        sections.append(
            "_Attached buildings share a wall with a neighbour. "
            "Surface assignment can mis-attribute a neighbour's roof, "
            "inflating height errors._\n"
        )
        sections.append(pipe_table(all_height_rows))

    # ── Batch Processing ──────────────────────────────────────────────────────
    batch_blocks: list[tuple[str, str]] = []
    for label, path in datasets:
        run = latest_run(path)
        bp  = batch_processing_section(run)
        if bp:
            batch_blocks.append((label, bp))

    if batch_blocks:
        sections.append("\n\n## Batch Processing\n")
        for label, bp in batch_blocks:
            if multi:
                sections.append(f"**{label}**\n")
            sections.append(bp)
            sections.append("")

    # ── Key Takeaways ─────────────────────────────────────────────────────────
    sections.append("\n\n## Key Takeaways\n")
    for label, path in datasets:
        run       = latest_run(path)
        ds_strat  = [r for r in all_strat_rows if r["Dataset"] == label]
        sections.append(key_takeaways(run, label, ds_strat))
        sections.append("")

    # ── Generate plots (Section 0b) ───────────────────────────────────────────
    for label, path in datasets:
        run       = latest_run(path)
        core_data = []
        for lbl, unit, surf_type, attr_name in CORE_ATTRS:
            r = get_summary_row(run, surf_type, attr_name)
            if r is None:
                continue
            rmse_v = safe_float(r.get("rmse"))
            mean_v = safe_float(r.get("mean_difference"))
            if rmse_v is not None:
                core_data.append((f"{lbl} ({unit})", rmse_v, mean_v or float("nan"), lbl))
        if core_data:
            accuracy_summary_plot(run, core_data)

    # ── Header ────────────────────────────────────────────────────────────────
    label_str = ", ".join(label for label, _ in datasets)
    run_dirs  = [str(latest_run(path)) for _, path in datasets]
    now       = datetime.now().strftime("%Y-%m-%d %H:%M")

    header = (
        f"# Validation Report\n\n"
        f"**Datasets:** {label_str}\n"
        f"**Generated:** {now}\n"
        f"**Generated from:**\n"
        + "".join(f"- `{r}`\n" for r in run_dirs)
        + "\n---\n\n"
    )

    return header + "\n\n".join(sections)


# ─────────────────────────────────────────────────────────────────────────────
# CLI
# ─────────────────────────────────────────────────────────────────────────────

def parse_args():
    parser = argparse.ArgumentParser(
        description="Generate a validation report from City2TABULA output folders.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Single dataset (label defaults to folder name)
  python generate_report.py validation/outputs/Germany/c2t_deggendorf

  # Multiple datasets with custom labels
  python generate_report.py \\
    --dataset "Freiburg (DE):validation/outputs/Germany/c2t_freiburg" \\
    --dataset "Vienna (AT):validation/outputs/Austria/c2t_vienna_130k" \\
    --dataset "Deggendorf (DE):validation/outputs/Germany/c2t_deggendorf"
""",
    )
    parser.add_argument(
        "path",
        nargs="?",
        metavar="PATH",
        help="Single dataset folder.",
    )
    parser.add_argument(
        "--dataset",
        action="append",
        dest="datasets",
        metavar="LABEL:PATH",
        help="Dataset as 'Label:path'. Repeat for multiple datasets.",
    )
    parser.add_argument(
        "--output",
        metavar="FILE",
        help="Output file (default: validation_report.md inside the run directory).",
    )
    return parser.parse_args()


def main():
    args = parse_args()

    if args.datasets:
        datasets = []
        for entry in args.datasets:
            if ":" not in entry:
                sys.exit(f"--dataset must be 'Label:path', got: {entry!r}")
            label, _, path_str = entry.partition(":")
            datasets.append((label.strip(), Path(path_str.strip())))
    elif args.path:
        p = Path(args.path)
        datasets = [(p.name, p)]
    else:
        sys.exit("Provide either a PATH or one or more --dataset LABEL:PATH arguments.")

    report = build_report(datasets)

    if args.output:
        out = Path(args.output)
    else:
        out = latest_run(datasets[0][1]) / "validation_report.md"

    out.write_text(report, encoding="utf-8")
    print(f"Report written to: {out}")


if __name__ == "__main__":
    main()
