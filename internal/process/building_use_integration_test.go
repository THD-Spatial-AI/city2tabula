//go:build integration

package process_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

const bagAttribution = `{
  "schema_version": 1,
  "dataset_id": "nl-bag-use",
  "provider": "Kadaster",
  "dataset": "Basisregistratie Adressen en Gebouwen (BAG)",
  "licence": "LicenseRef-PDM-1.0",
  "licence_url": "https://creativecommons.org/publicdomain/mark/1.0/",
  "credit": "Kadaster, Basisregistratie Adressen en Gebouwen (BAG), via PDOK",
  "changes": "Test extract."
}`

// stageBuildingUse writes an attribute dataset folder holding csv as its
// building_use file under dir.
func stageBuildingUse(t *testing.T, dir, csv string) {
	t.Helper()
	bag := filepath.Join(dir, "bag")
	if err := os.MkdirAll(bag, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bag, "attribution.json"), []byte(bagAttribution), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bag, importer.BuildingUseFile), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestServe_UseTypeFromBuildingUnits pins the Netherlands path: units staged in
// an attribute dataset are imported with their credit, a building takes the use
// covering most floor area, residential counts any dwelling, a building whose
// units are unmapped stays null, and restaging replaces the rows.
func TestServe_UseTypeFromBuildingUnits(t *testing.T) {
	ctx := context.Background()
	tc := pipelineTestCase{
		country: "netherlands",
		srid:    "28992",
		seeds:   []string{"testdata/netherlands/seed_lod2.sql", "testdata/netherlands/seed_tabula_variant.sql"},
	}
	resetSchemas(t)
	cfg := pipelineConfig(tc)
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	seedDB(t, tc.seeds...)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}
	mixed, unmapped, _ := pylovoLinkFixtureBuildings(t, ctx)

	attributes := t.TempDir()
	cfg.Data = &config.DataPaths{
		Lod2:       t.TempDir(),
		Lod3:       t.TempDir(),
		Tabula:     "data/tabula" + string(filepath.Separator),
		Attributes: attributes,
	}
	importUnits := func(csv string) map[string]onrequest.Building {
		t.Helper()
		stageBuildingUse(t, attributes, csv)
		if _, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil {
			t.Fatalf("SyncAttribution: %v", err)
		}
		if err := importer.ImportBuildingUse(ctx, testPool, cfg); err != nil {
			t.Fatalf("ImportBuildingUse: %v", err)
		}
		buildings, err := onrequest.BuildingsByObjectIDs(ctx, testPool, cfg, []string{mixed, unmapped})
		if err != nil {
			t.Fatalf("BuildingsByObjectIDs: %v", err)
		}
		byID := map[string]onrequest.Building{}
		for _, b := range buildings {
			byID[b.ObjectID] = b
		}
		return byID
	}

	got := importUnits("object_id,use_code,floor_area\n" +
		mixed + ",winkelfunctie,300\n" +
		mixed + ",woonfunctie,120\n" +
		unmapped + ",onbekend,50\n")
	if b := got[mixed]; show(b.UseType) != "retail" || b.Residential == nil || !*b.Residential {
		t.Errorf("%s: use_type %s, residential %v, want retail and true", mixed, show(b.UseType), b.Residential)
	}
	if b := got[unmapped]; b.UseType != nil || b.Residential != nil {
		t.Errorf("%s: use_type %s, want null for unmapped units", unmapped, show(b.UseType))
	}
	ids := onrequest.BuildingDatasetIDs([]onrequest.Building{got[mixed]})
	if !slices.Contains(ids, "nl-bag-use") {
		t.Errorf("credited datasets %v, want nl-bag-use among them", ids)
	}
	credits, err := onrequest.CreditsFor(ctx, testPool, cfg, ids)
	if err != nil {
		t.Fatalf("CreditsFor: %v", err)
	}
	if !slices.ContainsFunc(credits, func(c onrequest.DatasetCredit) bool { return c.DatasetID == "nl-bag-use" }) {
		t.Errorf("credits %+v carry no nl-bag-use row", credits)
	}

	got = importUnits("object_id,use_code,floor_area\n" + mixed + ",kantoorfunctie,90\n")
	if b := got[mixed]; show(b.UseType) != "office" || b.Residential == nil || *b.Residential {
		t.Errorf("after restaging %s: use_type %s, residential %v, want office and false", mixed, show(b.UseType), b.Residential)
	}
}
