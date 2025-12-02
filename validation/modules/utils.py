"""
Utility functions for loading data from City2TABULA and CityDB databases.

This module handles:
- Loading calculated data from City2TABULA tables
- Loading thematic data from CityDB property tables
"""

import pandas as pd
from sqlalchemy import text


def load_city2tabula_data(engine, config):
    """
    Load calculated building and surface data from City2TABULA database.

    Parameters:
    -----------
    engine : sqlalchemy.Engine
        The SQLAlchemy engine connected to the database.
    config : dict
        Configuration dictionary containing database schema and table names.

    Returns:
    --------
    tuple : (building_features_df, surface_features_df)
        - building_features_df: DataFrame with building-level calculated data
        - surface_features_df: DataFrame with surface-level calculated data
    """
    # Get database schema names from config
    city2tabula_schema = config['db'].get('city2tabula_schema', 'city2tabula')
    citydb_schema = config['db'].get('citydb_schema', 'lod2')

    # Get table names from config
    tables = config['db'].get('tables', {})
    building_table_base = tables.get('building_feature', 'building_feature')
    surface_table_base = tables.get('child_feature_surface', 'child_feature_surface')

    # Construct full table names: {schema}_{table}
    building_table = f"{citydb_schema}_{building_table_base}"
    surface_table = f"{citydb_schema}_{surface_table_base}"

    # Load building features
    print(f"Loading building features from {city2tabula_schema}.{building_table}...")
    query_buildings = f"SELECT * FROM {city2tabula_schema}.{building_table};"
    building_features_df = pd.read_sql(query_buildings, engine)
    print(f"Loaded {len(building_features_df)} buildings")

    # Load surface features with geometry
    print(f"Loading surface features from {city2tabula_schema}.{surface_table}...")
    query_surfaces = f"""
    SELECT
        surface_feature_id,
        building_feature_id,
        objectclass_id,
        classname,
        surface_area,
        tilt,
        azimuth,
        is_valid,
        is_planar,
        ST_AsText(geom) as geom
    FROM {city2tabula_schema}.{surface_table};
    """
    surface_features_df = pd.read_sql(query_surfaces, engine)
    print(f"Loaded {len(surface_features_df)} surfaces")

    return building_features_df, surface_features_df


def load_thematic_building_data(engine, config, building_feature_ids, attribute_mapping):
    """
    Load thematic building data from CityDB property table for specified buildings.

    Parameters:
    -----------
    engine : sqlalchemy.Engine
        Database connection engine
    config : dict
        Configuration dictionary
    building_feature_ids : list
        List of building feature IDs to fetch thematic data for
    attribute_mapping : dict
        Dictionary mapping computed columns to source property labels
        e.g., {'min_height': 'value', 'footprint_area': 'Flaeche'}

    Returns:
    --------
    pd.DataFrame : DataFrame with columns [feature_id, attribute_name, thematic_value]
    """
    if not building_feature_ids or not attribute_mapping:
        return pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])

    # Get config
    citydb_schema = config['db'].get('citydb_schema', 'lod2')
    property_table = config['db']['tables'].get('citydb_property', 'property')

    # Get source labels (filter out empty strings)
    source_labels = [label for label in attribute_mapping.values() if label]

    if not source_labels:
        print("No source labels found for building attributes")
        return pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])

    # Create placeholders for SQL IN clause
    feature_ids_str = ','.join(map(str, building_feature_ids))
    source_labels_str = ','.join(f"'{label}'" for label in source_labels)

    # Query thematic data
    # Try both val_double and val_string columns, converting strings to numeric
    query = f"""
    SELECT
        p.feature_id,
        p.name AS source_label,
        COALESCE(
            p.val_double,
            CASE
                WHEN p.val_string IS NOT NULL
                THEN
                    CASE
                        WHEN p.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$'
                        THEN p.val_string::numeric
                        ELSE NULL
                    END
                ELSE NULL
            END
        ) AS thematic_value
    FROM {citydb_schema}.{property_table} AS p
    WHERE p.feature_id IN ({feature_ids_str})
      AND p.name IN ({source_labels_str})
      AND (
          p.val_double IS NOT NULL
          OR (p.val_string IS NOT NULL AND p.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$')
      )
    ORDER BY p.feature_id, p.name;
    """

    thematic_df = pd.read_sql(query, engine)

    # Create reverse mapping: source_label -> list of computed_columns
    # This handles cases where multiple attributes map to the same source label
    # (e.g., min_height and max_height both use 'value')
    label_to_columns = {}
    for computed_col, source_label in attribute_mapping.items():
        if source_label not in label_to_columns:
            label_to_columns[source_label] = []
        label_to_columns[source_label].append(computed_col)

    # Expand rows for each attribute that uses the same source label
    expanded_rows = []
    for _, row in thematic_df.iterrows():
        source_label = row['source_label']
        if source_label in label_to_columns:
            for computed_col in label_to_columns[source_label]:
                expanded_rows.append({
                    'feature_id': row['feature_id'],
                    'attribute_name': computed_col,
                    'thematic_value': row['thematic_value']
                })

    result_df = pd.DataFrame(expanded_rows)

    print(f"Loaded thematic data for {len(result_df)} building attribute values")

    return result_df


def load_thematic_surface_data(engine, config, surface_feature_ids, attribute_mapping, surface_type='RoofSurface'):
    """
    Load thematic surface data from CityDB property table for specified surfaces.

    Parameters:
    -----------
    engine : sqlalchemy.Engine
        Database connection engine
    config : dict
        Configuration dictionary
    surface_feature_ids : list
        List of surface feature IDs to fetch thematic data for
    attribute_mapping : dict
        Dictionary mapping computed columns to source property labels
        e.g., {'surface_area': 'Flaeche', 'tilt': 'Dachneigung'}
    surface_type : str
        Surface type classname (e.g., 'RoofSurface', 'WallSurface', 'GroundSurface')

    Returns:
    --------
    pd.DataFrame : DataFrame with columns [feature_id, attribute_name, thematic_value]
    """
    if not surface_feature_ids or not attribute_mapping:
        return pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])

    # Get config
    citydb_schema = config['db'].get('citydb_schema', 'lod2')
    property_table = config['db']['tables'].get('citydb_property', 'property')

    # Get source labels (filter out empty strings)
    source_labels = [label for label in attribute_mapping.values() if label]

    if not source_labels:
        print(f"No source labels found for {surface_type} attributes")
        return pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])

    # Create placeholders for SQL IN clause
    feature_ids_str = ','.join(map(str, surface_feature_ids))
    source_labels_str = ','.join(f"'{label}'" for label in source_labels)

    # Query thematic data (same as building query)
    query = f"""
    SELECT
        p.feature_id,
        p.name AS source_label,
        COALESCE(
            p.val_double,
            CASE
                WHEN p.val_string IS NOT NULL
                THEN
                    CASE
                        WHEN p.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$'
                        THEN p.val_string::numeric
                        ELSE NULL
                    END
                ELSE NULL
            END
        ) AS thematic_value
    FROM {citydb_schema}.{property_table} AS p
    WHERE p.feature_id IN ({feature_ids_str})
      AND p.name IN ({source_labels_str})
      AND (
          p.val_double IS NOT NULL
          OR (p.val_string IS NOT NULL AND p.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$')
      )
    ORDER BY p.feature_id, p.name;
    """

    thematic_df = pd.read_sql(query, engine)

    # Create reverse mapping: source_label -> computed_column
    label_to_column = {v: k for k, v in attribute_mapping.items()}

    # Map source labels back to attribute names
    thematic_df['attribute_name'] = thematic_df['source_label'].map(label_to_column)

    # Keep only needed columns
    result_df = thematic_df[['feature_id', 'attribute_name', 'thematic_value']].copy()

    print(f"Loaded thematic data for {len(result_df)} {surface_type} attribute values")

    return result_df
