-- Min-max normalises x to [0, 1] over [lo, hi] for TABULA matching. NULL when x is
-- unknown or the range is empty, so the dimension drops out of that distance.
-- double precision, so the integer codes (storeys, complexity) do not floor to 0.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.minmax_norm(
    x double precision,
    lo double precision,
    hi double precision
)
RETURNS double precision AS $$
    SELECT (x - lo) / NULLIF(hi - lo, 0);
$$ LANGUAGE sql IMMUTABLE;
