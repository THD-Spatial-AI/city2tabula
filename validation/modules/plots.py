# Algorithm and validation methodology by the author.
# Code implementation assisted by GitHub Copilot.

"""
Plots module for visualizing validation results.

This module provides:
- Scatter plots (calculated vs thematic)
- Error distribution histograms
- Box plots for error analysis
- Correlation plots
"""

import matplotlib

HAS_IPE = False
try:
    matplotlib.use('module://backend_ipe')
    HAS_IPE = True
except Exception:
    matplotlib.use('Agg')

import matplotlib.pyplot as plt
import numpy as np

def compute_errors(df, attribute):
    """
    Compute difference and percent_error safely for each attribute.
    Handles circular variables and avoids division-by-zero explosions.
    """
    thematic = df["thematic_value"]
    calc = df["calculated_value"]
    eps = 1e-6

    # ----------------------------------------------------------
    # HEIGHT (linear, but thematic may be 0 or extremely small)
    # ----------------------------------------------------------
    if attribute in ["min_height", "max_height", "height"]:
        df["difference"] = calc - thematic
        df["percent_error"] = (df["difference"] / (thematic + eps)) * 100
        return df

    # ----------------------------------------------------------
    # SURFACE AREA (valid % error, linear)
    # ----------------------------------------------------------
    if attribute in ["surface_area", "area", "footprint_area"]:
        df["difference"] = calc - thematic
        df["percent_error"] = (df["difference"] / (thematic + eps)) * 100
        return df

    # ----------------------------------------------------------
    # TILT (0–90°, not suitable for percent error)
    # thematic tilt may be 0 → percent error is meaningless
    # ----------------------------------------------------------
    if attribute == "tilt":
        df["difference"] = calc - thematic
        df["percent_error"] = np.nan      # disable percent error
        return df

    # ----------------------------------------------------------
    # AZIMUTH (circular variable 0–360°)
    # must use smallest angular distance
    # percent error is meaningless
    # ----------------------------------------------------------
    if attribute == "azimuth":
        raw = np.abs(calc - thematic) % 360
        circ = np.where(raw > 180, 360 - raw, raw)
        df["difference"] = circ
        df["percent_error"] = np.nan      # disable % error
        return df

    # Default fallback
    df["difference"] = calc - thematic
    df["percent_error"] = np.nan
    return df

def plot_comparison_scatter(validation_df, attribute_name, save_path=None, fig_format=None):
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
    df = compute_errors(df, attribute_name)

    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    fig, ax = plt.subplots(figsize=(8, 6))

    # Scatter plot
    ax.scatter(df['thematic_value'], df['calculated_value'],
               alpha=0.6, s=50, edgecolors='k', linewidth=0.5)

    # Perfect agreement line (y = x)
    min_val = min(df['thematic_value'].min(), df['calculated_value'].min())
    max_val = max(df['thematic_value'].max(), df['calculated_value'].max())
    ax.plot([min_val, max_val], [min_val, max_val],
            'r--', label='Perfect Agreement', linewidth=2)

    # Calculate R² and RMSE using pre-computed difference (handles circular attrs)
    ss_res = (df['difference'] ** 2).sum()
    ss_tot = ((df['thematic_value'] - df['thematic_value'].mean()) ** 2).sum()
    r_squared = 1 - (ss_res / ss_tot) if ss_tot != 0 else 0

    rmse = np.sqrt((df['difference'] ** 2).mean())

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
        # If requesting IPE without backend support, save PNG alternative
        if fig_format == 'ipe' and not HAS_IPE:
            png_path = (
                save_path[:-4] + '.png' if save_path.lower().endswith('.ipe') else save_path + '.png'
            )
            print("IPE backend not available; saving PNG:", png_path)
            plt.savefig(png_path, dpi=300, bbox_inches='tight')
        else:
            plt.savefig(save_path, dpi=300, bbox_inches='tight', format=fig_format)
        plt.close(fig)
        return None

    return fig


def plot_error_distribution(validation_df, attribute_name, save_path=None, fig_format=None):
    """
    Create histogram of error distribution.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Validation results from validators module
    attribute_name : str
        Attribute to plot
    save_path : str, optional
        Path to save figure

    Returns:
    --------
    matplotlib.figure.Figure : The created figure
    """
    df = validation_df[validation_df['attribute_name'] == attribute_name].copy()
    df = compute_errors(df, attribute_name)
    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    fig, ax = plt.subplots()

    # Histogram
    ax.hist(df['difference'], bins=30, edgecolor='k', alpha=0.7, color='steelblue')
    ax.axvline(0, color='r', linestyle='--', linewidth=2, label='Zero Error')
    ax.axvline(df['difference'].mean(), color='g', linestyle='--',
                    linewidth=2, label=f'Mean: {df["difference"].mean():.4f}')
    ax.set_xlabel('Error (Calculated - Thematic)', fontsize=11, fontweight='bold')
    ax.set_ylabel('Frequency', fontsize=11, fontweight='bold')
    ax.set_title(f'Error Distribution: {attribute_name.replace("_", " ").title()} (n={len(df)})',
                 fontsize=12, fontweight='bold')
    ax.legend()
    ax.grid(True, alpha=0.3, axis='y')

    plt.tight_layout()

    if save_path:
        if fig_format == 'ipe' and not HAS_IPE:
            png_path = (
                save_path[:-4] + '.png' if save_path.lower().endswith('.ipe') else save_path + '.png'
            )
            print("IPE backend not available; saving PNG:", png_path)
            plt.savefig(png_path, dpi=300, bbox_inches='tight')
        else:
            plt.savefig(save_path, dpi=300, bbox_inches='tight', format=fig_format)
        plt.close(fig)
        return None

    return fig


def plot_percent_error_distribution(validation_df, attribute_name, save_path=None, fig_format=None):
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
    df = compute_errors(df, attribute_name)

    if df.empty:
        print(f"No data for attribute '{attribute_name}'")
        return None

    # Remove infinite and NaN values
    df = df[np.isfinite(df['percent_error'])]

    if df.empty:
        print(f"No valid percentage errors for attribute '{attribute_name}'")
        return None

    fig, ax = plt.subplots()

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
        if fig_format == 'ipe' and not HAS_IPE:
            png_path = (
                save_path[:-4] + '.png' if save_path.lower().endswith('.ipe') else save_path + '.png'
            )
            print("IPE backend not available; saving PNG:", png_path)
            plt.savefig(png_path, dpi=300, bbox_inches='tight')
        else:
            plt.savefig(save_path, dpi=300, bbox_inches='tight', format=fig_format)
        plt.close(fig)
        return None

    return fig


def plot_height_neighbour_split(validation_df, attributes=None, save_path=None, fig_format=None):
    """
    Scatter plot for height attributes split by has_attached_neighbour status.

    Shows standalone (False) and attached (True) buildings as two colour-coded
    groups on the same axes, with per-group RMSE and mean-difference annotations.
    Requires 'has_attached_neighbour' to be present in validation_df.

    Parameters:
    -----------
    validation_df : pd.DataFrame
        Building validation results that include a 'has_attached_neighbour' column.
    attributes : list, optional
        Height attributes to plot. Defaults to ['min_height', 'max_height'].
    save_path : str, optional
        Path to save figure.
    fig_format : str, optional
        Output format passed to savefig.

    Returns:
    --------
    matplotlib.figure.Figure or None
    """
    if 'has_attached_neighbour' not in validation_df.columns:
        print("Warning: 'has_attached_neighbour' column not found — skipping neighbour split plot")
        return None

    if attributes is None:
        attributes = ['min_height', 'max_height']

    avail = [a for a in attributes if a in validation_df['attribute_name'].unique()]
    if not avail:
        print("No matching attributes found for neighbour split plot")
        return None

    n_attrs = len(avail)
    fig, axes = plt.subplots(1, n_attrs, figsize=(6 * n_attrs, 5))
    if n_attrs == 1:
        axes = [axes]

    colors = {False: 'steelblue', True: 'tomato'}
    labels = {False: 'Standalone', True: 'Attached'}

    for ax, attr in zip(axes, avail):
        df = validation_df[validation_df['attribute_name'] == attr].copy()
        df = compute_errors(df, attr)
        if df.empty:
            ax.axis('off')
            continue

        min_val = min(df['thematic_value'].min(), df['calculated_value'].min())
        max_val = max(df['thematic_value'].max(), df['calculated_value'].max())
        ax.plot([min_val, max_val], [min_val, max_val], 'k--', linewidth=1.5, label='Perfect agreement')

        for flag in [False, True]:
            grp = df[df['has_attached_neighbour'] == flag]
            if grp.empty:
                continue
            rmse = np.sqrt((grp['difference'] ** 2).mean())
            mean_diff = grp['difference'].mean()
            ax.scatter(
                grp['thematic_value'], grp['calculated_value'],
                alpha=0.5, s=20, color=colors[flag], edgecolors='none',
                label=f"{labels[flag]}  n={len(grp)}, RMSE={rmse:.2f} m, mean={mean_diff:+.2f} m"
            )

        attr_display = attr.replace('_', ' ').title()
        ax.set_xlabel('Thematic Value (m)', fontsize=10)
        ax.set_ylabel('Calculated Value (m)', fontsize=10)
        ax.set_title(attr_display, fontsize=11, fontweight='bold')
        ax.legend(fontsize=8, loc='upper left')
        ax.grid(True, alpha=0.3)

    fig.suptitle('Height Validation: Standalone vs Attached Buildings',
                 fontsize=13, fontweight='bold')
    plt.tight_layout()

    if save_path:
        if fig_format == 'ipe' and not HAS_IPE:
            png_path = (
                save_path[:-4] + '.png' if save_path.lower().endswith('.ipe') else save_path + '.png'
            )
            print("IPE backend not available; saving PNG:", png_path)
            plt.savefig(png_path, dpi=300, bbox_inches='tight')
        else:
            plt.savefig(save_path, dpi=300, bbox_inches='tight', format=fig_format)
        plt.close(fig)
        return None

    return fig


def plot_multi_attribute_comparison(validation_df, save_path=None, figsize=(12, 8), title_prefix=None, fig_format=None):
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
        df = compute_errors(df, attr)

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

        # Calculate R² using pre-computed difference (handles circular attrs)
        ss_res = (df['difference'] ** 2).sum()
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
    if save_path:
        if fig_format == 'ipe' and not HAS_IPE:
            png_path = (
                save_path[:-4] + '.png' if save_path.lower().endswith('.ipe') else save_path + '.png'
            )
            print("IPE backend not available; saving PNG:", png_path)
            plt.savefig(png_path, dpi=300, bbox_inches='tight')
        else:
            plt.savefig(save_path, dpi=300, bbox_inches='tight', format=fig_format)
        plt.close(fig)
        return None

    return fig
