package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"City2TABULA/internal/config"
	"City2TABULA/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

// Ensure the target database exists by connecting to the bootstrap "postgres" DB
func EnsureDatabase(config *config.Config) error {
	bootstrapDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		config.DB.Host, config.DB.Port, config.DB.User, config.DB.Password, config.DB.SSLMode,
	)

	db, err := sql.Open("postgres", bootstrapDSN)
	if err != nil {
		return fmt.Errorf("connect to bootstrap DB failed: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping bootstrap DB failed: %w", err)
	}

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, config.DB.Name).Scan(&exists); err != nil {
		return fmt.Errorf("check database %s exists: %w", config.DB.Name, err)
	}

	if !exists {
		utils.Info.Printf("Database %s does not exist, creating...", config.DB.Name)
		// NOTE: requires the connecting role to have CREATEDB or be superuser
		if _, err := db.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, config.DB.Name)); err != nil {
			return fmt.Errorf("failed to create database %s: %w", config.DB.Name, err)
		}
		utils.Info.Printf("Database %s created successfully", config.DB.Name)
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
	pool.Exec(ctx, `PostgreSQL version: `) // dummy to ensure connection is alive
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
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgis_sfcgal-3`); err != nil {
		utils.Warn.Printf("PostGIS SFCGAL extension not available: %v", err)
		// Don't fail if SFCGAL is not installed
	} else {
		utils.Info.Println("PostGIS SFCGAL extension enabled")
	}

	return nil
}
