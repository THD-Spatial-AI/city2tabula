package process

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

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
