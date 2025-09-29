# City2TABULA Data Directory

This directory contains the input data required for the City2TABULA building classification pipeline. The tool processes 3D building data from CityDB schemas and additional reference datasets.

## 📁 Directory Structure

```
data/
├── README.md              # This file - overview of all data directories
├── lod2/                  # Level of Detail 2 building data
│   ├── .gitignore         # Excludes large data files from git
│   └── lks_deggendorf.meta4 # Metadata file for Deggendorf city LoD2 data
├── lod3/                  # Level of Detail 3 building data
│   ├── .gitignore         # Excludes large data files from git
│   └── hamburg/           # Example: Hamburg city LoD3 data
├── postcode/              # Postcode/postal code reference data
└── tabula/                # TABULA building type reference data
```

## Purpose

Each data directory serves a specific purpose in the building classification pipeline:

- **LoD2/LoD3**: 3D building geometries from CityDB schemas
- **Postcode**: Geographic postal code boundaries for spatial joining
- **TABULA**: Reference building type classifications for training

## Data Requirements

### **Before Running City2TABULA:**

1. **Database Setup**: Import your CityDB LoD2/LoD3 data into PostgreSQL with PostGIS
2. **Data Placement**: Place additional reference data in appropriate directories
3. **Configuration**: Update `.env` file with database connection details
4. **Execution**: Run the binary from the same directory as the data folder

### **Data Sources:**

- **CityDB Data**: Export from 3D city models (CityGML format)
- **Postcode Data**: National postal services or OpenStreetMap
- **TABULA Data**: European building typology databases

## 🔧 Usage

The City2TABULA tool expects this data directory to be in the same location as the executable:

```
your-working-directory/
├── City2TABULA-*           # The binary
├── .env                   # Configuration file
├── data/                  # This directory
│   ├── lod2/
│   ├── lod3/
│   ├── postcode/
│   └── tabula/
└── logs/                  # Generated log files
```

## More Information

For detailed information about each data type, see the README.md files in each subdirectory:

- [LoD2 Data Requirements](lod2/README.md)
- [LoD3 Data Requirements](lod3/README.md)
- [Postcode Data Requirements](postcode/README.md)
- [TABULA Data Requirements](tabula/README.md)

## Important Notes

- **Large Files**: Data files are excluded from git via `.gitignore`
- **Permissions**: Ensure the tool has read access to all data directories
- **Paths**: Use relative paths in configuration - the tool expects `data/` in the working directory
- **Formats**: See individual README files for specific format requirements
