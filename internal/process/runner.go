package process

import (
	"fmt"
	"math"
	"math/rand"
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
				delay := r.calculateDeadlockDelay(attempt)
				utils.Warn.Printf("Deadlock detected in task %s (attempt %d/%d), retrying in %v: %v",
					task.TaskType, attempt+1, retryConfig.DeadlockRetries+1, delay, err)
				time.Sleep(delay)
				continue
			} else {
				utils.Error.Printf("Task %s failed after %d deadlock retries: %v",
					task.TaskType, retryConfig.DeadlockRetries, err)
				return fmt.Errorf("task %s failed after %d deadlock retries: %w", task.TaskType, retryConfig.DeadlockRetries, err)
			}
		}

		// For other errors, use regular retry logic
		if attempt < maxRetries {
			delay := r.calculateRetryDelay(attempt, retryConfig)
			utils.Warn.Printf("Task %s failed (attempt %d/%d), retrying in %v: %v",
				task.TaskType, attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
		} else {
			utils.Error.Printf("Task %s failed after %d retries: %v", task.TaskType, maxRetries, err)
		}
	}

	return fmt.Errorf("task %s failed after %d retries: %w", task.TaskType, maxRetries, lastErr)
}

// runSingleTask reads the SQL file and executes it against the database.
// task.LodLevel drives which LOD schema and building IDs are used — -1 means no LOD context (schema setup, functions, etc.).
func (r *Runner) runSingleTask(task *Task, conn *pgxpool.Pool, workerID int) error {
	utils.Debug.Printf("[Worker %d] Starting task: %s (SQL file: %s)", workerID, task.TaskType, task.SQLFile)

	data, err := os.ReadFile(task.SQLFile)
	if err != nil {
		return fmt.Errorf("failed to read SQL file %s: %w", task.SQLFile, err)
	}

	// Check if this is a LOD2 or LOD3 task
	if strings.Contains(task.TaskType, "LOD2") || strings.Contains(task.TaskType, "LOD3") {
		var lod int
		if strings.Contains(task.TaskType, "LOD2") {
			lod = 2
		}
		if strings.Contains(task.TaskType, "LOD3") {
			lod = 3
		}
		if err := executeSQLScript(sqlScript, r.config, conn, lod, task.Params.BuildingIDs); err != nil {
			return fmt.Errorf("task %s failed (SQL file: %s): %w", task.TaskType, task.SQLFile, err)
		}
	} else {
		if err := executeSQLScript(sqlScript, r.config, conn, 0, nil); err != nil {
			return fmt.Errorf("task %s failed (SQL file: %s): %w", task.TaskType, task.SQLFile, err)
		}
	}

	utils.Debug.Printf("[Worker %d] Successfully executed SQL file: %s", workerID, task.SQLFile)
	return nil
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

// calculateRetryDelay calculates delay for regular retries using exponential backoff
func (r *Runner) calculateRetryDelay(attempt int, config *config.RetryConfig) time.Duration {
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt))

	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}

// calculateDeadlockDelay calculates delay specifically for deadlock retries
func (r *Runner) calculateDeadlockDelay(attempt int) time.Duration {
	baseDelay := time.Duration(50+attempt*25) * time.Millisecond
	jitter := time.Duration(rand.Intn(100)) * time.Millisecond

	totalDelay := baseDelay + jitter

	maxDelay := 2 * time.Second
	if totalDelay > maxDelay {
		totalDelay = maxDelay
	}

	return totalDelay
}
