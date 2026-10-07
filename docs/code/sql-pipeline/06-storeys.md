---
audience: developer
---

# Script 06: Storeys

**File:** `sql/scripts/main/06_calc_storeys.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Counts each building's storeys from its geometry and sets the heated floor area. The rule separates the storeys below the eave from a storey under the roof, because the two are counted differently in building-energy practice: TABULA counts complete storeys without the attic, and an observer looking at a house counts the roof storey too.

No single published method combines these steps. Each step below rests on its own source, and the result is checked against an independent storey count ([Validation](#validation)).

---

## Method

| Step | Rule | Source |
|------|------|--------|
| 1. Full storeys | `full_storeys = max(1, round(min_height / storey_height))` | Geometric method of Roy et al. (2023): building height divided by storey height, rounded to the nearest storey |
| 2. Storey height | `storey_height` = `STOREY_HEIGHT`, default 2.8 m, floor surface to floor surface, slab included | EnEV 2014, Anlage 1 Nr. 1.3.3 (see below) |
| 3. Attic storey | `attic_storey` when `max_height − min_height ≥ 2 m` | WoFlV § 4: floor area with a clear height of at least 2 m counts in full |
| 4. Attic floor area | `attic_floor_area` from the roof geometry (script 04), weighted as in WoFlV § 4 | WoFlV § 4; TABULA adds the attic as a fraction (`n_Storey_eff`, Loga et al. 2015) |
| 5. Totals | `number_of_storeys = full_storeys + attic_storey`; `area_total_floor = footprint_area × full_storeys`, plus `attic_floor_area` when `attic_storey` | |

`min_height` is the eave height from script 04: each solid's eave (the roof-area-weighted lowest point of its roof), combined across solids by footprint. `max_height` is the ridge height.

TABULA matching ([post script 02](07-tabula-labelling.md)) compares `full_storeys`, not `number_of_storeys`, with TABULA's `n_Storey`, which counts complete conditioned storeys without attic and cellar (Loga et al. 2015).

### Storey height

EnEV 2014, Anlage 1 Nr. 1.3.3, sets the reference floor area of a residential building to A_N = 0.32 m⁻¹ · Ve when the average storey height h_G, "gemessen von der Oberfläche des Fußbodens zur Oberfläche des Fußbodens des darüber liegenden Geschosses", lies between 2.5 m and 3 m, and to A_N = (1/h_G − 0.04 m⁻¹) · Ve otherwise. The two formulas agree at h_G = 2.78 m, the storey height the 0.32 factor assumes. The current law (GModG § 25 Abs. 10, with DIN V 18599-1:2018-09) keeps the 2.5 m to 3 m band. Dutch storey heights are about 3.0 m for buildings a century old and 2.65 m for buildings since 2003 (cited in Roy et al. 2023). In the TABULA example buildings, gross volume over gross floor area (`V_C / A_C_ExtDim`) has country medians of 2.75 m to 3.33 m.

One value serves all countries. Rounding absorbs an error of up to half a storey, so a constant gives the right count when it lies within this range of the true storey height:

| True storeys | Constant may be off by |
|---|---|
| 2 | −20 % to +33 % |
| 3 | −14 % to +20 % |
| 4 | −11 % to +14 % |
| 5 | −9 % to +11 % |

The TABULA country medians lie within that band of 2.8 m. The spread within a country, by construction period and ground-floor use, is larger than the spread between countries.

### Attic storey and attic floor area

WoFlV § 4 counts floor area with a clear height of at least 2 m in full, from 1 m to 2 m half, and below 1 m not at all. The roof space counts as a storey when part of it reaches the 2 m, which for a pitched roof means a rise of at least 2 m above the eave.

Script 04 computes `attic_floor_area` per solid. Each roof face is taken to rise linearly from its lowest to its highest point above the eave, so the share of its plan area at height h or more is (top − h) / (top − bottom). That plan area is weighted 1 above 2 m and 0.5 between 1 m and 2 m. A 10 m by 8 m gable house with a 30° roof (rise 2.31 m) has 28.0 m² of attic floor area on its 80 m² footprint.

`attic_floor_area` is stored for every building but adds to `area_total_floor` only when the roof space counts as a storey.

---

## Validation

`b3_bouwlagen` in 3DBAG is the floor count estimated by the gradient-boosting model of Roy et al. (2023), and it includes floors under the roof. For 1,303 buildings of the Loenen test dataset, against `number_of_storeys`:

| Rule | Exact | Mean absolute error | Bias |
|---|---|---|---|
| Before: tallest wall / 2.5 m | 34.2 % | 0.73 | +0.70 |
| Eave / 2.8 m, no attic | 8.4 % | 1.06 | −1.06 |
| **Eave / 2.8 m, plus attic storey at a rise of 2 m** | **73.5 %** | **0.27** | **−0.11** |
| Same at 3.0 m | 71.2 % | 0.29 | −0.19 |

Over all 10,398 buildings of the Loenen source tiles that have a 3DBAG floor count, the rule is exact for 78.2 % (mean absolute error 0.22, bias −0.10). The other rows were computed per building from the same faces, without the pipeline's per-solid weighting.

The geometric baseline of Roy et al. (2023) was exact for 69.9 % of Dutch residential buildings with up to five floors. The 2 m attic threshold comes from WoFlV § 4, not from this data; of the thresholds tested, 2 m also fits best (2.5 m: 69.7 %, 3 m: 62.6 % exact).

!!! warning "Limits"
    - The rise test measures from the eave and ignores the top slab and any knee wall (Drempel).
    - WoFlV is German law. Applying its 2 m threshold elsewhere is an assumption; national attic rules are similar, for example the Netherlands excludes floor area under 1.5 m from usable area.
    - The only validation reference is a model estimate for one Dutch village.

---

## Corrections

`storey_height` and `number_of_storeys` can be corrected by hand. The correction triggers keep `min_height = storey_height × full_storeys` from either side and recompute `area_total_floor`; see `sql/schema/main/03_create_correction_triggers.sql`. `room_height` stays at TABULA's 2.5 m reference room height (`h_room`), which TABULA uses only for ventilation volume.

---

## References

- Roy, E., Pronk, M., Agugiaro, G. and Ledoux, H. (2023). Inferring the number of floors for residential buildings. *International Journal of Geographical Information Science* 37(4), 938–962. <https://doi.org/10.1080/13658816.2022.2160454>
- Loga, T., Müller, K., Reifschläger, K. and Stein, B. (2015). *Evaluation of the TABULA Database: Comparison of Typical Buildings and Heat Supply Systems from 20 European Countries.* Institut Wohnen und Umwelt, Darmstadt. <https://episcope.eu/fileadmin/tabula/public/docs/report/TABULA_WorkReport_EvaluationDatabase.pdf>
- Energieeinsparverordnung (EnEV) 2014, Anlage 1 Nr. 1.3.3.
- Gebäudemodernisierungsgesetz (GModG), § 25 Abs. 10. <https://www.gesetze-im-internet.de/geg/__25.html>
- Wohnflächenverordnung (WoFlV), § 4. <https://www.gesetze-im-internet.de/woflv/__4.html>
