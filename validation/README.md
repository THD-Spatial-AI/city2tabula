# City2TABULA Validation Module

Validation framework for comparing City2TABULA calculated building attributes against source CityGML thematic data.

## Overview

This module validates City2TABULA pipeline outputs by:
1. Loading calculated building/surface attributes from City2TABULA tables
2. Fetching corresponding thematic data from CityDB property tables
3. Comparing calculated vs thematic values
4. Generating statistical metrics and visualizations
5. Exporting validation reports

## Architecture

```
validation/
├── configs/              # Country-specific YAML configurations
│   ├── config_germany.yaml
│   └── config_example.yaml
├── modules/              # Python validation modules
│   ├── config.py         # Configuration loading and parsing
│   ├── db.py             # Database connection management
│   ├── utils.py          # Data loading utilities
│   ├── validators.py     # Validation and comparison logic
│   ├── metrics.py        # Statistical calculations
│   └── plots.py          # Visualization functions
├── outputs/              # Validation results and plots
├── validation.ipynb      # Main Jupyter notebook workflow
├── README.md             # This documentation file
└── requirements.txt      # Python dependencies
```

## Quick Start

### 1. Environment Setup

**Option A: Conda (Recommended)**
```bash
conda create -n city2tabula-validation python=3.13
conda activate city2tabula-validation
pip install -r validation/requirements.txt
python -m ipykernel install --user --name=city2tabula-validation
```

**Option B: Virtual Environment**
```bash
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r validation/requirements.txt
```

### 2. Database Configuration

Create `.env` file in project root:
```bash
DB_USER=your_username
DB_PASSWORD=your_password
DB_HOST=localhost
DB_PORT=5432
COUNTRY=germany
```

### 3. Country Configuration

Edit `configs/config_germany.yaml` to map source thematic labels to City2TABULA columns:

```yaml
attributes:
  parent:
    footprint_area:
      computed_column: "footprint_area"
      source_label: "Flaeche"          # German: "Area"
      unit: "m²"

  child:
    roof:
      area:
        computed_column: "surface_area"
        source_label: "Flaeche"
      tilt:
        computed_column: "tilt"
        source_label: "Dachneigung"    # German: "Roof slope"
```

### 4. Run Validation

Open `validation.ipynb` in VS Code or Jupyter:
```bash
jupyter notebook validation.ipynb
```

Execute cells sequentially to:
1. Load configuration and connect to database
2. Load City2TABULA calculated data
3. Fetch thematic data from CityDB
4. Validate building and surface attributes
5. Generate metrics and visualizations
6. Export results to `outputs/`

## 📚 Module Documentation

### `config.py` - Configuration Management

**Key Functions:**
- `load_config(config_path)` - Load YAML configuration with database connection string
- `get_building_attribute_mapping(config)` - Extract building attribute mappings
- `get_surface_attribute_mapping(config, surface_type)` - Extract surface mappings (roof/wall/floor)
- `print_config_summary(config)` - Display configuration overview

**Example:**
```python
from modules.config import load_config, get_building_attribute_mapping

config = load_config('configs/config_germany.yaml')
building_attrs = get_building_attribute_mapping(config)
# Returns: {'footprint_area': 'Flaeche', 'min_height': 'value', ...}
```

### `db.py` - Database Connections

**Key Functions:**
- `get_db_engine(config)` - Create SQLAlchemy engine from config
- `close_db_engine(engine)` - Dispose database engine

**Example:**
```python
from modules.db import get_db_engine

engine = get_db_engine(config)
# Use engine for queries...
close_db_engine(engine)
```

### `utils.py` - Data Loading

**Key Functions:**
- `load_city2tabula_data(engine, config)` - Load calculated building/surface data
- `load_thematic_building_data(engine, config, building_ids, attr_map)` - Fetch building thematic data
- `load_thematic_surface_data(engine, config, surface_ids, attr_map, surface_type)` - Fetch surface thematic data

**Example:**
```python
from modules.utils import load_city2tabula_data, load_thematic_building_data

# Load calculated data
buildings_df, surfaces_df = load_city2tabula_data(engine, config)

# Load thematic data
building_ids = buildings_df['building_feature_id'].tolist()
thematic_df = load_thematic_building_data(engine, config, building_ids, building_attrs)
```

### `validators.py` - Validation Logic

**Key Functions:**
- `validate_building_attributes(calc_df, thematic_df, attr_map)` - Compare building attributes
- `validate_surface_attributes(calc_df, thematic_df, attr_map, surface_type)` - Compare surface attributes
- `get_validation_summary(validation_df)` - Generate statistical summary

**Returns:**
Validation DataFrame with columns:
- `feature_id` - Building or surface ID
- `attribute_name` - Attribute being validated
- `calculated_value` - City2TABULA computed value
- `thematic_value` - CityDB source value
- `difference` - calculated - thematic
- `percent_error` - (difference / thematic) * 100

**Example:**
```python
from modules.validators import validate_building_attributes, get_validation_summary

validation_df = validate_building_attributes(buildings_df, thematic_df, building_attrs)
summary = get_validation_summary(validation_df)
print(summary)
```

### `metrics.py` - Statistical Analysis

**Key Functions:**
- `calculate_rmse(validation_df, attribute_name)` - Root Mean Square Error
- `calculate_mae(validation_df, attribute_name)` - Mean Absolute Error
- `calculate_r_squared(validation_df, attribute_name)` - Coefficient of determination
- `check_tolerance(validation_df, config)` - Check if within tolerance thresholds
- `get_outliers(validation_df, method='iqr')` - Detect outliers

**Example:**
```python
from modules.metrics import calculate_rmse, check_tolerance

rmse = calculate_rmse(validation_df, 'footprint_area')
validation_df_checked = check_tolerance(validation_df, config, tolerance_type='percent')
```

### `plots.py` - Visualization

**Key Functions:**
- `plot_comparison_scatter(validation_df, attr_name, save_path)` - Scatter plot (calculated vs thematic)
- `plot_error_distribution(validation_df, attr_name)` - Error histogram & box plot
- `plot_percent_error_distribution(validation_df, attr_name)` - Percentage error histogram
- `plot_multi_attribute_comparison(validation_df)` - Multi-attribute grid plot

**Example:**
```python
from modules.plots import plot_comparison_scatter, plot_error_distribution

fig1 = plot_comparison_scatter(validation_df, 'footprint_area',
                                save_path='outputs/footprint_scatter.png')
fig2 = plot_error_distribution(validation_df, 'footprint_area')
```

## Workflow Example

Complete validation workflow:

```python
# 1. Setup
from modules.config import load_config, get_building_attribute_mapping
from modules.db import get_db_engine
from modules.utils import load_city2tabula_data, load_thematic_building_data
from modules.validators import validate_building_attributes, get_validation_summary
from modules.plots import plot_comparison_scatter

config = load_config('configs/config_germany.yaml')
engine = get_db_engine(config)

# 2. Load data
buildings_df, surfaces_df = load_city2tabula_data(engine, config)
building_attrs = get_building_attribute_mapping(config)

# 3. Fetch thematic data
building_ids = buildings_df['building_feature_id'].tolist()
thematic_df = load_thematic_building_data(engine, config, building_ids, building_attrs)

# 4. Validate
validation_df = validate_building_attributes(buildings_df, thematic_df, building_attrs)
summary = get_validation_summary(validation_df)

# 5. Visualize
plot_comparison_scatter(validation_df, 'footprint_area', save_path='outputs/footprint.png')

# 6. Export
validation_df.to_csv('outputs/building_validation.csv', index=False)
summary.to_csv('outputs/building_summary.csv', index=False)
```

## Configuration Schema

### Database Section
```yaml
db:
  city2tabula_schema: "city2tabula"      # Calculated data schema
  citydb_schema: "lod2"                  # Source data schema (lod2/lod3)
  tables:
    citydb_property: "property"          # CityDB property table
    building_feature: "building_feature" # Building table (no prefix)
    child_feature_surface: "child_feature_surface"  # Surface table (no prefix)
```

### Attribute Mapping
```yaml
attributes:
  parent:  # Building-level
    <attribute_name>:
      computed_column: "<city2tabula_column>"
      source_label: "<citydb_property_name>"
      unit: "<measurement_unit>"

  child:   # Surface-level
    roof:  # RoofSurface
      <attribute_name>:
        computed_column: "<city2tabula_column>"
        source_label: "<citydb_property_name>"
    wall:  # WallSurface
    floor: # GroundSurface
```

### Validation Tolerances
```yaml
validation:
  tolerance:
    abs:
      height: 0.5      # ±0.5 m
      tilt: 2.0        # ±2.0°
    percent:
      footprint_area: 5.0   # ±5%
      surface_area: 5.0     # ±5%
```

## Output Files

Validation outputs saved to `outputs/validation_<timestamp>/`:
- `building_validation.csv` - Detailed building validation results
- `building_summary.csv` - Statistical summary for building attributes
- `roof_validation.csv` - Detailed roof surface validation results
- `roof_summary.csv` - Statistical summary for roof attributes
- `*.png` - Visualization plots


## License

Part of City2TABULA project. See main repository [LICENSE](/LICENSE) file