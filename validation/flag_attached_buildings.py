"""
Standalone script to detect and flag buildings that have attached/neighbouring
geometry (overlapping or near-touching footprints).

Populates has_attached_neighbour, attached_neighbour_id, total_attached_neighbour,
and attached_neighbour_class on the city2tabula building_feature table.

Run this before the validation notebook to enable the standalone-vs-attached
height accuracy split.

Usage:
    python flag_attached_buildings.py [--lod lod2] [--tolerance 0.5] [--env ../.env]
"""

import argparse
import os
import sys

import psycopg2
from dotenv import load_dotenv


def build_connection_string(env_path):
    load_dotenv(env_path)
    host     = os.getenv("DB_HOST", "localhost")
    port     = os.getenv("DB_PORT", "5432")
    dbname   = os.getenv("DB_NAME")
    user     = os.getenv("DB_USER")
    password = os.getenv("DB_PASSWORD")
    if not dbname or not user:
        raise ValueError("DB_NAME and DB_USER must be set in the .env file")
    return f"host={host} port={port} dbname={dbname} user={user} password={password}"


def flag_attached_buildings(conn, schema, lod, tolerance):
    neighbours_table  = f"{schema}.{lod}_building_neighbours"
    building_table    = f"{schema}.{lod}_building_feature"

    with conn.cursor() as cur:
        # 1 — reset all flags so re-runs are idempotent
        print(f"  Resetting has_attached_neighbour on {building_table}...")
        cur.execute(f"""
            UPDATE {building_table}
            SET    has_attached_neighbour   = FALSE,
                   attached_neighbour_id    = NULL,
                   total_attached_neighbour = 0,
                   attached_neighbour_class = 0
        """)
        print(f"    {cur.rowcount} rows reset")

        # 2 — repopulate the neighbour-pairs table
        print(f"  Clearing {neighbours_table}...")
        cur.execute(f"TRUNCATE {neighbours_table}")

        print(f"  Detecting neighbours (ST_DWithin tolerance = {tolerance} m)...")
        cur.execute(f"""
            INSERT INTO {neighbours_table} (id, building_id_a, building_id_b)
            SELECT gen_random_uuid(),
                   a.building_feature_id,
                   b.building_feature_id
            FROM   {building_table} a
            JOIN   {building_table} b
                      ON  a.building_feature_id < b.building_feature_id
                      AND ST_DWithin(
                            ST_Force2D(a.building_footprint_geom),
                            ST_Force2D(b.building_footprint_geom),
                            %s
                          )
            WHERE  a.building_footprint_geom IS NOT NULL
              AND  b.building_footprint_geom IS NOT NULL
        """, (tolerance,))
        pair_count = cur.rowcount
        print(f"    {pair_count} neighbour pairs found")

        if pair_count == 0:
            print("  No neighbours detected — all buildings marked standalone")
            conn.commit()
            return

        # 3 — update building_feature with neighbour info
        print(f"  Updating {building_table} with neighbour flags...")
        cur.execute(f"""
            UPDATE {building_table} bf
            SET    has_attached_neighbour   = (nd.neighbour_count > 0),
                   attached_neighbour_id    = nd.neighbour_ids,
                   total_attached_neighbour = nd.neighbour_count,
                   attached_neighbour_class = CASE
                                               WHEN nd.neighbour_count = 0 THEN 0
                                               WHEN nd.neighbour_count = 1 THEN 1
                                               ELSE 2
                                             END
            FROM (
                SELECT building_id_a AS fid,
                       COUNT(*)::INTEGER AS neighbour_count,
                       ARRAY_AGG(building_id_b ORDER BY building_id_b) AS neighbour_ids
                FROM   {neighbours_table}
                GROUP  BY building_id_a
                UNION ALL
                SELECT building_id_b,
                       COUNT(*)::INTEGER,
                       ARRAY_AGG(building_id_a ORDER BY building_id_a)
                FROM   {neighbours_table}
                GROUP  BY building_id_b
            ) nd
            WHERE  bf.building_feature_id = nd.fid
        """)
        flagged = cur.rowcount
        print(f"    {flagged} buildings flagged as having attached neighbours")

        conn.commit()

        # 4 — print a quick summary
        cur.execute(f"""
            SELECT has_attached_neighbour, COUNT(*)
            FROM   {building_table}
            GROUP  BY has_attached_neighbour
            ORDER  BY has_attached_neighbour
        """)
        rows = cur.fetchall()
        print("\n  Summary:")
        for flag, count in rows:
            label = "Attached  (has_attached_neighbour=True) " if flag else "Standalone (has_attached_neighbour=False)"
            print(f"    {label}: {count}")


def main():
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--lod",       default="lod2",
                        help="LoD schema prefix, e.g. lod2 or lod3 (default: lod2)")
    parser.add_argument("--tolerance", type=float, default=0.5,
                        help="ST_DWithin distance in CRS units (metres) for neighbour detection (default: 0.5)")
    parser.add_argument("--schema",    default="city2tabula",
                        help="City2TABULA schema name (default: city2tabula)")
    parser.add_argument("--env",       default=os.path.join(os.path.dirname(__file__), "../.env"),
                        help="Path to .env file (default: ../.env relative to this script)")
    args = parser.parse_args()

    env_path = os.path.abspath(args.env)
    if not os.path.exists(env_path):
        print(f"ERROR: .env file not found at {env_path}", file=sys.stderr)
        sys.exit(1)

    print(f"Loading environment from: {env_path}")
    conn_str = build_connection_string(env_path)

    print(f"Connecting to database...")
    try:
        conn = psycopg2.connect(conn_str)
    except Exception as e:
        print(f"ERROR: Could not connect to database: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"Connected.")
    print(f"Schema: {args.schema}  |  LoD: {args.lod}  |  Tolerance: {args.tolerance} m\n")

    try:
        flag_attached_buildings(conn, args.schema, args.lod, args.tolerance)
        print("\nDone.")
    except Exception as e:
        conn.rollback()
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
    finally:
        conn.close()


if __name__ == "__main__":
    main()
