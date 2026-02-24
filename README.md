# City2TABULA

[![go](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml/badge.svg)](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml)
&nbsp;
[![rtd](https://app.readthedocs.org/projects/city2tabula/badge/?version__slug=latest)](https://city2tabula.readthedocs.io/en/latest/)
&nbsp;
[![GitHub release](https://img.shields.io/github/v/release/THD-Spatial/City2TABULA.svg)](https://github.com/THD-Spatial/City2TABULA/releases)

City2TABULA is a high-performance, Go-based data preparation tool for 3D building datasets stored in PostgreSQL/PostGIS using CityDB schemas. Its primary purpose is to extract, normalise, and enrich geometric and spatial attributes from LoD2 and LoD3 building models, enabling downstream tasks such as building typology classification and heating demand estimation.

The tool is designed as an upstream component within a larger research and modelling pipeline, where prepared building-level features are later consumed by machine learning models or energy calculation services (for example, TABULA-based workflows).

City2TABULA focuses on scalable, database-centric processing of large national or city-scale datasets, avoiding country-specific assumptions and minimising manual intervention.

---

## Getting started

- Setup & installation guide (Docker + manual): [docs/getting-started/setup.md](docs/getting-started/setup.md)
- Full documentation: [city2tabula docs](https://thd-spatial.github.io/city2tabula)

Local docs preview (MkDocs):

```bash
python -m pip install -r docs/requirements.txt
python -m mkdocs serve
```

## CLI

Build locally:

```bash
go build -o city2tabula ./cmd
./city2tabula --help
```

Common flags:

- `--create-db`
- `--reset-all`
- `--reset-citydb`
- `--reset-city2tabula`
- `--extract-features`
- `--version` / `-v`

## License

This project is licensed under the Apache License Version 2.0 - see the [LICENSE](/LICENSE) file for details.

> [!NOTE]
> This tool is under active development. Features and performance may evolve with future releases.

## Acknowledgments

This project is being developed in the context of the research project RENvolveIT (<https://projekte.ffg.at/projekt/5127011>).
This research was funded by CETPartnership, the Clean Energy Transition Partnership under the 2023 joint call for research proposals, co-funded by the European Commission (GA N°101069750) and with the funding organizations detailed on <https://cetpartnership.eu/funding-agencies-and-call-modules>.​

<img src="docs/assets/sponsors/CETP-logo.svg" alt="CETPartnership" width="144" height="72">  <img src="docs/assets/sponsors/EN_Co-fundedbytheEU_RGB_POS.png" alt="EU" width="180" height="40">

**3DCityDB:** For providing the foundation for 3D spatial data management. (accessed 13.11.2025, [https://www.3dcitydb.org/](https://github.com/3dcitydb))

**TABULA & EPISCOPE (IEE Projects):** building-characteristic data (accessed 13.11.2025, [https://episcope.eu/iee-project/tabula/](https://episcope.eu/iee-project/tabula/))
