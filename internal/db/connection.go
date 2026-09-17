package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// bootstrapDSN connects to the "postgres" maintenance DB, used for operations
// (existence checks, CREATE DATABASE) that can't run against the target DB itself.
func bootstrapDSN(cfg *config.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.SSLMode,
	)
}

// ErrCountryNotConfigured reports that a country has no usable dataset: either
// no database, or a database without the City2TABULA tables. Read-only callers
// use it to answer "no data for this country" instead of creating one:
// ConnectPool creates the database as a side effect, which a GET must not do.
var ErrCountryNotConfigured = errors.New("country has no dataset")

// MissingRunSchemas returns the schemas an incremental import needs but cfg's
// database does not have. ImportAllData assumes CreateCompleteDatabase built
// them; a database restored from a fixture holds only the City2TABULA schema,
// so the import would otherwise fail on the first missing relation.
func MissingRunSchemas(pool *pgxpool.Pool, cfg *config.Config) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	required := []string{cfg.DB.Schemas.Tabula, cfg.DB.Schemas.CityDB, cfg.DB.Schemas.Lod2}
	var missing []string
	for _, schema := range required {
		var present bool
		if err := pool.QueryRow(ctx,
			`SELECT to_regnamespace($1) IS NOT NULL`, schema,
		).Scan(&present); err != nil {
			return nil, fmt.Errorf("check schema %s exists in %s: %w", schema, cfg.DB.Name, err)
		}
		if !present {
			missing = append(missing, schema)
		}
	}
	return missing, nil
}

// CountryProvisioned reports whether cfg's database has the City2TABULA tables.
// A database can exist without them, because earlier builds created one as a
// side effect of a read, so existence alone does not mean a country is usable.
func CountryProvisioned(pool *pgxpool.Pool, cfg *config.Config) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	qualified := cfg.DB.Schemas.City2Tabula + ".building_link"
	var provisioned bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, qualified,
	).Scan(&provisioned); err != nil {
		return false, fmt.Errorf("check %s exists in %s: %w", qualified, cfg.DB.Name, err)
	}
	return provisioned, nil
}

// DatabaseExists reports whether cfg.DB.Name already exists as a Postgres database.
// Used by the on-request HTTP server (internal/api) to decide whether a country is
// being touched for the first time (needs the full CreateCompleteDatabase flow) or
// already has a database (an incremental ImportAllData is enough).
func DatabaseExists(cfg *config.Config) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, bootstrapDSN(cfg))
	if err != nil {
		return false, fmt.Errorf("connect to bootstrap DB failed: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.DB.Name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check database %s exists: %w", cfg.DB.Name, err)
	}
	return exists, nil
}

// Ensure the target database exists by connecting to the bootstrap "postgres" DB
func EnsureDatabase(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, bootstrapDSN(cfg))
	if err != nil {
		return fmt.Errorf("connect to bootstrap DB failed: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.DB.Name).Scan(&exists); err != nil {
		return fmt.Errorf("check database %s exists: %w", cfg.DB.Name, err)
	}

	if !exists {
		utils.Info.Printf("Database %s does not exist, creating...", cfg.DB.Name)
		// NOTE: requires the connecting role to have CREATEDB or be superuser
		if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, cfg.DB.Name)); err != nil {
			return fmt.Errorf("failed to create database %s: %w", cfg.DB.Name, err)
		}
		utils.Info.Printf("Database %s created successfully", cfg.DB.Name)
	}

	return nil
}

// Open pooled DB connection to the *target* database and ensure PostGIS there
func ConnectPool(config *config.Config) (*pgxpool.Pool, error) {
	if err := EnsureDatabase(config); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d",
		config.DB.Host, config.DB.Port, config.DB.User, config.DB.Password,
		config.DB.Name, config.DB.SSLMode, config.Batch.Threads,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect with pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	// Ensure PostGIS *in this database*
	if err := enablePostgis(ctx, pool, config); err != nil {
		pool.Close()
		return nil, err
	}

	utils.Info.Println("Connected to PostgreSQL database with pool")
	return pool, nil
}

// create extensions in the *current* DB (the one dsn points to)
func enablePostgis(ctx context.Context, pool *pgxpool.Pool, config *config.Config) error {
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis`); err != nil {
		return fmt.Errorf("failed to enable PostGIS: %w", err)
	}
	utils.Info.Println("PostGIS extension enabled")

	// PostGIS 3.6+ doesn't have separate postgis_raster extension
	// Check PostGIS version and handle accordingly
	var version string
	err := pool.QueryRow(ctx, `SELECT PostGIS_Version()`).Scan(&version)
	if err != nil {
		utils.Warn.Printf("Could not determine PostGIS version: %v", err)
	} else {
		utils.Info.Printf("PostGIS version: %s", version)
	}

	// Try postgis_raster but don't fail if it doesn't exist (PostGIS 3.6+)
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis_raster`); err != nil {
		utils.Warn.Printf("PostGIS Raster extension not available (likely PostGIS 3.6+): %v", err)
		// Don't return error - raster functionality is built into PostGIS 3.6+
	} else {
		utils.Info.Println("PostGIS Raster extension enabled")
	}

	// Try SFCGAL extension
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis_sfcgal`); err != nil {
		utils.Warn.Printf("PostGIS SFCGAL extension not available: %v", err)
		// Don't fail if SFCGAL is not installed
	} else {
		utils.Info.Println("PostGIS SFCGAL extension enabled")
	}

	return nil
}

func ClosePool(pool *pgxpool.Pool) {
	pool.Close()
	utils.Debug.Println("Database pool connection closed")
}
