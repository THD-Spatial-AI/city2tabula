package process

import (
	"os"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// TestCreateJob_AllJobTypes covers every JobType branch in createJob's switch,
// including MainTable and PyLovoLink, which no production caller currently
// passes (MainDBSetupJobQueue uses LOD2/LOD3 for main-table scripts despite
// the MainTable constant existing) - createJob itself is pure and
// dependency-free, so every case is directly testable without a DB.
func TestCreateJob_AllJobTypes(t *testing.T) {
	cases := []struct {
		jobType      JobType
		wantPrefix   string
		wantLodLevel int
	}{
		{LOD2, "LOD2", 2},
		{LOD3, "LOD3", 3},
		{Function, "FUNCTION", -1},
		{MainTable, "MAIN_TABLE", -1},
		{Supplementary, "SUPPLEMENTARY", -1},
		{SupplementaryTable, "SUPPLEMENTARY_TABLE", -1},
		{PyLovoLink, "PYLOVO_LINK", 2},
	}

	for _, tc := range cases {
		t.Run(string(tc.jobType), func(t *testing.T) {
			batch := []int64{1, 2}
			scripts := []string{"sql/scripts/main/01_create_link_tables.sql", "sql/scripts/main/02_create_link_tables.sql"}

			job := createJob(batch, scripts, tc.jobType)

			if len(job.Tasks) != len(scripts) {
				t.Fatalf("expected %d tasks (one per script), got %d", len(scripts), len(job.Tasks))
			}
			for i, task := range job.Tasks {
				if !strings.HasPrefix(task.TaskType, tc.wantPrefix+":") {
					t.Errorf("task %d: expected TaskType to start with %q, got %q", i, tc.wantPrefix+":", task.TaskType)
				}
				if task.LodLevel != tc.wantLodLevel {
					t.Errorf("task %d: LodLevel = %d, want %d", i, task.LodLevel, tc.wantLodLevel)
				}
				if task.Priority != i+1 {
					t.Errorf("task %d: Priority = %d, want %d", i, task.Priority, i+1)
				}
			}
		})
	}
}

// TestCreateJob_UnknownJobTypeGetsNoPrefix documents the switch's implicit
// default: a JobType with no matching case leaves prefix empty rather than
// erroring - createJob has no fallback/validation for this today.
func TestCreateJob_UnknownJobTypeGetsNoPrefix(t *testing.T) {
	job := createJob([]int64{1}, []string{"script.sql"}, JobType("unknown"))

	if len(job.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(job.Tasks))
	}
	if !strings.HasPrefix(job.Tasks[0].TaskType, ": ") {
		t.Errorf("expected an empty prefix for an unrecognised JobType, got TaskType %q", job.Tasks[0].TaskType)
	}
}

// TestBuildFeatureExtractionQueue_BothLODsProduceJobs covers the LOD3 batch
// loop, which no existing test exercises (every fixture only seeds LOD2 data)
// - BuildFeatureExtractionQueue itself only loads script paths from disk and
// builds Job/Task structs, so no DB is needed to exercise it with non-empty
// LOD3 batches.
func TestBuildFeatureExtractionQueue_BothLODsProduceJobs(t *testing.T) {
	if err := os.Chdir(projectRoot()); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
	cfg := &config.Config{}

	queue, err := BuildFeatureExtractionQueue(cfg, [][]int64{{1, 2}}, [][]int64{{3, 4}})
	if err != nil {
		t.Fatalf("BuildFeatureExtractionQueue: %v", err)
	}
	if queue.Len() != 2 {
		t.Fatalf("expected 1 job per LOD (2 total), got %d", queue.Len())
	}
}

// TestJobQueueBuilders_LoadSQLScriptsFailurePropagates drives every queue
// builder's error-wrap for a failed config.LoadSQLScripts() call by chdir-ing
// somewhere with no sql/ directory tree - one root cause covering five
// separate call sites (BuildFeatureExtractionQueue calls LoadSQLScripts
// directly; the other four go through the shared loadScriptsAndQueue helper).
func TestJobQueueBuilders_LoadSQLScriptsFailurePropagates(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := &config.Config{}

	builders := map[string]func() error{
		"BuildFeatureExtractionQueue": func() error {
			_, err := BuildFeatureExtractionQueue(cfg, nil, nil)
			return err
		},
		"MainDBSetupJobQueue": func() error {
			_, err := MainDBSetupJobQueue(cfg)
			return err
		},
		"SupplementaryDBSetupJobQueue": func() error {
			_, err := SupplementaryDBSetupJobQueue(cfg)
			return err
		},
		"SupplementaryJobQueue": func() error {
			_, err := SupplementaryJobQueue(cfg)
			return err
		},
		"PyLovoLinkJobQueue": func() error {
			_, err := PyLovoLinkJobQueue(cfg, nil)
			return err
		},
	}

	for name, call := range builders {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Errorf("expected %s to propagate a LoadSQLScripts failure, got nil", name)
			}
		})
	}
}

// TestSupplementaryJobQueue_Success covers SupplementaryJobQueue's happy
// path, which no other test calls at all.
func TestSupplementaryJobQueue_Success(t *testing.T) {
	if err := os.Chdir(projectRoot()); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
	cfg := &config.Config{}

	queue, err := SupplementaryJobQueue(cfg)
	if err != nil {
		t.Fatalf("SupplementaryJobQueue: %v", err)
	}
	if queue.Len() == 0 {
		t.Error("expected at least 1 job from the supplementary scripts directory")
	}
}
