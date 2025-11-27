import os
import yaml
from dotenv import load_dotenv
from sqlalchemy import create_engine, text

# Load environment variables from a .env file first (before using os.getenv)
load_dotenv()


def load_config(config_path: str):
    """
    Load configuration from a YAML file.

    Parameters:
    -----------
    config_path : str, optional
        Path to the config file. If None, uses the default path based on COUNTRY env var.

    Returns:
    --------
    dict : Configuration dictionary
    """
    # Check if file exists
    if not os.path.exists(config_path):
        raise FileNotFoundError(
            f"Configuration file not found: {config_path}\n"
            f"Expected file: config_{os.getenv('COUNTRY', '')}.yaml\n"
            f"Available configs: {os.listdir(os.path.join(os.path.dirname(__file__), '..', 'configs'))}"
        )

    with open(config_path, 'r') as file:
        config = yaml.safe_load(file)

    print(f"Loaded configuration for: {config.get('dataset', {}).get('country', 'Unknown')}")

    # Add database configuration
    config = _add_db_config(config)

    return config

def _add_db_config(config):
    """
    Add database connection string to the existing config dictionary.

    This preserves the existing db configuration (schemas, tables, etc.)
    from the YAML file and only adds the connection_string.

    Parameters:
    -----------
    config : dict
        Existing configuration dictionary loaded from YAML.

    Returns:
    --------
    dict : Updated configuration dictionary with connection_string added to db section.
    """
    # Get or initialize the db section
    if 'db' not in config:
        config['db'] = {}

    # Add connection string (preserving existing db config from YAML)
    country = config.get('dataset', {}).get('country', '').lower()
    config['db']['connection_string'] = (
        f"postgresql+psycopg2://{os.getenv('DB_USER')}:{os.getenv('DB_PASSWORD')}"
        f"@{os.getenv('DB_HOST')}:{os.getenv('DB_PORT')}/city2tabula_{country}"
    )

    return config

def get_building_attribute_mapping(config):
    """
    Extract building-level attribute mapping from config.

    Returns:
    --------
    dict : {'computed_column': 'source_label', ...}
    """
    parent_attrs = config.get('attributes', {}).get('parent', {})
    mapping = {}

    for attr_name, attr_config in parent_attrs.items():
        source_label = attr_config.get('source_label', '')
        if not source_label:  # Skip if no source label
            continue

        # Check if computed_columns (plural) exists - for attributes with multiple columns
        if 'computed_columns' in attr_config:
            for computed_col in attr_config['computed_columns']:
                mapping[computed_col] = source_label
        # Otherwise use computed_column (singular)
        elif 'computed_column' in attr_config:
            mapping[attr_config['computed_column']] = source_label

    return mapping


def get_surface_attribute_mapping(config, surface_type='roof'):
    """
    Extract surface-level attribute mapping from config.

    Parameters:
    -----------
    surface_type : str
        Surface type: 'roof', 'wall', or 'floor'

    Returns:
    --------
    dict : {'computed_column': 'source_label', ...}
    """
    child_attrs = config.get('attributes', {}).get('child', {}).get(surface_type, {})
    return {
        attr_config.get('computed_column'): attr_config.get('source_label', '')
        for attr_name, attr_config in child_attrs.items()
        if attr_config.get('source_label', '')  # Only include if source_label is not empty
    }


def get_surface_type_filter(attribute_name):
    """
    Determine which surface types should be validated for a given attribute.

    Parameters:
    -----------
    attribute_name : str
        Attribute name (e.g., 'tilt', 'azimuth', 'area')

    Returns:
    --------
    list or None : List of surface classnames (e.g., ['RoofSurface']) or None for all surfaces
    """
    # Tilt and azimuth should only be validated for roof surfaces
    if attribute_name in ['tilt', 'azimuth']:
        return ['RoofSurface']

    # Area can be validated for all surface types
    return None


def print_config_summary(config):
    """Print a formatted summary of the loaded configuration."""
    print("\n" + "="*80)
    print("CONFIGURATION SUMMARY")
    print("="*80)

    dataset = config.get('dataset', {})
    print(f"\n Dataset: {dataset.get('name', 'Unknown')}")
    print(f"   Country: {dataset.get('country', 'Unknown')}")
    print(f"   LoD: {dataset.get('lod', 'Unknown')}")
    print(f"   Description: {dataset.get('description', 'N/A')}")

    print(f"\n Building Attributes:")
    building_mapping = get_building_attribute_mapping(config)
    for computed_col, source_label in building_mapping.items():
        # Find the parent attribute config that contains this computed_column
        parent_attrs = config['attributes']['parent']
        unit = ''
        for attr_name, attr_config in parent_attrs.items():
            # Check if this is a single computed_column
            if attr_config.get('computed_column') == computed_col:
                unit = attr_config.get('unit', '')
                break
            # Or if it's in computed_columns list
            elif computed_col in attr_config.get('computed_columns', []):
                unit = attr_config.get('unit', '')
                break
        print(f"   {computed_col:20s} <- '{source_label}' ({unit})")

    print(f"\n Surface Attributes:")
    for surface_type in ['roof', 'wall', 'floor']:
        surface_mapping = get_surface_attribute_mapping(config, surface_type)
        if surface_mapping:
            print(f"   {surface_type.upper()}:")
            for computed_col, source_label in surface_mapping.items():
                # Find the attribute config by matching computed_column
                child_attrs = config['attributes']['child'][surface_type]
                surface_config = None
                for attr_name, attr_config in child_attrs.items():
                    if attr_config.get('computed_column') == computed_col:
                        surface_config = attr_config
                        break
                unit = surface_config.get('unit', '') if surface_config else ''
                print(f"   {computed_col:20s} <- '{source_label}' ({unit})")

    print(f"\n Validation Tolerances:")
    tolerances = config.get('validation', {}).get('tolerance', {})
    abs_tol = tolerances.get('abs', {})
    pct_tol = tolerances.get('percent', {})
    if abs_tol:
        print("   Absolute:")
        for attr, val in abs_tol.items():
            print(f"   {attr:20s} ±{val}")
    if pct_tol:
        print("   Percentage:")
        for attr, val in pct_tol.items():
            print(f"   {attr:20s} ±{val}%")

    print("="*80)

if __name__ == "__main__":
    # Test loading configuration
    try:
        config = load_config(os.path.join(os.path.dirname(__file__), '..', 'configs', f'config_germany.yaml'))  # Replace with an actual test config path
        print_config_summary(config)

        # Test helper functions
        print("\n" + "="*80)
        print("TESTING HELPER FUNCTIONS")
        print("="*80)

        print("\n1. Building attribute mapping:")
        building_map = get_building_attribute_mapping(config)
        for k, v in building_map.items():
            print(f"   {k}: {v}")

        print("\n2. Roof surface attribute mapping:")
        roof_map = get_surface_attribute_mapping(config, 'roof')
        for k, v in roof_map.items():
            print(f"   {k}: {v}")

        print("\n3. Surface type filters:")
        for attr in ['tilt', 'azimuth', 'area']:
            filter_val = get_surface_type_filter(attr)
            print(f"   {attr}: {filter_val}")

        print("4. Database configuration:")
        db_config = _add_db_config(config).get('db', {})
        for k, v in db_config.items():
            print(f"   {k}: {v}")

        print("\n" + "="*80)
        print("ALL TESTS PASSED")
        print("="*80)

    except Exception as e:
        print(f"Error loading configuration: {e}")
        import traceback
        traceback.print_exc()