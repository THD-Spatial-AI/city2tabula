package process

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// setupPylovoFDW wires the current database to the central PyLovo database via
// postgres_fdw so 01_build_pylovo_link.sql can read pylovo.res and pylovo.oth as
// local foreign tables. It is a no-op when PYLOVO_FDW_HOST is unset, in which
// case those tables must already exist in cfg.DB.Schemas.Pylvo.
//
// The server, user mapping and foreign schema are dropped and recreated on every
// call, so a changed host or credential in .env takes effect without hand-run
// DDL. `extensions 'postgis'` on the server is what lets the bbox pre-filter run
// on the PyLovo side instead of pulling every res/oth row across the connection.
func setupPylovoFDW(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	fdw := cfg.PylovoFDW
	if !fdw.Enabled() {
		return nil
	}

	schema := cfg.DB.Schemas.Pylvo
	if schema == "" || schema == cfg.DB.Schemas.Public {
		return fmt.Errorf("refusing to import PyLovo foreign tables into schema %q", schema)
	}
	schemaIdent := pgx.Identifier{schema}.Sanitize()

	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS postgres_fdw`,
		`DROP SERVER IF EXISTS pylovo_srv CASCADE`,
		fmt.Sprintf(
			`CREATE SERVER pylovo_srv FOREIGN DATA WRAPPER postgres_fdw `+
				`OPTIONS (host %s, port %s, dbname %s, extensions 'postgis')`,
			quoteSQLLiteral(fdw.Host), quoteSQLLiteral(fdw.Port), quoteSQLLiteral(fdw.DBName),
		),
		fmt.Sprintf(
			`CREATE USER MAPPING FOR CURRENT_USER SERVER pylovo_srv OPTIONS (user %s, password %s)`,
			quoteSQLLiteral(fdw.User), quoteSQLLiteral(fdw.Password),
		),
		fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schemaIdent),
		fmt.Sprintf(`CREATE SCHEMA %s`, schemaIdent),
		fmt.Sprintf(
			`IMPORT FOREIGN SCHEMA public LIMIT TO (res, oth) FROM SERVER pylovo_srv INTO %s`,
			schemaIdent,
		),
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("PyLovo FDW setup failed at %q: %w", sqlPrefix(stmt), err)
		}
	}

	utils.Info.Printf("PyLovo FDW ready: %s.res/oth via %s@%s:%s/%s",
		schema, fdw.User, fdw.Host, fdw.Port, fdw.DBName)
	return nil
}

// quoteSQLLiteral renders s as a single-quoted SQL string literal. Used only for
// postgres_fdw OPTIONS values, which cannot be passed as query parameters.
func quoteSQLLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// sqlPrefix returns the first line of a statement, capped, for error messages.
func sqlPrefix(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	if i := strings.IndexByte(stmt, '\n'); i >= 0 {
		stmt = stmt[:i]
	}
	if len(stmt) > 60 {
		stmt = stmt[:60]
	}
	return stmt
}
