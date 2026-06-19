package process

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// projectRoot returns the absolute path to the repository root so tests that
// load SQL files from disk can chdir to the right location.
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestPyLovoLinkJobQueue_EmptyBatches(t *testing.T) {
	// Given: a valid config pointing at real SQL files on disk
	if err := os.Chdir(projectRoot()); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
	cfg := &config.Config{
		DB: &config.DBConfig{
			Schemas: &config.Schemas{
				City2Tabula: "city2tabula",
				Lod2:        "lod2",
				Pylvo:       "public",
			},
			Tables: &config.Tables{
				Tabula:        "tabula",
				TabulaVariant: "tabula_variant",
			},
		},
		CityDB:      &config.CityDB{SRID: "25832"},
		City2Tabula: &config.City2TabulaConfig{RoomHeight: "2.5", LinkGridSize: 1000},
	}

	// When: no batches are passed
	queue, err := PyLovoLinkJobQueue(cfg, nil)

	// Then: queue is created with no jobs
	if err != nil {
		t.Fatalf("PyLovoLinkJobQueue returned unexpected error: %v", err)
	}
	if queue.Len() != 0 {
		t.Errorf("expected empty queue, got %d jobs", queue.Len())
	}
}

func TestPyLovoLinkJobQueue_WithBatches(t *testing.T) {
	// Given
	if err := os.Chdir(projectRoot()); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
	cfg := &config.Config{
		DB: &config.DBConfig{
			Schemas: &config.Schemas{
				City2Tabula: "city2tabula",
				Lod2:        "lod2",
				Pylvo:       "public",
			},
			Tables: &config.Tables{
				Tabula:        "tabula",
				TabulaVariant: "tabula_variant",
			},
		},
		CityDB:      &config.CityDB{SRID: "25832"},
		City2Tabula: &config.City2TabulaConfig{RoomHeight: "2.5", LinkGridSize: 1000},
	}
	batches := [][]int64{{1, 2, 3}, {4, 5}}

	// When
	queue, err := PyLovoLinkJobQueue(cfg, batches)

	// Then: one job per batch
	if err != nil {
		t.Fatalf("PyLovoLinkJobQueue returned unexpected error: %v", err)
	}
	if queue.Len() != len(batches) {
		t.Errorf("expected %d jobs, got %d", len(batches), queue.Len())
	}
}

func TestLodSchema(t *testing.T) {
	// Given
	cfg := &config.Config{
		DB: &config.DBConfig{
			Schemas: &config.Schemas{
				Lod2: "lod2_schema",
				Lod3: "lod3_schema",
			},
		},
	}

	cases := []struct {
		name    string
		lod     int
		want    string
		wantErr bool
	}{
		{"LOD2 returns lod2 schema", 2, "lod2_schema", false},
		{"LOD3 returns lod3 schema", 3, "lod3_schema", false},
		{"unsupported LOD returns error", 4, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			got, err := lodSchema(cfg, tc.lod)

			// Then
			if (err != nil) != tc.wantErr {
				t.Errorf("lodSchema(%d) error = %v, wantErr %v", tc.lod, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("lodSchema(%d) = %q, want %q", tc.lod, got, tc.want)
			}
		})
	}
}
