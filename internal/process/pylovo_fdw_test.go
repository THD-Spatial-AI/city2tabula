package process

import (
	"context"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func TestQuoteSQLLiteral(t *testing.T) {
	cases := map[string]string{
		"pylovo_db":    "'pylovo_db'",
		"":             "''",
		"o'brien":      "'o''brien'",
		"a'b'c":        "'a''b''c'",
		"127.0.0.1":    "'127.0.0.1'",
		"x') DROP; --": "'x'') DROP; --'",
	}
	for in, want := range cases {
		if got := quoteSQLLiteral(in); got != want {
			t.Errorf("quoteSQLLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetupPylovoFDW_NoOpWhenDisabled(t *testing.T) {
	cfg := &config.Config{
		DB:        &config.DBConfig{Schemas: &config.Schemas{Pylvo: "public", Public: "public"}},
		PylovoFDW: &config.PylovoFDW{}, // Host empty -> disabled
	}
	// pool is nil: the function must return before touching it.
	if err := setupPylovoFDW(context.Background(), nil, cfg); err != nil {
		t.Fatalf("expected no-op nil, got %v", err)
	}
}

func TestSetupPylovoFDW_RefusesUnsafeSchema(t *testing.T) {
	cfg := &config.Config{
		DB: &config.DBConfig{Schemas: &config.Schemas{Pylvo: "public", Public: "public"}},
		PylovoFDW: &config.PylovoFDW{
			Host: "db.example", Port: "5432", DBName: "pylovo_db",
			User: "reader", Password: "secret",
		},
	}
	err := setupPylovoFDW(context.Background(), nil, cfg)
	if err == nil || !strings.Contains(err.Error(), "refusing to import") {
		t.Fatalf("expected refusal to drop the public schema, got %v", err)
	}
}
