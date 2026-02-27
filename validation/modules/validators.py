# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Validators module for comparing City2TABULA calculated values with thematic data.

This module handles:
- Comparing building-level attributes (footprint_area, heights)
- Comparing surface-level attributes (area, tilt, azimuth)
- Calculating differences and percentage errors
- Filtering and validating specific surface types
"""

import pandas as pd
import numpy as np


def validate_building_attributes(building_calc_df, building_thematic_df, attribute_mapping):
    """
    Compare calculated building attributes with thematic values from CityDB.

    Parameters:
    -----------
    building_calc_df : pd.DataFrame
        DataFrame with calculated building data from City2TABULA
        Must have columns: building_feature_id, and computed columns from config
    building_thematic_df : pd.DataFrame
        DataFrame with thematic building data from CityDB property table
        Must have columns: feature_id, attribute_name, thematic_value
    attribute_mapping : dict
        Dictionary mapping computed columns to source labels
        e.g., {'footprint_area': 'Flaeche', 'min_height': 'value'}

    Returns:
    --------
    pd.DataFrame : Validation results with columns:
        - building_feature_id: Building ID
        - attribute_name: Name of the attribute being validated
        - calculated_value: Value computed by City2TABULA
        - thematic_value: Value from CityDB thematic data
        - difference: calculated_value - thematic_value
        - percent_error: (difference / thematic_value) * 100
        - is_valid: Boolean indicating if difference is within tolerance
    """
    if building_calc_df.empty or building_thematic_df.empty:
        print("Warning: Empty input dataframes for building validation")
        return pd.DataFrame()

    results = []

    # Process each attribute
    for computed_column, source_label in attribute_mapping.items():
        # Skip if no source label defined
        if not source_label:
            continue

        # Filter thematic data for this attribute
        attr_thematic = building_thematic_df[
            building_thematic_df['attribute_name'] == computed_column
        ].copy()

        if attr_thematic.empty:
            print(f"Warning: No thematic data found for attribute '{computed_column}'")
            continue

        # Merge calculated and thematic data on building_feature_id
        merged = building_calc_df[['building_feature_id', computed_column]].merge(
            attr_thematic[['feature_id', 'thematic_value']],
            left_on='building_feature_id',
            right_on='feature_id',
            how='inner'
        )

        # Calculate differences and errors
        merged['attribute_name'] = computed_column
        merged['calculated_value'] = merged[computed_column]
        merged['difference'] = merged['calculated_value'] - merged['thematic_value']

        # Calculate percentage error (handle division by zero)
        merged['percent_error'] = np.where(
            merged['thematic_value'] != 0,
            (merged['difference'] / merged['thematic_value']) * 100,
            np.nan
        )

        # Keep only needed columns
        result_cols = [
            'building_feature_id', 'attribute_name',
            'calculated_value', 'thematic_value',
            'difference', 'percent_error'
        ]
        results.append(merged[result_cols])

    if not results:
        print("Warning: No validation results generated for building attributes")
        return pd.DataFrame()

    # Combine all attribute results
    validation_df = pd.concat(results, ignore_index=True)

    print(f"Validated {len(validation_df)} building attribute values across {validation_df['building_feature_id'].nunique()} buildings")

    return validation_df


def validate_surface_attributes(surface_calc_df, surface_thematic_df, attribute_mapping, surface_type='RoofSurface'):
    """
    Compare calculated surface attributes with thematic values from CityDB.

    Parameters:
    -----------
    surface_calc_df : pd.DataFrame
        DataFrame with calculated surface data from City2TABULA
        Must have columns: surface_feature_id, building_feature_id, classname, geom, and computed columns
    surface_thematic_df : pd.DataFrame
        DataFrame with thematic surface data from CityDB property table
        Must have columns: feature_id, attribute_name, thematic_value
    attribute_mapping : dict
        Dictionary mapping computed columns to source labels
        e.g., {'surface_area': 'Flaeche', 'tilt': 'Dachneigung', 'azimuth': 'Dachorientierung'}
    surface_type : str
        Surface type to filter (e.g., 'RoofSurface', 'WallSurface', 'GroundSurface')

    Returns:
    --------
    pd.DataFrame : Validation results with columns:
        - building_feature_id: Building ID
        - surface_feature_id: Surface ID
        - classname: Surface type
        - attribute_name: Name of the attribute being validated
        - calculated_value: Value computed by City2TABULA
        - thematic_value: Value from CityDB thematic data
        - difference: calculated_value - thematic_value
        - percent_error: (difference / thematic_value) * 100
        - geom: Surface geometry (if available)
    """
    if surface_calc_df.empty or surface_thematic_df.empty:
        print(f"Warning: Empty input dataframes for {surface_type} validation")
        return pd.DataFrame()

    # Filter surfaces by type
    filtered_calc = surface_calc_df[surface_calc_df['classname'] == surface_type].copy()

    if filtered_calc.empty:
        print(f"Warning: No surfaces found with classname '{surface_type}'")
        return pd.DataFrame()

    results = []

    # Determine which columns to keep (include geom if it exists)
    base_cols = ['surface_feature_id', 'building_feature_id', 'classname']
    if 'geom' in filtered_calc.columns:
        base_cols.append('geom')

    # Process each attribute
    for computed_column, source_label in attribute_mapping.items():
        # Skip if no source label defined
        if not source_label:
            continue

        # Skip if column doesn't exist in calculated data
        if computed_column not in filtered_calc.columns:
            print(f"Warning: Column '{computed_column}' not found in calculated data")
            continue

        # Filter thematic data for this attribute
        attr_thematic = surface_thematic_df[
            surface_thematic_df['attribute_name'] == computed_column
        ].copy()

        if attr_thematic.empty:
            print(f"Warning: No thematic data found for attribute '{computed_column}' in {surface_type}")
            continue

        # Select columns to merge
        merge_cols = base_cols + [computed_column]

        # Merge calculated and thematic data on surface_feature_id
        merged = filtered_calc[merge_cols].merge(
            attr_thematic[['feature_id', 'thematic_value']],
            left_on='surface_feature_id',
            right_on='feature_id',
            how='inner'
        )

        # Special handling for azimuth: exclude -1 values (undefined for flat roofs)
        if computed_column == 'azimuth':
            before_count = len(merged)
            merged = merged[(merged[computed_column] != -1) & (merged['thematic_value'] != -1)].copy()
            excluded_count = before_count - len(merged)
            if excluded_count > 0:
                print(f"  Excluded {excluded_count} surfaces with azimuth = -1 (flat roofs/undefined)")

        # Calculate differences and errors
        merged['attribute_name'] = computed_column
        merged['calculated_value'] = merged[computed_column]
        merged['difference'] = merged['calculated_value'] - merged['thematic_value']

        # Calculate percentage error (handle division by zero)
        merged['percent_error'] = np.where(
            merged['thematic_value'] != 0,
            (merged['difference'] / merged['thematic_value']) * 100,
            np.nan
        )

        # Keep only needed columns
        result_cols = base_cols + [
            'attribute_name',
            'calculated_value', 'thematic_value',
            'difference', 'percent_error'
        ]
        results.append(merged[result_cols])

    if not results:
        print(f"Warning: No validation results generated for {surface_type} attributes")
        return pd.DataFrame()

    # Combine all attribute results
    validation_df = pd.concat(results, ignore_index=True)

    print(f"Validated {len(validation_df)} {surface_type} attribute values across {validation_df['surface_feature_id'].nunique()} surfaces")

    return validation_df


def get_validation_summary(validation_df):
    """
    Generate summary statistics for validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validate_building_attributes or validate_surface_attributes

    Returns:
    --------
    pd.DataFrame : Summary statistics grouped by attribute_name with columns:
        - attribute_name: Name of the attribute
        - count: Number of comparisons
        - mean_difference: Average difference
        - std_difference: Standard deviation of differences
        - mean_percent_error: Average percentage error
        - median_percent_error: Median percentage error
        - rmse: Root mean square error
    """
    if validation_df.empty:
        print("Warning: Empty validation dataframe")
        return pd.DataFrame()

    summary = validation_df.groupby('attribute_name').agg({
        'difference': ['count', 'mean', 'std', lambda x: np.sqrt((x**2).mean())],  # RMSE
        'percent_error': ['mean', 'median', 'std']
    }).reset_index()

    # Flatten column names
    summary.columns = [
        'attribute_name', 'count',
        'mean_difference', 'std_difference', 'rmse',
        'mean_percent_error', 'median_percent_error', 'std_percent_error'
    ]

    # Round to 4 decimal places
    summary = summary.round(4)

    return summary


def export_problematic_surfaces(validation_df, output_path, error_threshold=10.0):
    """
    Export surfaces with validation errors above threshold.

    Simply filters the validation DataFrame for high errors and exports to CSV.
    The CSV includes building_id, surface_id, geometry, and all validation metrics.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validate_surface_attributes
        Must include: building_feature_id, surface_feature_id, attribute_name,
                     calculated_value, thematic_value, difference, percent_error
                     Optional: geom (surface geometry)
    output_path : str or Path
        Full path for output CSV file
    error_threshold : float
        Percentage error threshold (default: 10%)

    Returns:
    --------
    pd.DataFrame : Filtered DataFrame with only problematic surfaces
    """
    if validation_df.empty:
        print("No validation data to export")
        return pd.DataFrame()

    # Filter for problematic surfaces based on error threshold and special cases
    problematic = validation_df[
        (abs(validation_df['percent_error']) > error_threshold) |
        (abs(validation_df['difference']) > 170)  # Catch 180° azimuth flips
    ].copy()

    if problematic.empty:
        print(f"No surfaces found with errors above {error_threshold}% threshold")
        return pd.DataFrame()

    # Export to CSV
    problematic.to_csv(output_path, index=False)

    n_surfaces = problematic['surface_feature_id'].nunique()
    n_buildings = problematic['building_feature_id'].nunique() if 'building_feature_id' in problematic.columns else 0

    print(f"\n{'='*80}")
    print(f"Exported {len(problematic)} problematic validations")
    print(f"  - {n_surfaces} unique surfaces")
    print(f"  - {n_buildings} unique buildings")
    print(f"  - Saved to: {output_path}")
    print(f"{'='*80}\n")

    return problematic
