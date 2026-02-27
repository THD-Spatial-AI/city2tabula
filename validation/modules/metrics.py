# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Metrics module for calculating statistical measures on validation results.

This module provides:
- Statistical metrics (RMSE, MAE, R²)
- Distribution analysis
- Outlier detection
- Tolerance checking
"""

import pandas as pd
import numpy as np


def calculate_rmse(validation_df, attribute_name=None):
    """
    Calculate Root Mean Square Error for validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str, optional
        Filter by specific attribute name

    Returns:
    --------
    float or dict : RMSE value(s)
    """
    if validation_df.empty:
        return None

    if attribute_name:
        df = validation_df[validation_df['attribute_name'] == attribute_name]
        if df.empty:
            return None
        return np.sqrt((df['difference'] ** 2).mean())

    # Calculate RMSE for each attribute
    return validation_df.groupby('attribute_name')['difference'].apply(
        lambda x: np.sqrt((x ** 2).mean())
    ).to_dict()


def calculate_mae(validation_df, attribute_name=None):
    """
    Calculate Mean Absolute Error for validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str, optional
        Filter by specific attribute name

    Returns:
    --------
    float or dict : MAE value(s)
    """
    if validation_df.empty:
        return None

    if attribute_name:
        df = validation_df[validation_df['attribute_name'] == attribute_name]
        if df.empty:
            return None
        return df['difference'].abs().mean()

    # Calculate MAE for each attribute
    return validation_df.groupby('attribute_name')['difference'].apply(
        lambda x: x.abs().mean()
    ).to_dict()


def calculate_r_squared(validation_df, attribute_name=None):
    """
    Calculate coefficient of determination (R²) for validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str, optional
        Filter by specific attribute name

    Returns:
    --------
    float or dict : R² value(s)
    """
    if validation_df.empty:
        return None

    def r2_score(df):
        if len(df) < 2:
            return None
        ss_res = ((df['calculated_value'] - df['thematic_value']) ** 2).sum()
        ss_tot = ((df['thematic_value'] - df['thematic_value'].mean()) ** 2).sum()
        if ss_tot == 0:
            return None
        return 1 - (ss_res / ss_tot)

    if attribute_name:
        df = validation_df[validation_df['attribute_name'] == attribute_name]
        if df.empty:
            return None
        return r2_score(df)

    # Calculate R² for each attribute
    return validation_df.groupby('attribute_name').apply(r2_score).to_dict()


def check_tolerance(validation_df, config, tolerance_type='abs'):
    """
    Check if validation results are within configured tolerance thresholds.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    config : dict
        Configuration dictionary with validation.tolerance settings
    tolerance_type : str
        'abs' for absolute tolerance or 'percent' for percentage tolerance

    Returns:
    --------
    pd.DataFrame : Validation results with added 'within_tolerance' column
    """
    if validation_df.empty:
        return validation_df

    tolerances = config.get('validation', {}).get('tolerance', {}).get(tolerance_type, {})

    if not tolerances:
        print(f"Warning: No {tolerance_type} tolerances defined in config")
        return validation_df

    result_df = validation_df.copy()

    def is_within_tolerance(row):
        attr_name = row['attribute_name']

        # Map computed columns to tolerance keys
        tolerance_key = attr_name
        if attr_name in ['surface_area']:
            tolerance_key = 'surface_area'
        elif attr_name in ['min_height', 'max_height']:
            tolerance_key = 'height'

        threshold = tolerances.get(tolerance_key)
        if threshold is None:
            return None

        if tolerance_type == 'abs':
            return abs(row['difference']) <= threshold
        else:  # percent
            return abs(row['percent_error']) <= threshold

    result_df['within_tolerance'] = result_df.apply(is_within_tolerance, axis=1)

    return result_df


def get_outliers(validation_df, attribute_name=None, method='iqr', threshold=3.0):
    """
    Detect outliers in validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str, optional
        Filter by specific attribute name
    method : str
        'iqr' for interquartile range or 'zscore' for standard deviation
    threshold : float
        Threshold for outlier detection (IQR multiplier or Z-score)

    Returns:
    --------
    pd.DataFrame : Subset of validation_df containing only outliers
    """
    if validation_df.empty:
        return validation_df

    df = validation_df.copy()
    if attribute_name:
        df = df[df['attribute_name'] == attribute_name]

    if method == 'iqr':
        Q1 = df['difference'].quantile(0.25)
        Q3 = df['difference'].quantile(0.75)
        IQR = Q3 - Q1
        lower_bound = Q1 - threshold * IQR
        upper_bound = Q3 + threshold * IQR
        outliers = df[(df['difference'] < lower_bound) | (df['difference'] > upper_bound)]

    elif method == 'zscore':
        mean_diff = df['difference'].mean()
        std_diff = df['difference'].std()
        z_scores = (df['difference'] - mean_diff) / std_diff
        outliers = df[abs(z_scores) > threshold]

    else:
        raise ValueError(f"Unknown method: {method}. Use 'iqr' or 'zscore'")

    return outliers
