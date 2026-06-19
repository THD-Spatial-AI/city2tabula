package process

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner handles execution of tasks and jobs
type Runner struct {
	config *config.Config
}

// NewRunner creates a new runner instance
func NewRunner(config *config.Config) *Runner {
	return &Runner{
		config: config,
	}
}

// RunJob executes all tasks in a job in priority order
func (r *Runner) RunJob(job *Job, conn *pgxpool.Pool, workerID int) error {
	// Sort tasks in job based on priority
	sort.Slice(job.Tasks, func(i, j int) bool {
		return job.Tasks[i].Priority < job.Tasks[j].Priority
	})

	// Process the sorted tasks
	for _, task := range job.Tasks {
		if err := r.RunTaskWithRetry(task, conn, r.config, workerID); err != nil {
			return fmt.Errorf("job failed at task %s: %w", task.TaskType, err)
		}
	}
	return nil
}

// RunTaskWithRetry executes a single task with retry logic
func (r *Runner) RunTaskWithRetry(task *Task, conn *pgxpool.Pool, config *config.Config, workerID int) error {
	var lastErr error
	retryConfig := config.RetryConfig

	maxRetries := retryConfig.MaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := r.runSingleTask(task, conn, workerID)

		if err == nil {
			if attempt > 0 {
				utils.Info.Printf("Task %s succeeded after %d retries", task.TaskType, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if this is a deadlock error
		if isDeadlockError(err) {
			if attempt < retryConfig.DeadlockRetries {
				utils.Warn.Printf("Deadlock detected in task %s (attempt %d/%d), retrying in %v: %v",
					task.TaskType, attempt+1, retryConfig.DeadlockRetries+1, retryConfig.DeadlockDelay, err)
				time.Sleep(retryConfig.DeadlockDelay)
				continue
			}
			utils.Error.Printf("Task %s failed after %d deadlock retries: %v",
				task.TaskType, retryConfig.DeadlockRetries, err)
			return fmt.Errorf("task %s failed after %d deadlock retries: %w", task.TaskType, retryConfig.DeadlockRetries, err)
		}

		// For other errors, use regular retry logic
		if attempt < maxRetries {
			utils.Warn.Printf("Task %s failed (attempt %d/%d), retrying in %v: %v",
				task.TaskType, attempt+1, maxRetries+1, retryConfig.RetryDelay, err)
			time.Sleep(retryConfig.RetryDelay)
		} else {
			utils.Error.Printf("Task %s failed after %d retries: %v", task.TaskType, maxRetries, err)
		}
	}

	return fmt.Errorf("task %s failed after %d retries: %w", task.TaskType, maxRetries, lastErr)
}

// runSingleTask is the internal method that actually executes a task
func (r *Runner) runSingleTask(task *Task, conn *pgxpool.Pool, workerID int) error {
	utils.Debug.Printf("[Worker %d] Starting task: %s (SQL file: %s)", workerID, task.TaskType, task.SQLFile)
	sqlScript, err := r.getSQLScript(task.SQLFile)
	if err != nil {
		return fmt.Errorf("failed to read SQL file %s: %w", task.SQLFile, err)
	}

	// task.LodLevel carries 2, 3, or -1 (no LOD context). Pass it directly so
	// any task type — including future link types — gets the correct lod_schema
	// substitution without requiring string parsing of the task name.
	buildingIDs := task.Params.BuildingIDs
	if task.LodLevel == -1 {
		buildingIDs = nil
	}
	if err := executeSQLScript(sqlScript, r.config, conn, task.LodLevel, buildingIDs); err != nil {
		return fmt.Errorf("task %s failed (SQL file: %s): %w", task.TaskType, task.SQLFile, err)
	}

	utils.Debug.Printf("[Worker %d] Successfully executed SQL file: %s", workerID, task.SQLFile)
	return nil
}

func (r *Runner) getSQLScript(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isDeadlockError checks if the error is a PostgreSQL deadlock error
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "deadlock detected") ||
		strings.Contains(errStr, "sqlstate 40p01")
}
