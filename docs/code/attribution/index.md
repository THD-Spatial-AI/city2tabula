---
audience: developer
---

# Dataset attribution file

Every source dataset carries one `attribution.json` at the root of its folder. It credits the provider and states the licence, so the credit can be shown with any output derived from the dataset.

```json
{
  "schema_version": 1,
  "dataset_id": "de-bavaria-lod2",
  "provider": "Bayerische Vermessungsverwaltung",
  "dataset": "3D-Gebäudemodelle (LoD2)",
  "licence": "CC-BY-4.0",
  "licence_url": "https://creativecommons.org/licenses/by/4.0/",
  "credit": "Bayerische Vermessungsverwaltung – www.geodaten.bayern.de",
  "credit_url": "https://www.geodaten.bayern.de",
  "terms_url": "https://geodaten.bayern.de/odd/m/3/html/nutzungsbedingungen.html",
  "changes": "City2TABULA derives building attributes (areas, heights, volume, storeys, TABULA type) from the geometry, merges a building's parts into one building, removes or clips faces shared between parts, and omits buildings without wall or roof faces."
}
```

!!! note "Current use"
    `internal/attribution` reads and validates the file. The importer does not read it yet.

The format is published as a JSON Schema, [`attribution.schema.json`](attribution.schema.json), for checking a file before City2TABULA reads it.

## Fields

| Field | Required | Rule |
|---|---|---|
| `schema_version` | yes | `1` |
| `dataset_id` | yes | Lowercase letters and digits in hyphen-separated words, unique across datasets. 3D datasets use `<country code>-<region>-<lod>`, e.g. `cz-brno-lod2`. |
| `provider` | yes | Organisation that publishes the dataset |
| `dataset` | yes | Dataset name as the provider publishes it |
| `licence` | yes | SPDX licence id, e.g. `CC-BY-4.0` or `DL-DE-BY-2.0`, or `LicenseRef-<id>` for a licence not on the SPDX list |
| `licence_url` | yes | Absolute http(s) URL of the licence text or the provider's usage rules |
| `credit` | yes | Credit line exactly as the provider requires it. Required for every licence, including those that do not demand a credit. |
| `credit_url` | no | Link the provider asks the credit to carry. Omit it when the provider names none. |
| `terms_url` | no | Provider's terms of use, when separate from the licence |
| `changes` | yes | What City2TABULA does to the data, not the state of the download. The credit is shown with City2TABULA's output, and CC BY requires modifications to be indicated. |

Unknown fields are rejected, so a misspelt key fails rather than leaving its field empty. `licence` is checked for form only, not against the SPDX list.

## Validation errors

A file that breaks a rule fails with one error naming the file and listing every broken rule:

```text
data/lod2/czechia/brno/attribution.json: credit is required
licence "CC BY 4.0" must be an SPDX licence id, e.g. CC-BY-4.0, or LicenseRef-<id>
```
