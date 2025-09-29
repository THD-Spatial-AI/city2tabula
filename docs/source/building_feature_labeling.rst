Building Feature Labeling Algorithm
===================================

Overview
--------

The building feature labeling algorithm is a critical component of the City2TABULA pipeline that automatically assigns TABULA building variant codes to LoD2 building features. This process enables the creation of labeled training datasets for machine learning models by matching extracted building characteristics with known TABULA building archetypes.

The algorithm employs a **multi-dimensional distance-based matching approach** that compares building features across nine key architectural and geometric characteristics to find the most similar TABULA variant for each building.

Problem Context
---------------

**Challenge**: Given a set of buildings with extracted geometric and architectural features, we need to classify them according to the TABULA building typology system. TABULA (Typology Approach for Building Stock Energy Assessment) provides a standardized European building classification system with predefined building variants that have known energy performance characteristics.

**Solution**: Use a normalized Euclidean distance metric to find the closest matching TABULA variant for each building based on their feature similarity.

Algorithm Overview
------------------

The algorithm consists of three main phases:

1. **Statistical Normalization**: Calculate min/max ranges across all features
2. **Distance Calculation**: Compute normalized Euclidean distances between buildings and TABULA variants
3. **Best Match Assignment**: Assign each building the TABULA variant with the smallest distance

Feature Dimensions
------------------

The algorithm compares buildings across these nine dimensions:

.. list-table:: Building Feature Dimensions
   :header-rows: 1
   :widths: 25 75

   * - Feature
     - Description
   * - ``max_volume``
     - Maximum building volume (m³)
   * - ``footprint_area``
     - Building footprint area (m²)
   * - ``number_of_storeys``
     - Number of building floors/storeys
   * - ``attached_neighbour_class``
     - Classification of building attachment (detached, semi-detached, etc.)
   * - ``footprint_complexity``
     - Geometric complexity measure of the building footprint
   * - ``roof_complexity``
     - Geometric complexity measure of the roof structure
   * - ``area_total_roof``
     - Total roof surface area (m²)
   * - ``area_total_wall``
     - Total exterior wall surface area (m²)
   * - ``area_total_floor``
     - Total floor area (m²)

Mathematical Method
-------------------

Normalization Process
~~~~~~~~~~~~~~~~~~~~~

To ensure fair comparison across features with different scales and units, the algorithm first normalizes all values to a [0,1] range:

.. math::

   normalized\_value = \frac{value - min\_value}{max\_value - min\_value}

Where:
- ``value`` is the original feature value
- ``min_value`` and ``max_value`` are computed across both building features and TABULA variants

This normalization ensures that:
- Large-scale features (like volume) don't dominate small-scale features (like number of storeys)
- All features contribute equally to the distance calculation
- The algorithm works regardless of measurement units

Distance Calculation
~~~~~~~~~~~~~~~~~~~~

For each building, the algorithm calculates the **normalized Euclidean distance** to every TABULA variant:

.. math::

   distance = \sqrt{\sum_{i=1}^{9} (normalized\_building\_feature_i - normalized\_tabula\_feature_i)^2}

This creates a 9-dimensional feature space where:
- Each dimension represents one building characteristic
- Each building and TABULA variant is a point in this space
- Distance represents overall similarity between buildings

Best Match Selection
~~~~~~~~~~~~~~~~~~~~

The algorithm uses SQL window functions to rank matches:

1. **Partition**: Group results by building (``PARTITION BY building_feature_id``)
2. **Order**: Sort by distance (``ORDER BY distance ASC``)
3. **Rank**: Assign rank numbers (``ROW_NUMBER()``)
4. **Select**: Choose the top-ranked match (``WHERE rnk = 1``)

Implementation Details
----------------------

SQL Structure
~~~~~~~~~~~~~

The implementation uses a Common Table Expression (CTE) structure:

.. code-block:: sql

   WITH stats AS (
     -- Calculate normalization ranges across all data
   ),
   ranked AS (
     -- Calculate distances and rank matches
   )
   UPDATE building_features
   SET tabula_variant = best_match;

Data Quality Handling
~~~~~~~~~~~~~~~~~~~~~

The algorithm includes several data quality measures:

**Missing Value Handling**:
- Uses ``COALESCE()`` to handle NULL values by defaulting them to 0
- Filters out records with missing critical features using ``WHERE`` clauses

**Division by Zero Protection**:
- Uses ``NULLIF()`` to prevent division by zero in normalization
- Handles cases where min and max values are identical

**Cross Join Strategy**:
- Compares every building against every TABULA variant
- Ensures comprehensive coverage of all possible matches

Performance Considerations
--------------------------

**Computational Complexity**:
- Time complexity: O(n × m) where n = buildings, m = TABULA variants
- Space complexity: O(n × m) for intermediate results

**Optimization Strategies**:
- Pre-filtering removes records with missing essential features
- Window functions enable efficient ranking without subqueries
- Single UPDATE statement minimizes database transactions

Validation and Quality Assurance
---------------------------------

**Distance Validation**:
- Distances are always ≥ 0 (Euclidean property)
- Perfect matches have distance = 0
- Normalized features ensure bounded distance ranges

**Match Quality Indicators**:
- Small distances indicate high similarity
- Large distances may indicate outliers or gaps in TABULA coverage
- Distribution of distances can reveal data quality issues

Use Cases and Applications
--------------------------

**Training Data Generation**:
- Creates labeled datasets for machine learning model training
- Enables supervised learning for building classification

**Building Stock Analysis**:
- Categorizes existing building stock according to TABULA typology
- Supports energy performance assessment at scale

**Quality Control**:
- Identifies buildings that don't match well with existing TABULA variants
- Highlights potential gaps in building typology coverage

Limitations and Considerations
------------------------------

**Assumptions**:
- Equal weighting of all features (each contributes equally to distance)
- Linear relationships between feature differences
- Euclidean distance is appropriate for the feature space

**Potential Improvements**:
- Feature weighting based on importance or reliability
- Alternative distance metrics (Manhattan, Mahalanobis)
- Clustering validation to assess match quality
- Incorporation of uncertainty measures

**Data Dependencies**:
- Requires complete feature extraction pipeline
- Depends on TABULA variant database completeness
- Sensitive to feature extraction accuracy

Conclusion
----------

The building feature labeling algorithm provides a robust, automated approach to classifying buildings according to the TABULA typology system. By using normalized multi-dimensional distance matching, it creates high-quality labeled datasets essential for machine learning applications in building energy assessment and urban analytics.

The mathematical foundation ensures fair comparison across diverse building characteristics, while the SQL implementation provides efficiency and scalability for large building datasets.
