package config

import "testing"

func TestDefaultRetryConfig(t *testing.T) {
	c := DefaultRetryConfig()
	if c == nil {
		t.Errorf("expected non-nil config, got nil")
	}
	if c.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", c.MaxRetries)
	}
	if c.DeadlockRetries != 5 {
		t.Errorf("expected DeadlockRetries=5, got %d", c.DeadlockRetries)
	}
	if c.RetryDelay <= 0 {
		t.Errorf("expected RetryDelay > 0, got %v", c.RetryDelay)
	}
	if c.DeadlockDelay <= 0 {
		t.Errorf("expected DeadlockDelay > 0, got %v", c.DeadlockDelay)
	}
}

func TestLoadSchemas(t *testing.T) {
	c := loadSchemas()
	cases := []struct {
		got  string
		want string
	}{
		{
			got:  c.Public,
			want: PublicSchema,
		},
		{
			got:  c.CityDB,
			want: CityDBSchema,
		},
		{
			got:  c.CityDBPkg,
			want: CityDBPkgSchema,
		},
		{
			got:  c.Lod2,
			want: Lod2Schema,
		},
		{
			got:  c.Lod3,
			want: Lod3Schema,
		},
		{
			got:  c.Tabula,
			want: TabulaSchema,
		},
		{
			got:  c.City2Tabula,
			want: City2TabulaSchema,
		},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("expected %q, got %q", tc.want, tc.got)
		}
	}

}

func TestSchemasAll(t *testing.T) {
	c := loadSchemas()
	all := c.All()
	expected := []string{
		PublicSchema,
		CityDBSchema,
		CityDBPkgSchema,
		Lod2Schema,
		Lod3Schema,
		TabulaSchema,
		City2TabulaSchema,
	}
	if len(all) != len(expected) {
		t.Fatalf("expected %d schemas, got %d", len(expected), len(all))
	}
	for i, want := range expected {
		if all[i] != want {
			t.Errorf("expected schema at index %d to be %q, got %q", i, want, all[i])
		}
	}
}
