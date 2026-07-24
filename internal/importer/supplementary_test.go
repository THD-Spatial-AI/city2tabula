package importer

import (
	"os"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func minimalSupplementaryConfig() *config.Config {
	return &config.Config{
		DB: &config.DBConfig{
			Host:     "dbhost",
			Port:     "5432",
			Name:     "testdb",
			User:     "user",
			Password: "pass",
		},
	}
}

func TestGetCsvImportCommand_ArgsStructure(t *testing.T) {
	// Given
	cfg := minimalSupplementaryConfig()

	// When
	cmd, err := getCsvImportCommand("testdata/tabula.csv", cfg)
	if err != nil {
		t.Fatalf("getCsvImportCommand: %v", err)
	}

	// Then: connection flags reflect the config
	args := strings.Join(cmd.Args, " ")
	for _, want := range []string{"-h dbhost", "-p 5432", "-U user", "-d testdb"} {
		if !strings.Contains(args, want) {
			t.Errorf("expected args to contain %q, got %q", want, args)
		}
	}

	// Then: the \copy command targets tabula.tabula with an absolute path
	if !strings.Contains(args, "\\copy tabula.tabula FROM") {
		t.Errorf("expected a \\copy tabula.tabula command, got %q", args)
	}
	wd, _ := os.Getwd()
	if !strings.Contains(args, wd) {
		t.Errorf("expected the CSV path to be made absolute (containing %q), got %q", wd, args)
	}
}

// TestGetCsvImportCommand_InheritsCurrentEnvironment guards against a bug where
// cmd.Env was built as append(cmd.Env, "PGPASSWORD=...") starting from cmd's zero
// value (nil) instead of os.Environ(). Per os/exec, a non-nil Env *replaces* the
// child process's environment rather than extending it - so the psql subprocess
// ran with only PGPASSWORD set and nothing else (no PATH, HOME, locale, ...).
func TestGetCsvImportCommand_InheritsCurrentEnvironment(t *testing.T) {
	// Given: a marker variable known to be in the current process's environment
	t.Setenv("CITY2TABULA_TEST_ENV_MARKER", "present")
	cfg := minimalSupplementaryConfig()

	// When
	cmd, err := getCsvImportCommand("testdata/tabula.csv", cfg)
	if err != nil {
		t.Fatalf("getCsvImportCommand: %v", err)
	}

	// Then: the marker survives into the command's environment
	found := false
	for _, e := range cmd.Env {
		if e == "CITY2TABULA_TEST_ENV_MARKER=present" {
			found = true
		}
	}
	if !found {
		t.Error("expected the current process's environment to be inherited, but the marker var is missing — cmd.Env likely replaced the environment instead of extending it")
	}

	// Then: PGPASSWORD is still set correctly on top of the inherited environment
	wantPassword := false
	for _, e := range cmd.Env {
		if e == "PGPASSWORD=pass" {
			wantPassword = true
		}
	}
	if !wantPassword {
		t.Error("expected PGPASSWORD=pass in cmd.Env")
	}
}

func TestImportCsvWithPsql_ReturnsErrorOnFailure(t *testing.T) {
	// Given: a config pointing at a non-existent database
	cfg := &config.Config{
		DB: &config.DBConfig{
			Host:     "127.0.0.1",
			Port:     "9999",
			Name:     "nonexistent",
			User:     "nobody",
			Password: "wrong",
		},
	}

	// When: psql command is executed against an unreachable host
	err := ImportCsvWithPsql("/dev/null", cfg)

	// Then: error is propagated (psql exits non-zero)
	if err == nil {
		t.Error("expected error from psql against unreachable host, got nil")
	}
}

// unreachableSupplementaryConfig points at both an unreachable Postgres host
// and a Tabula data dir with no matching CSV - psql fails to even connect
// before it would notice the missing file, so ImportTabulaData/
// ImportCsvWithPsql fail for real without needing any database at all.
func unreachableSupplementaryConfig() *config.Config {
	return &config.Config{
		DB: &config.DBConfig{
			Host: "127.0.0.1", Port: "9999", Name: "nonexistent", User: "nobody", Password: "wrong",
		},
		Data:    &config.DataPaths{Tabula: "/nonexistent/"},
		Country: "germany",
	}
}

// TestImportTabulaData_ReturnsWrappedErrorOnFailure covers ImportTabulaData's
// own error wrap around a real ImportCsvWithPsql failure - ImportTabulaData
// never touches conn, so nil is safe here.
func TestImportTabulaData_ReturnsWrappedErrorOnFailure(t *testing.T) {
	err := ImportTabulaData(nil, unreachableSupplementaryConfig())
	if err == nil {
		t.Fatal("expected an error from psql against an unreachable host, got nil")
	}
	if !strings.Contains(err.Error(), "failed to import Tabula data") {
		t.Errorf("expected ImportTabulaData's own error wrap, got: %v", err)
	}
}

// TestImportSupplementaryData_ReturnsErrorWhenImportTabulaDataFails covers
// ImportSupplementaryData's passthrough of ImportTabulaData's failure - the
// failure happens on the very first step, before process.SupplementaryJobQueue
// or conn are ever touched, so nil is safe here too.
func TestImportSupplementaryData_ReturnsErrorWhenImportTabulaDataFails(t *testing.T) {
	err := ImportSupplementaryData(nil, unreachableSupplementaryConfig())
	if err == nil {
		t.Fatal("expected ImportSupplementaryData to fail when ImportTabulaData fails, got nil")
	}
}
