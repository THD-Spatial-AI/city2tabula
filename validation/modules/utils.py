# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Utility functions for loading data from City2TABULA and CityDB databases.

This module handles:
- Loading calculated data from City2TABULA tables
- Loading thematic data from CityDB property tables
"""

import pandas as pd


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
        is_planar
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


def load_thematic_surface_data(engine, config, surface_feature_ids, attribute_mapping,
                               surface_type='RoofSurface', surface_df=None):
    """
    Load thematic surface data from CityDB property table for specified surfaces.

    Tries a direct match first (property.feature_id = surface_feature_id).  For
    surfaces with no direct hit, optionally falls back to the parent building /
    building-part level: some datasets store per-surface attributes (e.g.
    Dachneigung) once on the parent feature, implying the same value applies to
    all its surfaces.  Pass `surface_df` (the City2TABULA surface DataFrame, which
    must contain both `surface_feature_id` and `building_feature_id` columns) to
    enable this fallback.

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
    surface_df : pd.DataFrame, optional
        City2TABULA surface features with at least surface_feature_id and
        building_feature_id columns.  When provided, enables inherited fallback.

    Returns:
    --------
    pd.DataFrame : DataFrame with columns
        [feature_id, attribute_name, thematic_value, match_source]
        match_source is 'direct' or 'inherited'.
    """
    empty = pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value', 'match_source'])

    if not surface_feature_ids or not attribute_mapping:
        return empty

    citydb_schema = config['db'].get('citydb_schema', 'lod2')
    property_table = config['db']['tables'].get('citydb_property', 'property')

    source_labels = [label for label in attribute_mapping.values() if label]
    if not source_labels:
        print(f"No source labels found for {surface_type} attributes")
        return empty

    source_labels_str = ','.join(f"'{label}'" for label in source_labels)

    def _property_query(feature_ids):
        ids_str = ','.join(map(str, feature_ids))
        return f"""
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
        WHERE p.feature_id IN ({ids_str})
          AND p.name IN ({source_labels_str})
          AND (
              p.val_double IS NOT NULL
              OR (p.val_string IS NOT NULL
                  AND p.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$')
          )
        ORDER BY p.feature_id, p.name;
        """

    label_to_column = {v: k for k, v in attribute_mapping.items()}

    # --- direct match ---
    unique_surface_ids = list(set(surface_feature_ids))
    direct_raw = pd.read_sql(_property_query(unique_surface_ids), engine)
    direct_raw['attribute_name'] = direct_raw['source_label'].map(label_to_column)
    direct_df = direct_raw[['feature_id', 'attribute_name', 'thematic_value']].copy()
    direct_df['match_source'] = 'direct'
    print(f"  Direct match: {len(direct_df)} {surface_type} attribute values "
          f"across {direct_df['feature_id'].nunique()} surfaces")

    # --- inherited fallback via parent building/building-part ---
    inherited_df = pd.DataFrame()
    if surface_df is not None and not surface_df.empty:
        required = {'surface_feature_id', 'building_feature_id'}
        if not required.issubset(surface_df.columns):
            print(f"  Warning: surface_df missing columns {required - set(surface_df.columns)} "
                  f"— skipping inherited fallback")
        else:
            # Surfaces with no direct thematic data for ANY attribute
            directly_matched_ids = set(direct_df['feature_id'].unique())
            unmatched_ids = set(unique_surface_ids) - directly_matched_ids

            if unmatched_ids:
                # Build surface_id → building_id mapping (take first building per surface)
                surf_filtered = surface_df[
                    surface_df['classname'] == surface_type
                ][['surface_feature_id', 'building_feature_id']].drop_duplicates('surface_feature_id')

                unmatched_surf = surf_filtered[
                    surf_filtered['surface_feature_id'].isin(unmatched_ids)
                ]

                unique_building_ids = unmatched_surf['building_feature_id'].dropna().unique().tolist()

                if unique_building_ids:
                    parent_raw = pd.read_sql(_property_query(unique_building_ids), engine)
                    parent_raw['attribute_name'] = parent_raw['source_label'].map(label_to_column)

                    # Join parent property back to each surface
                    inherited_raw = unmatched_surf.merge(
                        parent_raw[['feature_id', 'attribute_name', 'thematic_value']],
                        left_on='building_feature_id',
                        right_on='feature_id',
                        how='inner'
                    )
                    inherited_raw = inherited_raw.rename(
                        columns={'surface_feature_id': 'feature_id'}
                    )
                    inherited_df = inherited_raw[
                        ['feature_id', 'attribute_name', 'thematic_value']
                    ].drop_duplicates(['feature_id', 'attribute_name']).copy()
                    inherited_df['match_source'] = 'inherited'
                    print(f"  Inherited fallback: {len(inherited_df)} {surface_type} attribute values "
                          f"across {inherited_df['feature_id'].nunique()} surfaces "
                          f"(parent building/part level)")

    result_df = pd.concat(
        [df for df in [direct_df, inherited_df] if not df.empty],
        ignore_index=True
    )

    total_surfaces = result_df['feature_id'].nunique()
    print(f"  Total thematic data: {len(result_df)} {surface_type} attribute values "
          f"across {total_surfaces} surfaces")

    return result_df
