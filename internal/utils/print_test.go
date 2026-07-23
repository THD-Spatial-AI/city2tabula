package utils_test

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// captureInfo swaps utils.Info for a fresh logger writing to a buffer, restoring
// the original *log.Logger afterward. Using SetOutput on the shared logger instead
// would mutate it in place, so saving/restoring the same pointer wouldn't undo
// anything - the swap is what makes restoration actually work.
func captureInfo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := utils.Info
	utils.Info = log.New(&buf, orig.Prefix(), orig.Flags())
	t.Cleanup(func() { utils.Info = orig })
	return &buf
}

func TestPrintTaskInfo(t *testing.T) {
	t.Run("5 or fewer building IDs are all shown", func(t *testing.T) {
		buf := captureInfo(t)

		utils.PrintTaskInfo("task-1", "extract", time.Now(), []int64{1, 2, 3}, []string{"lod2_building"}, []string{"city2tabula"})

		out := buf.String()
		for _, want := range []string{"task-1", "extract", "lod2_building", "city2tabula", "[1 2 3]"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected output to contain %q, got:\n%s", want, out)
			}
		}
	})

	t.Run("more than 5 building IDs are truncated", func(t *testing.T) {
		buf := captureInfo(t)

		utils.PrintTaskInfo("task-2", "extract", time.Now(), []int64{1, 2, 3, 4, 5, 6, 7}, nil, nil)

		out := buf.String()
		if !strings.Contains(out, "Building IDs:        [1 2 3 4 5]...") {
			t.Errorf("expected building IDs truncated to the first 5 with a trailing ellipsis, got:\n%s", out)
		}
	})
}

func TestPrintJobInfo(t *testing.T) {
	buf := captureInfo(t)

	utils.PrintJobInfo("job-1", []int64{1, 2}, 4)

	out := buf.String()
	for _, want := range []string{"job-1", "Total Building IDs     : 2", "Total Tasks            : 4"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPrintJobQueueInfo(t *testing.T) {
	buf := captureInfo(t)

	utils.PrintJobQueueInfo(3, 8, &config.BatchConfig{Threads: 4})

	out := buf.String()
	for _, want := range []string{
		"Total Jobs              : 3",
		"Total Tasks per Job     : 8",
		"Total Tasks             : 24", // 3 * 8
		"Total Workers           : 4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPrintWorkerInfo(t *testing.T) {
	buf := captureInfo(t)

	utils.PrintWorkerInfo(7)

	if out := buf.String(); !strings.Contains(out, "Worker ID              : 7") {
		t.Errorf("expected output to contain the worker ID, got:\n%s", out)
	}
}

func TestPrintRunnerInfo(t *testing.T) {
	buf := captureInfo(t)

	utils.PrintRunnerInfo("task-1", "job-1", 5)

	out := buf.String()
	for _, want := range []string{"task-1", "job-1", "Total Tasks in Job:    5"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
