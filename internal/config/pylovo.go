package config

// PylovoFDW holds the connection details for the central PyLovo database that
// -link-pylovo reaches through postgres_fdw. Under the database-per-country
// layout each country's City2TABULA database is separate, so pylovo.res and
// pylovo.oth cannot be a local join; the FDW makes them local foreign tables.
//
// Host empty means FDW is disabled: pylovo.res and pylovo.oth must already exist
// as local tables in PYLOVO_SCHEMA. That is the path the integration tests and
// any single-database deployment use.
type PylovoFDW struct {
	Host     string // PYLOVO_FDW_HOST; empty disables FDW setup
	Port     string // PYLOVO_FDW_PORT
	DBName   string // PYLOVO_FDW_DBNAME
	User     string // PYLOVO_FDW_USER; a read-only role on the PyLovo database
	Password string // PYLOVO_FDW_PASSWORD
}

// Enabled reports whether FDW federation is configured.
func (p *PylovoFDW) Enabled() bool {
	return p != nil && p.Host != ""
}

// loadPylovoFDWConfig reads the PYLOVO_FDW_* environment variables.
func loadPylovoFDWConfig() *PylovoFDW {
	return &PylovoFDW{
		Host:     GetEnv("PYLOVO_FDW_HOST", ""),
		Port:     GetEnv("PYLOVO_FDW_PORT", "5432"),
		DBName:   GetEnv("PYLOVO_FDW_DBNAME", ""),
		User:     GetEnv("PYLOVO_FDW_USER", ""),
		Password: GetEnv("PYLOVO_FDW_PASSWORD", ""),
	}
}
