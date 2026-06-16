# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Utility functions for loading data from City2TABULA and CityDB databases.

Loading strategies
------------------
Buildings  (City2TABULA → CityDB):
    Sample N building IDs from City2TABULA, then fetch their thematic
    properties from CityDB.  Some buildings may have no thematic data
    and are simply excluded from the result.

Surfaces   (CityDB → City2TABULA, reversed):
    Start from CityDB property table: find surface IDs that have the
    target thematic attribute AND exist in City2TABULA with the right
    classname.  Sample N from that intersection.  Because we start from
    the source, N matches are guaranteed (up to total available).
"""

import pandas as pd


# ── Helpers ───────────────────────────────────────────────────────────────────

def _numeric_coalesce(alias: str = 'p') -> str:
    """COALESCE across val_double, val_int, and numeric-castable val_string."""
    return f"""COALESCE(
            {alias}.val_double,
            {alias}.val_int::numeric,
            CASE
                WHEN {alias}.val_string IS NOT NULL
                 AND {alias}.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$'
                THEN {alias}.val_string::numeric
                ELSE NULL
            END
        )"""


def _has_numeric_value(alias: str = 'p') -> str:
    """WHERE fragment: row has at least one non-null numeric value column."""
    return f"""(
          {alias}.val_double IS NOT NULL
          OR {alias}.val_int IS NOT NULL
          OR ({alias}.val_string IS NOT NULL
              AND {alias}.val_string ~ '^[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?$')
      )"""


# ── City2TABULA data ──────────────────────────────────────────────────────────

def load_city2tabula_data(engine, config):
    """
    Load calculated building and surface data from City2TABULA database.

    Returns
    -------
    tuple : (building_features_df, surface_features_df)
    """
    city2tabula_schema = config['db'].get('city2tabula_schema', 'city2tabula')
    citydb_schema      = config['db'].get('citydb_schema', 'lod2')
    tables             = config['db'].get('tables', {})
    building_table     = f"{citydb_schema}_{tables.get('building_feature', 'building_feature')}"
    surface_table      = f"{citydb_schema}_{tables.get('child_feature_surface', 'child_feature_surface')}"

    print(f"Loading building features from {city2tabula_schema}.{building_table}...")
    building_features_df = pd.read_sql(
        f"SELECT * FROM {city2tabula_schema}.{building_table};", engine)
    print(f"Loaded {len(building_features_df)} buildings")

    print(f"Loading surface features from {city2tabula_schema}.{surface_table}...")
    surface_features_df = pd.read_sql(f"""
        SELECT surface_feature_id, building_feature_id, objectclass_id, classname,
               surface_area, tilt, azimuth, is_valid, is_planar
        FROM {city2tabula_schema}.{surface_table};
    """, engine)
    print(f"Loaded {len(surface_features_df)} surfaces")

    return building_features_df, surface_features_df


# ── Thematic data loaders ─────────────────────────────────────────────────────

def load_thematic_building_data(engine, config, attribute_mapping, limit=0):
    """
    Load thematic building data.  Direction: City2TABULA → CityDB.

    Samples N building IDs from City2TABULA, then fetches their thematic
    properties from CityDB.  Buildings with no matching thematic data are
    excluded from the result.

    Parameters
    ----------
    attribute_mapping : dict
        ``{computed_column: source_label}`` — multiple computed columns
        may share the same source label (e.g. min_height and max_height
        both mapped to 'value').
    limit : int
        Number of building IDs to sample (0 = all buildings).

    Returns
    -------
    pd.DataFrame
        Columns: feature_id, attribute_name, thematic_value
    """
    empty = pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])
    if not attribute_mapping:
        return empty

    citydb_schema      = config['db'].get('citydb_schema', 'lod2')
    city2tabula_schema = config['db'].get('city2tabula_schema', 'city2tabula')
    property_table     = config['db']['tables'].get('citydb_property', 'property')
    building_table     = (f"{citydb_schema}_"
                          f"{config['db']['tables'].get('building_feature', 'building_feature')}")

    source_labels = [lbl for lbl in attribute_mapping.values() if lbl]
    if not source_labels:
        print("No source labels configured for building attributes")
        return empty
    source_labels_str = ', '.join(f"'{lbl}'" for lbl in source_labels)

    limit_clause = f"LIMIT {limit}" if limit > 0 else ""

    query = f"""
    WITH sampled_buildings AS (
        SELECT building_feature_id
        FROM   {city2tabula_schema}.{building_table}
        ORDER  BY RANDOM()
        {limit_clause}
    )
    SELECT
        p.feature_id,
        p.name                   AS source_label,
        {_numeric_coalesce()}    AS thematic_value
    FROM   sampled_buildings b
    JOIN   {citydb_schema}.{property_table} p
           ON p.feature_id = b.building_feature_id
    WHERE  p.name IN ({source_labels_str})
      AND  {_has_numeric_value()}
    ORDER  BY p.feature_id, p.name;
    """

    raw_df = pd.read_sql(query, engine)

    if raw_df.empty:
        print("No thematic data found for building attributes")
        return empty

    # Multiple computed columns may share the same source label.
    # Expand: one raw row → one result row per mapped computed column.
    label_to_columns: dict = {}
    for computed_col, source_label in attribute_mapping.items():
        if source_label:
            label_to_columns.setdefault(source_label, []).append(computed_col)

    rows = []
    for _, row in raw_df.iterrows():
        for computed_col in label_to_columns.get(row['source_label'], []):
            rows.append({
                'feature_id':     row['feature_id'],
                'attribute_name': computed_col,
                'thematic_value': row['thematic_value'],
            })

    result_df = pd.DataFrame(rows)
    print(f"Loaded thematic data for {result_df['feature_id'].nunique()} buildings "
          f"({len(result_df)} attribute values)")
    return result_df


def load_thematic_surface_data(engine, config, attribute_mapping,
                               surface_type='RoofSurface', limit=0):
    """
    Load thematic surface data.  Direction: CityDB -> City2TABULA (reversed).

    Each attribute is sampled **independently**: N surfaces with tilt, N surfaces
    with azimuth, etc.  This ensures N comparisons per attribute even when not all
    surfaces carry every attribute (e.g. some roofs have tilt but no azimuth).

    For each source label the query:
      1. Finds surface IDs that have that label in CityDB AND exist in City2TABULA
         with the correct classname  (the eligible set).
      2. Draws a random sample of N from that set.
      3. Fetches the thematic values for those N surfaces.

    Only direct matches are used: property.feature_id = surface_feature_id.
    Surfaces whose thematic values are stored at the building/parent level
    (e.g. BW Dachneigung on BuildingPart) are excluded -- direct match only.

    Parameters
    ----------
    attribute_mapping : dict
        ``{computed_column: source_label}``
    surface_type : str
        'RoofSurface' | 'WallSurface' | 'GroundSurface'
    limit : int
        Number of surface IDs to sample per attribute (0 = all eligible).

    Returns
    -------
    pd.DataFrame
        Columns: feature_id, attribute_name, thematic_value
    """
    empty = pd.DataFrame(columns=['feature_id', 'attribute_name', 'thematic_value'])
    if not attribute_mapping:
        return empty

    citydb_schema      = config['db'].get('citydb_schema', 'lod2')
    city2tabula_schema = config['db'].get('city2tabula_schema', 'city2tabula')
    property_table     = config['db']['tables'].get('citydb_property', 'property')
    surface_table      = (f"{citydb_schema}_"
                          f"{config['db']['tables'].get('child_feature_surface', 'child_feature_surface')}")

    # Group computed columns by source label — one query per label so that
    # each attribute gets its own independent random sample of N surfaces.
    label_to_columns: dict = {}
    for computed_col, source_label in attribute_mapping.items():
        if source_label:
            label_to_columns.setdefault(source_label, []).append(computed_col)

    if not label_to_columns:
        print(f"No source labels configured for {surface_type} attributes")
        return empty

    limit_clause = f"LIMIT {limit}" if limit > 0 else ""
    parts = []

    for source_label, computed_cols in label_to_columns.items():
        sl_sql = source_label.replace("'", "''")   # escape for SQL literal

        # Azimuth uses -1 as a sentinel for flat/undefined roofs.
        # Filter BOTH the thematic value and the calculated column so that the
        # eligible pool only contains surfaces where both sides are meaningful.
        # This must be restricted to azimuth only — for area, height, tilt etc.
        # -1 is not a sentinel and applying != -1 would silently drop surfaces
        # with NULL calculated values (NULL != -1 evaluates to NULL in SQL).
        azimuth_cols = [col for col in computed_cols if col == 'azimuth']
        calc_filters = " ".join(f"AND sf.{col} != -1" for col in azimuth_cols)

        thematic_sentinel_filter = (
            f"AND {_numeric_coalesce()} != -1" if azimuth_cols else ""
        )

        query = f"""
        WITH eligible AS (
            SELECT DISTINCT p.feature_id
            FROM   {citydb_schema}.{property_table} p
            INNER  JOIN {city2tabula_schema}.{surface_table} sf
                   ON  sf.surface_feature_id = p.feature_id
                   AND sf.classname = '{surface_type}'
                   {calc_filters}
            WHERE  p.name = '{sl_sql}'
              AND  {_has_numeric_value()}
              {thematic_sentinel_filter}
        ),
        sampled AS (
            SELECT feature_id
            FROM   eligible
            ORDER  BY RANDOM()
            {limit_clause}
        )
        SELECT
            p.feature_id,
            {_numeric_coalesce()} AS thematic_value
        FROM   sampled s
        JOIN   {citydb_schema}.{property_table} p ON p.feature_id = s.feature_id
        WHERE  p.name = '{sl_sql}'
          AND  {_has_numeric_value()}
        ORDER  BY p.feature_id;
        """

        raw = pd.read_sql(query, engine)
        if raw.empty:
            print(f"  {surface_type}: no data for '{source_label}'")
            continue

        n_surfaces = raw['feature_id'].nunique()
        for computed_col in computed_cols:
            part = raw[['feature_id', 'thematic_value']].copy()
            part['attribute_name'] = computed_col
            parts.append(part[['feature_id', 'attribute_name', 'thematic_value']])
            print(f"  {surface_type} {computed_col}: {n_surfaces} surfaces")

    if not parts:
        print(f"No thematic data found for {surface_type}")
        return empty

    result_df = pd.concat(parts, ignore_index=True)
    print(f"Loaded {len(result_df)} {surface_type} attribute values "
          f"across {result_df['feature_id'].nunique()} unique surfaces")
    return result_df
