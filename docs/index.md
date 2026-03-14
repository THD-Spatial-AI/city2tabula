<img src="assets/logo/svg/city2tabula_logo_complete.svg" alt="city2tabula logo" width="300" align="left"/>

[![go](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml/badge.svg)](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml)
&nbsp;
[![rtd](https://app.readthedocs.org/projects/city2tabula/badge/?version__slug=latest)](https://city2tabula.readthedocs.io/en/latest/)
&nbsp;
[![GitHub release](https://img.shields.io/github/v/release/THD-Spatial/City2TABULA.svg)](https://github.com/THD-Spatial/City2TABULA/releases)

!!! note
    This project is under active development and may change frequently.

## Overview

City2TABULA is a Go-based CLI tool for processing raw 3D city model data and generating enriched building feature data in PostgreSQL/PostGIS. It supports the preparation of building datasets for downstream energy-modelling, research, and classification workflows.

The tool is designed for scalable processing of LoD2 and LoD3 building data stored through CityDB-based database structures. It automates key steps such as data setup, import, and feature extraction within a single pipeline.

![City2TABULA System Context](assets/diagrams/architecture/system-context/system-context.svg)

## Key Capabilities

- CLI-driven pipeline for 3D city model processing
- Support for CityGML and CityJSON input data
- PostgreSQL/PostGIS integration for persistent storage and analysis
- Feature extraction from LoD2 and LoD3 building data
- Scalable batch-based and parallel processing
- Outputs for downstream research and energy-modelling workflows

## Documentation Guide

To get started, see the installation and setup section.
For a deeper technical understanding, refer to the architecture and processing workflow pages.
For practical usage, see the CLI and pipeline documentation.

## Support

If you encounter a problem or would like to suggest an improvement, please open an issue in the project repository.