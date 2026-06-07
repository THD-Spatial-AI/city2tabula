# Generating Validation Reports

`validation/generate_report.py` parses City2TABULA validation output folders and produces a single paper-ready markdown report. It automatically finds the latest run in each folder and extracts four sections: accuracy, geometry quality, error stratification, and building height analysis.

---

## When to Use It

After running `city2tabula -extract-features` and the validation notebook on one or more datasets, use this script to turn the raw CSVs into a formatted document you can copy directly into a paper or archive alongside the results.

---

## Input: Validation Output Structure

The script expects the folder layout produced by the validation notebook:

```
outputs/
└── Germany/
    └── c2t_deggendorf/
        └── validation_20260528_190945/   ← latest run picked automatically
            ├── building_summary.csv
            ├── building_validation.csv
            ├── roof_summary.csv
            ├── roof_validation.csv
            ├── wall_summary.csv
            ├── wall_validation.csv
            ├── floor_summary.csv
            ├── floor_validation.csv
            ├── problematic_roofs.csv
            ├── problematic_walls.csv
            └── plots/
```

If multiple `validation_*` subfolders exist, the lexicographically latest one (i.e. the most recent timestamp) is used.

---

## Usage

### Single dataset

```bash
python validation/generate_report.py validation/outputs/Germany/c2t_deggendorf
```

Output: `validation/outputs/Germany/c2t_deggendorf/report.md`

### Multiple datasets — combined report

```bash
python validation/generate_report.py \
  --dataset "Freiburg (DE):validation/outputs/Germany/c2t_freiburg" \
  --dataset "Vienna (AT):validation/outputs/Austria/c2t_vienna_130k" \
  --dataset "Deggendorf (DE):validation/outputs/Germany/c2t_deggendorf" \
  --output validation/outputs/combined_report.md
```

Labels can be any string; they appear as the **Dataset** column in all tables.

### Custom output path

```bash
python validation/generate_report.py \
  --dataset "Deggendorf (DE):validation/outputs/Germany/c2t_deggendorf" \
  --output my_report.md
```

---

## Output Sections

### 1 — Accuracy

RMSE and mean signed difference for every validated attribute across all datasets.

```
| Dataset        | Level    | Attribute        |       n | RMSE  | Mean diff | Median %err |
| ---            | ---      | ---              |     --- | ---   | ---       | ---         |
| Freiburg (DE)  | Roof     | Surface area (m²)| 22,411  | 0.392 | −0.004    | 0.000       |
| Freiburg (DE)  | Wall     | Surface area (m²)| 55,129  | 0.004 |  0.000    | 0.000       |
| Vienna (AT)    | Wall     | Surface area (m²)|1,180,291| 17.360| −0.030    | 0.000       |
...
```

### 2 — Geometry Quality

Per surface type: how many surfaces are PostGIS-invalid (`ST_IsValid = false`) or non-planar. Counted on unique surfaces (area attribute only, to avoid double-counting tilt/azimuth rows).

```
| Dataset        | Surface type | n surfaces | Invalid (%) | Non-planar (%) |
| ---            | ---          | ---        | ---         | ---            |
| Freiburg (DE)  | Roof         | 22,411     | 0.0%        | 52.2%          |
| Freiburg (DE)  | Wall         | 55,129     | 89.0%       | 13.1%          |
| Vienna (AT)    | Wall         |1,180,291   | 99.4%       | 17.2%          |
...
```

### 3 — RMSE by Geometry Validity

Surface area RMSE split into two groups: surfaces that passed both `is_valid` and `is_planar`, and surfaces that failed either check. This isolates the contribution of degenerate geometry to overall error.

```
| Dataset        | Surface type | RMSE all | RMSE valid+planar | RMSE invalid/non-planar | n valid | n invalid |
| ---            | ---          | ---      | ---               | ---                     | ---     | ---       |
| Deggendorf (DE)| Wall         | 38.177   | 0.016             | 38.256                  | 3,482   | 840,307   |
| Vienna (AT)    | Wall         | 17.360   | 0.003             | 17.362                  | 240     | 1,180,051 |
...
```

!!! tip "The geometry argument"
    The **Key Takeaways** section at the end of the report computes the ratio of
    invalid/non-planar RMSE to valid+planar RMSE automatically. For example:

    > Wall area: RMSE for invalid/non-planar surfaces (38.256 m², n=840,307)
    > is **2,431× higher** than valid+planar surfaces (0.016 m², n=3,482).

    This is the quantitative argument that geometry quality — not the pipeline — drives the remaining error.

### 4 — Building Height by Attachment Status

Height RMSE split by `has_attached_neighbour`. Buildings that share a wall with a neighbour can have their roof surfaces mis-attributed during geometry-based surface assignment, inflating height errors.

!!! note
    This section only appears when `has_attached_neighbour` is present in `building_validation.csv`. Run `flag_attached_buildings.py` before the validation notebook to populate this column.

---

## Example: Full Three-Dataset Run

```bash
cd city2tabula

python validation/generate_report.py \
  --dataset "Freiburg (DE):validation/outputs/Germany/c2t_freiburg" \
  --dataset "Vienna (AT):validation/outputs/Austria/c2t_vienna_130k" \
  --dataset "Deggendorf (DE):validation/outputs/Germany/c2t_deggendorf" \
  --output validation/outputs/combined_report.md

# Report written to: validation/outputs/combined_report.md
```

Open `combined_report.md` — all four tables are ready to copy into the paper. The **Key Takeaways** prose sentences can be used directly in the Discussion section.

---

## What the Script Does Not Do

- It does not re-run validation — it reads existing CSV outputs only.
- It does not generate plots — the individual per-dataset plots are already in each `plots/` subfolder.
- It does not modify any source data or database.
