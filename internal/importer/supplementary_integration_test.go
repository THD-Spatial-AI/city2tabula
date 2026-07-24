//go:build integration

package importer_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testHost string
	testPort string
	testPool *pgxpool.Pool
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "postgis/postgis:17-3.4",
		Env: map[string]string{
			"POSTGRES_DB":       "postgres",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").AsRegexp(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start PostGIS container: %v", err)
	}
	defer container.Terminate(ctx)

	testHost, err = container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get container port: %v", err)
	}
	testPort = port.Port()

	if err := waitForBootstrapDB(ctx); err != nil {
		log.Fatalf("PostgreSQL not ready: %v", err)
	}

	pool, err := db.ConnectPool(testConfig())
	if err != nil {
		log.Fatalf("failed to connect shared test pool: %v", err)
	}
	testPool = pool
	defer db.ClosePool(testPool)

	os.Exit(m.Run())
}

// waitForBootstrapDB mirrors internal/db's own TestMain: PostGIS images do
// extra initialization after the "ready" log line.
func waitForBootstrapDB(ctx context.Context) error {
	bootstrapDSN := fmt.Sprintf("host=%s port=%s user=test password=test dbname=postgres sslmode=disable", testHost, testPort)
	pool, err := pgxpool.New(ctx, bootstrapDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = pool.Ping(ctx); lastErr == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func testConfig() *config.Config {
	return &config.Config{
		DB: &config.DBConfig{
			Host: testHost, Port: testPort, Name: "importer_dbtest", User: "test", Password: "test", SSLMode: "disable",
			Schemas: &config.Schemas{Lod2: "lod2", Lod3: "lod3", City2Tabula: "city2tabula", Tabula: "tabula"},
			Tables:  &config.Tables{Tabula: "tabula", TabulaVariant: "tabula_variant"},
		},
		CityDB:      &config.CityDB{SRID: "25832", SRSName: "urn:ogc:def:crs:EPSG::25832"},
		City2Tabula: &config.City2TabulaConfig{RoomHeight: "2.5"},
		Batch:       &config.BatchConfig{Threads: 2},
		RetryConfig: config.DefaultRetryConfig(),
	}
}

// setupMinimalTabulaFixture runs the real sql/schema/supplementary/ DDL (which
// creates both the real 216-column tabula.tabula and city2tabula.tabula_variant,
// the INSERT target 01_extract_tabula_attributes.sql writes to), then replaces
// tabula.tabula with a minimal stand-in table - only the 17 columns that real
// script actually SELECTs from, not the real TABULA project's 216-column
// format this repo doesn't ship a fixture for. Column order matches the CSV
// column order, since \copy ... CSV HEADER (no explicit column list) maps
// purely positionally.
func setupMinimalTabulaFixture(t *testing.T, ctx context.Context, cfg *config.Config) {
	t.Helper()
	t.Chdir(projectRoot())

	if _, err := testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.City2Tabula+` CASCADE`); err != nil {
		t.Fatalf("failed to reset city2tabula schema: %v", err)
	}
	if _, err := testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.Tabula+` CASCADE`); err != nil {
		t.Fatalf("failed to reset tabula schema: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.City2Tabula+` CASCADE`)
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.Tabula+` CASCADE`)
	})

	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	if _, err := testPool.Exec(ctx, `
		DROP TABLE IF EXISTS tabula.tabula;
		CREATE TABLE tabula.tabula (
			id INTEGER,
			"Code_BuildingVariant" TEXT,
			"Number_BuildingVariant" INTEGER,
			"Year1_Building" INTEGER,
			"Year2_Building" INTEGER,
			"V_C" DOUBLE PRECISION,
			"A_C_National" DOUBLE PRECISION,
			"n_Storey" INTEGER,
			"Code_ComplexFootprint" TEXT,
			"Code_AttachedNeighbours" TEXT,
			"Code_ComplexRoof" TEXT,
			"A_Roof_1" DOUBLE PRECISION,
			"A_Roof_2" DOUBLE PRECISION,
			"A_Wall_1" DOUBLE PRECISION,
			"A_Wall_2" DOUBLE PRECISION,
			"A_Wall_3" DOUBLE PRECISION,
			"A_C_ExtDim" DOUBLE PRECISION,
			"Code_BuildingSizeClass" TEXT
		);`,
	); err != nil {
		t.Fatalf("failed to create minimal tabula.tabula fixture: %v", err)
	}
}

// writeMinimalTabulaCSV writes one data row matching setupMinimalTabulaFixture's
// column order exactly - empty fields become NULL under COPY ... CSV, which
// COALESCE(..., 0) in 01_extract_tabula_attributes.sql then zeroes out.
func writeMinimalTabulaCSV(t *testing.T, dir, country string) {
	t.Helper()
	header := `id,Code_BuildingVariant,Number_BuildingVariant,Year1_Building,Year2_Building,V_C,A_C_National,n_Storey,Code_ComplexFootprint,Code_AttachedNeighbours,Code_ComplexRoof,A_Roof_1,A_Roof_2,A_Wall_1,A_Wall_2,A_Wall_3,A_C_ExtDim,Code_BuildingSizeClass`
	row := `1,TEST.VARIANT.001,1,1990,2000,500.0,150.0,3,Regular,B_Alone,Simple,60.0,,40.0,40.0,,145.0,SFH`
	csv := header + "\n" + row + "\n"
	path := filepath.Join(dir, country+".csv")
	if err := os.WriteFile(path, []byte(csv), 0644); err != nil {
		t.Fatalf("failed to write CSV fixture: %v", err)
	}
}

// TestImportSupplementaryData_SupplementaryJobQueueFailure isolates
// process.SupplementaryJobQueue's LoadSQLScripts-failure branch: ImportTabulaData
// succeeds for real (the minimal fixture above), then chdir to an empty temp
// dir makes the SQL-script-loading step fail.
func TestImportSupplementaryData_SupplementaryJobQueueFailure(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	setupMinimalTabulaFixture(t, ctx, cfg)

	dataDir := t.TempDir()
	writeMinimalTabulaCSV(t, dataDir, "germany")
	cfg.Data = &config.DataPaths{Tabula: dataDir + string(filepath.Separator)}
	cfg.Country = "germany"

	t.Chdir(t.TempDir()) // after ImportTabulaData's success, before SupplementaryJobQueue's LoadSQLScripts

	err := importer.ImportSupplementaryData(testPool, cfg)
	if err == nil {
		t.Fatal("expected an error when SupplementaryJobQueue's LoadSQLScripts fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to setup DB queue") {
		t.Errorf("expected ImportSupplementaryData's own error wrap, got: %v", err)
	}
}

// TestImportSupplementaryData_Success drives the whole function to its real
// success return: ImportTabulaData imports the minimal fixture CSV for real,
// then the real sql/scripts/supplementary/01_extract_tabula_attributes.sql
// runs against it and inserts a computed row into city2tabula.tabula_variant.
func TestImportSupplementaryData_Success(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	setupMinimalTabulaFixture(t, ctx, cfg)

	dataDir := t.TempDir()
	writeMinimalTabulaCSV(t, dataDir, "germany")
	cfg.Data = &config.DataPaths{Tabula: dataDir + string(filepath.Separator)}
	cfg.Country = "germany"

	if err := importer.ImportSupplementaryData(testPool, cfg); err != nil {
		t.Fatalf("ImportSupplementaryData: %v", err)
	}

	var (
		code                                                                                     string
		year1, year2, storeys, footprintComplexity, attachedNeighbour, roofComplexity, sizeClass int
		maxVolume, totalArea, footprintArea, areaRoof, areaWall, areaFloor                       float64
	)
	err := testPool.QueryRow(ctx, `
		SELECT tabula_variant_code, construction_year_1, construction_year_2, max_volume, total_area,
		       footprint_area, number_of_storeys, footprint_complexity, attached_neighbour_class,
		       roof_complexity, area_total_roof, area_total_wall, area_total_floor, building_size_class
		FROM city2tabula.tabula_variant WHERE tabula_variant_code_id = 1`,
	).Scan(&code, &year1, &year2, &maxVolume, &totalArea, &footprintArea, &storeys,
		&footprintComplexity, &attachedNeighbour, &roofComplexity, &areaRoof, &areaWall, &areaFloor, &sizeClass)
	if err != nil {
		t.Fatalf("failed to read the inserted tabula_variant row: %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"tabula_variant_code", code, "TEST.VARIANT.001"},
		{"construction_year_1", year1, 1990},
		{"construction_year_2", year2, 2000},
		{"max_volume", maxVolume, 500.0},
		{"total_area", totalArea, 150.0},
		{"footprint_area", footprintArea, 50.0}, // 150.0 / n_Storey(3)
		{"number_of_storeys", storeys, 3},
		{"footprint_complexity", footprintComplexity, 1},   // Regular
		{"attached_neighbour_class", attachedNeighbour, 0}, // B_Alone
		{"roof_complexity", roofComplexity, 0},             // Simple
		{"area_total_roof", areaRoof, 60.0},                // A_Roof_1 + COALESCE(A_Roof_2, 0)
		{"area_total_wall", areaWall, 80.0},                // A_Wall_1 + A_Wall_2 + COALESCE(A_Wall_3, 0)
		{"area_total_floor", areaFloor, 145.0},             // A_C_ExtDim
		{"building_size_class", sizeClass, 0},              // SFH
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
