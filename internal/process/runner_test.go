package process

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestIsDeadLockError verifies that isDeadlockError correctly identifies PostgreSQL deadlock errors.
//
// Use case: RunTaskWithRetry uses this to decide whether to apply deadlock-specific retry logic
// instead of the general retry path.
func TestIsDeadLockError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "given error message containing 'deadlock detected', when called, then returns true",
			err:  errors.New("ERROR: deadlock detected"),
			want: true,
		},
		{
			name: "given error message containing 'sqlstate 40p01', when called, then returns true",
			err:  errors.New("ERROR: sqlstate 40p01"),
			want: true,
		},
		{
			name: "given error message not containing 'deadlock detected', when called, then returns false",
			err:  errors.New("ERROR: some other error"),
			want: false,
		},
		{
			name: "given nil error, when called, then returns false",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			// (error is defined in the test table above)

			// When
			got := isDeadlockError(tc.err)

			// Then
			if got != tc.want {
				t.Errorf("isDeadlockError(%v) = %v; want %v", tc.err, got, tc.want)
			}
		})
	}
}

// fastRetryConfig keeps every delay at 1ms so the retry-exhaustion tests below
// don't actually wait through real backoff sleeps.
func fastRetryConfig() *config.RetryConfig {
	return &config.RetryConfig{
		MaxRetries:      3,
		RetryDelay:      time.Millisecond,
		DeadlockRetries: 2,
		DeadlockDelay:   time.Millisecond,
	}
}

func newTestRunnerAndTask(retryConfig *config.RetryConfig) (*Runner, *Task) {
	cfg := &config.Config{RetryConfig: retryConfig}
	r := NewRunner(cfg)
	task := NewTask("TEST: task", Params{}, "irrelevant.sql", 1, -1)
	return r, task
}

// TestRunTaskWithRetry_SucceedsFirstTry covers the zero-retry happy path: one
// call to runTask, nil error, no retry logging.
func TestRunTaskWithRetry_SucceedsFirstTry(t *testing.T) {
	r, task := newTestRunnerAndTask(fastRetryConfig())
	calls := 0
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		calls++
		return nil
	}

	if err := r.RunTaskWithRetry(task, nil, r.config, 1); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call on first-try success, got %d", calls)
	}
}

// TestRunTaskWithRetry_SucceedsAfterGenericRetries drives the "attempt > 0"
// success-logging branch and the regular (non-deadlock) retry-and-sleep branch:
// two generic failures, then success on the third attempt.
func TestRunTaskWithRetry_SucceedsAfterGenericRetries(t *testing.T) {
	r, task := newTestRunnerAndTask(fastRetryConfig())
	calls := 0
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	}

	if err := r.RunTaskWithRetry(task, nil, r.config, 1); err != nil {
		t.Fatalf("expected nil error after eventual success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", calls)
	}
}

// TestRunTaskWithRetry_GenericRetriesExhausted drives the exhaustion branch:
// runTask always fails with a non-deadlock error, so the loop runs
// MaxRetries+1 times and returns a wrapped "failed after N retries" error.
func TestRunTaskWithRetry_GenericRetriesExhausted(t *testing.T) {
	retryConfig := fastRetryConfig()
	r, task := newTestRunnerAndTask(retryConfig)
	calls := 0
	wantErr := errors.New("persistent failure")
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		calls++
		return wantErr
	}

	err := r.RunTaskWithRetry(task, nil, r.config, 1)
	if err == nil {
		t.Fatal("expected an error after retries are exhausted, got nil")
	}
	if calls != retryConfig.MaxRetries+1 {
		t.Errorf("expected %d calls (MaxRetries+1), got %d", retryConfig.MaxRetries+1, calls)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the final error to wrap the last underlying error, got: %v", err)
	}
}

// TestRunTaskWithRetry_DeadlockSucceedsAfterRetries drives the
// deadlock-detected-then-retry branch: two deadlock errors, then success.
func TestRunTaskWithRetry_DeadlockSucceedsAfterRetries(t *testing.T) {
	r, task := newTestRunnerAndTask(fastRetryConfig())
	calls := 0
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		calls++
		if calls < 3 {
			return errors.New("ERROR: deadlock detected")
		}
		return nil
	}

	if err := r.RunTaskWithRetry(task, nil, r.config, 1); err != nil {
		t.Fatalf("expected nil error after eventual success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 deadlocks + 1 success), got %d", calls)
	}
}

// TestRunTaskWithRetry_DeadlockRetriesExhausted drives the deadlock-specific
// exhaustion branch, which returns after DeadlockRetries+1 attempts - a
// separate limit from MaxRetries, and must not fall through to the generic
// "failed after N retries" message.
func TestRunTaskWithRetry_DeadlockRetriesExhausted(t *testing.T) {
	retryConfig := fastRetryConfig()
	r, task := newTestRunnerAndTask(retryConfig)
	calls := 0
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		calls++
		return errors.New("ERROR: deadlock detected")
	}

	err := r.RunTaskWithRetry(task, nil, r.config, 1)
	if err == nil {
		t.Fatal("expected an error after deadlock retries are exhausted, got nil")
	}
	if calls != retryConfig.DeadlockRetries+1 {
		t.Errorf("expected %d calls (DeadlockRetries+1), got %d", retryConfig.DeadlockRetries+1, calls)
	}
	if got := err.Error(); !strings.Contains(got, "deadlock retries") {
		t.Errorf("expected the deadlock-specific exhaustion message, got: %v", err)
	}
}

// TestRunJob_WrapsTaskFailureError covers RunJob's error-wrap when a task
// exhausts its retries: the returned error must name the failing task and
// wrap the underlying retry-exhaustion error.
func TestRunJob_WrapsTaskFailureError(t *testing.T) {
	r, _ := newTestRunnerAndTask(fastRetryConfig())
	r.runTask = func(task *Task, conn *pgxpool.Pool, workerID int) error {
		return errors.New("boom")
	}

	job := NewJob([]int64{1}, []*Task{
		NewTask("TEST: failing-task", Params{}, "irrelevant.sql", 1, -1),
	})

	err := r.RunJob(job, nil, 1)
	if err == nil {
		t.Fatal("expected RunJob to return an error when a task exhausts retries, got nil")
	}
	if !strings.Contains(err.Error(), "job failed at task") {
		t.Errorf("expected RunJob's error to name the failing task, got: %v", err)
	}
}

// TestRunner_GetSQLScript_MissingFileReturnsError covers the os.ReadFile
// error path - no DB or fixture needed, just a path that doesn't exist.
func TestRunner_GetSQLScript_MissingFileReturnsError(t *testing.T) {
	r := NewRunner(&config.Config{})

	if _, err := r.getSQLScript("/nonexistent/path/xyz.sql"); err == nil {
		t.Error("expected an error for a missing SQL file, got nil")
	}
}

// TestRunSingleTask_MissingSQLFileIsWrapped covers runSingleTask's own wrap
// around a getSQLScript failure (distinct from calling getSQLScript
// directly, above) - the missing-file check happens before conn is ever
// touched, so nil is safe here and no DB is needed.
func TestRunSingleTask_MissingSQLFileIsWrapped(t *testing.T) {
	r := NewRunner(&config.Config{RetryConfig: fastRetryConfig()})
	task := NewTask("TEST: missing file", Params{}, "/nonexistent/path/xyz.sql", 1, -1)

	err := r.runSingleTask(task, nil, 1)
	if err == nil {
		t.Fatal("expected an error for a missing SQL file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read SQL file") {
		t.Errorf("expected runSingleTask's own wrap message, got: %v", err)
	}
}
