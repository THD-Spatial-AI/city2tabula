"""
Plots module for visualizing validation results.

This module provides:
- Scatter plots (calculated vs thematic)
- Error distribution histograms
- Box plots for error analysis
- Correlation plots
"""

import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np
import pandas as pd


def plot_comparison_scatter(validation_df, attribute_name, save_path=None, figsize=(8, 6)):
    """
    Create scatter plot comparing calculated vs thematic values.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str
        Attribute to plot
    save_path : str, optional
        Path to save figure
    figsize : tuple
        Figure size (width, height)

    Returns:
    --------
    matplotlib.figure.Figure : The created figure
    """
    df = validation_df[validation_df['attribute_name'] == attribute_name].copy()

    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    fig, ax = plt.subplots(figsize=figsize)

    # Scatter plot
    ax.scatter(df['thematic_value'], df['calculated_value'],
               alpha=0.6, s=50, edgecolors='k', linewidth=0.5)

    # Perfect agreement line (y = x)
    min_val = min(df['thematic_value'].min(), df['calculated_value'].min())
    max_val = max(df['thematic_value'].max(), df['calculated_value'].max())
    ax.plot([min_val, max_val], [min_val, max_val],
            'r--', label='Perfect Agreement', linewidth=2)

    # Calculate R²
    ss_res = ((df['calculated_value'] - df['thematic_value']) ** 2).sum()
    ss_tot = ((df['thematic_value'] - df['thematic_value'].mean()) ** 2).sum()
    r_squared = 1 - (ss_res / ss_tot) if ss_tot != 0 else 0

    # Calculate RMSE
    rmse = np.sqrt(((df['calculated_value'] - df['thematic_value']) ** 2).mean())

    # Labels and title
    ax.set_xlabel('Thematic Value', fontsize=12, fontweight='bold')
    ax.set_ylabel('Calculated Value (City2TABULA)', fontsize=12, fontweight='bold')
    ax.set_title(f'Validation: {attribute_name.replace("_", " ").title()}\n'
                 f'n={len(df)}, R²={r_squared:.4f}, RMSE={rmse:.4f}',
                 fontsize=14, fontweight='bold')
    ax.legend(fontsize=10)
    ax.grid(True, alpha=0.3)

    plt.tight_layout()

    if save_path:
        plt.savefig(save_path, dpi=300, bbox_inches='tight')
        print(f"Saved plot: {save_path}")

    return fig


def plot_error_distribution(validation_df, attribute_name, save_path=None, figsize=(10, 4)):
    """
    Create histogram and box plot of error distribution.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str
        Attribute to plot
    save_path : str, optional
        Path to save figure
    figsize : tuple
        Figure size (width, height)

    Returns:
    --------
    matplotlib.figure.Figure : The created figure
    """
    df = validation_df[validation_df['attribute_name'] == attribute_name].copy()

    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    fig, axes = plt.subplots(1, 2, figsize=figsize)

    # Histogram
    axes[0].hist(df['difference'], bins=30, edgecolor='k', alpha=0.7, color='steelblue')
    axes[0].axvline(0, color='r', linestyle='--', linewidth=2, label='Zero Error')
    axes[0].axvline(df['difference'].mean(), color='g', linestyle='--',
                    linewidth=2, label=f'Mean: {df["difference"].mean():.4f}')
    axes[0].set_xlabel('Error (Calculated - Thematic)', fontsize=11, fontweight='bold')
    axes[0].set_ylabel('Frequency', fontsize=11, fontweight='bold')
    axes[0].set_title('Error Distribution', fontsize=12, fontweight='bold')
    axes[0].legend()
    axes[0].grid(True, alpha=0.3, axis='y')

    # Box plot
    axes[1].boxplot(df['difference'], vert=True, patch_artist=True,
                    boxprops=dict(facecolor='lightblue', edgecolor='k'),
                    medianprops=dict(color='red', linewidth=2),
                    whiskerprops=dict(color='k'),
                    capprops=dict(color='k'))
    axes[1].axhline(0, color='r', linestyle='--', linewidth=2, label='Zero Error')
    axes[1].set_ylabel('Error (Calculated - Thematic)', fontsize=11, fontweight='bold')
    axes[1].set_title('Error Box Plot', fontsize=12, fontweight='bold')
    axes[1].legend()
    axes[1].grid(True, alpha=0.3, axis='y')

    fig.suptitle(f'Error Analysis: {attribute_name.replace("_", " ").title()} (n={len(df)})',
                 fontsize=14, fontweight='bold', y=1.02)

    plt.tight_layout()

    if save_path:
        plt.savefig(save_path, dpi=300, bbox_inches='tight')
        print(f"Saved plot: {save_path}")
        plt.close(fig)
        return None

    return fig


def plot_percent_error_distribution(validation_df, attribute_name, save_path=None, figsize=(8, 5)):
    """
    Create histogram of percentage error distribution.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str
        Attribute to plot
    save_path : str, optional
        Path to save figure
    figsize : tuple
        Figure size (width, height)

    Returns:
    --------
    matplotlib.figure.Figure : The created figure
    """
    df = validation_df[validation_df['attribute_name'] == attribute_name].copy()

    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    # Remove infinite and NaN values
    df = df[np.isfinite(df['percent_error'])]

    if df.empty:
        print(f"No valid percentage errors for attribute '{attribute_name}'")
        return None

    fig, ax = plt.subplots(figsize=figsize)

    # Histogram
    ax.hist(df['percent_error'], bins=30, edgecolor='k', alpha=0.7, color='coral')
    ax.axvline(0, color='r', linestyle='--', linewidth=2, label='Zero Error')
    ax.axvline(df['percent_error'].mean(), color='g', linestyle='--',
               linewidth=2, label=f'Mean: {df["percent_error"].mean():.2f}%')
    ax.axvline(df['percent_error'].median(), color='b', linestyle='--',
               linewidth=2, label=f'Median: {df["percent_error"].median():.2f}%')

    ax.set_xlabel('Percentage Error (%)', fontsize=12, fontweight='bold')
    ax.set_ylabel('Frequency', fontsize=12, fontweight='bold')
    ax.set_title(f'Percentage Error Distribution: {attribute_name.replace("_", " ").title()}\n'
                 f'n={len(df)}, std={df["percent_error"].std():.2f}%',
                 fontsize=14, fontweight='bold')
    ax.legend(fontsize=10)
    ax.grid(True, alpha=0.3, axis='y')

    plt.tight_layout()

    if save_path:
        plt.savefig(save_path, dpi=300, bbox_inches='tight')
        print(f"Saved plot: {save_path}")
        plt.close(fig)
        return None

    return fig


def plot_multi_attribute_comparison(validation_df, save_path=None, figsize=(12, 8), title_prefix=None):
    """
    Create comparison plots for all attributes in validation results.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    save_path : str, optional
        Path to save figure
    figsize : tuple
        Figure size (width, height)
    title_prefix : str, optional
        Prefix for subplot titles (e.g., 'Roof', 'Wall', 'Floor')

    Returns:
    --------
    matplotlib.figure.Figure : The created figure
    """
    if validation_df.empty:
        print("No data to plot")
        return None

    attributes = validation_df['attribute_name'].unique()
    n_attrs = len(attributes)

    if n_attrs == 0:
        print("No attributes found")
        return None

    # Calculate grid dimensions
    n_cols = 2
    n_rows = (n_attrs + 1) // 2

    fig, axes = plt.subplots(n_rows, n_cols, figsize=figsize)

    # Handle axes array properly based on number of subplots
    if n_rows == 1 and n_cols == 1:
        axes = [axes]
    elif n_rows == 1 or n_cols == 1:
        axes = axes.flatten()
    else:
        axes = axes.flatten()

    for idx, attr in enumerate(attributes):
        df = validation_df[validation_df['attribute_name'] == attr].copy()

        if df.empty:
            continue

        ax = axes[idx]

        # Scatter plot
        ax.scatter(df['thematic_value'], df['calculated_value'],
                   alpha=0.6, s=30, edgecolors='k', linewidth=0.5)

        # Perfect agreement line
        min_val = min(df['thematic_value'].min(), df['calculated_value'].min())
        max_val = max(df['thematic_value'].max(), df['calculated_value'].max())
        ax.plot([min_val, max_val], [min_val, max_val], 'r--', linewidth=1.5)

        # Calculate R²
        ss_res = ((df['calculated_value'] - df['thematic_value']) ** 2).sum()
        ss_tot = ((df['thematic_value'] - df['thematic_value'].mean()) ** 2).sum()
        r_squared = 1 - (ss_res / ss_tot) if ss_tot != 0 else 0

        # Format attribute name for display
        attr_display = attr.replace("_", " ").title()

        # Add prefix if provided (e.g., "Roof Surface Area", "Wall Surface Area")
        if title_prefix:
            attr_display = f'{title_prefix} {attr_display}'

        ax.set_xlabel('Thematic', fontsize=9)
        ax.set_ylabel('Calculated', fontsize=9)
        ax.set_title(f'{attr_display}\n'
                     f'n={len(df)}, R²={r_squared:.3f}',
                     fontsize=10, fontweight='bold')
        ax.grid(True, alpha=0.3)

    # Hide unused subplots
    for idx in range(n_attrs, len(axes)):
        axes[idx].axis('off')

    fig.suptitle('Multi-Attribute Validation Comparison',
                 fontsize=16, fontweight='bold', y=0.995)

    plt.tight_layout()

    return fig
