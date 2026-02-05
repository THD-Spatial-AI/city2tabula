package process

import (
	"fmt"
	"math"
	"math/rand"
	"os"

	"City2TABULA/internal/config"
	"City2TABULA/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner handles execution of jobs and pipelines
type Runner struct {
	config *config.Config
}

// NewRunner creates a new runner instance
func NewRunner(config *config.Config) *Runner {
	return &Runner{
		config: config,
	}
}

// RunPipeline executes all jobs in a pipeline in priority order
func (r *Runner) RunPipeline(pipeline *Pipeline, conn *pgxpool.Pool, workerID int) error {
	// Sort jobs in pipeline based on priority
	sort.Slice(pipeline.Jobs, func(i, j int) bool {
		return pipeline.Jobs[i].Priority < pipeline.Jobs[j].Priority
	})

	// Process the sorted jobs
	for _, job := range pipeline.Jobs {
		if err := r.RunJobWithRetry(job, conn, r.config, workerID); err != nil {
			return fmt.Errorf("pipeline failed at job %s: %w", job.JobType, err)
		}
	}
	return nil
}

// // RunJob executes a single job without retry logic
// func (r *Runner) RunJob(job *Job, conn *pgxpool.Pool, workerID int) error {
// 	return r.runSingleJob(job, conn, workerID)
// }

// RunJobWithRetry executes a single job with retry logic
func (r *Runner) RunJobWithRetry(job *Job, conn *pgxpool.Pool, config *config.Config, workerID int) error {
	var lastErr error
	retryConfig := config.RetryConfig

	maxRetries := retryConfig.MaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := r.runSingleJob(job, conn, workerID)

		if err == nil {
			// Success!
			if attempt > 0 {
				utils.Info.Printf("Job %s succeeded after %d retries", job.JobType, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if this is a deadlock error
		if isDeadlockError(err) {
			// Use special deadlock retry logic
			if attempt < retryConfig.DeadlockRetries {
				delay := r.calculateDeadlockDelay(attempt)
				utils.Warn.Printf("Deadlock detected in job %s (attempt %d/%d), retrying in %v: %v",
					job.JobType, attempt+1, retryConfig.DeadlockRetries+1, delay, err)
				time.Sleep(delay)
				continue
			} else {
				utils.Error.Printf("Job %s failed after %d deadlock retries: %v",
					job.JobType, retryConfig.DeadlockRetries, err)
				return fmt.Errorf("job %s failed after %d deadlock retries: %w", job.JobType, retryConfig.DeadlockRetries, err)
			}
		}

		// For other errors, use regular retry logic
		if attempt < maxRetries {
			delay := r.calculateRetryDelay(attempt, retryConfig)
			utils.Warn.Printf("Job %s failed (attempt %d/%d), retrying in %v: %v",
				job.JobType, attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
		} else {
			utils.Error.Printf("Job %s failed after %d retries: %v", job.JobType, maxRetries, err)
		}
	}

	return fmt.Errorf("job %s failed after %d retries: %w", job.JobType, maxRetries, lastErr)
}

// runSingleJob is the internal method that actually executes a job
func (r *Runner) runSingleJob(job *Job, conn *pgxpool.Pool, workerID int) error {
	utils.Debug.Printf("[Worker %d] Starting job: %s (SQL file: %s)", workerID, job.JobType, job.SQLFile)
	sqlScript, err := r.getSQLScript(job.SQLFile)
	if err != nil {
		return fmt.Errorf("failed to read SQL file %s: %w", job.SQLFile, err)
	}

	// Check if this is a LOD2 or LOD3 job
	if strings.Contains(job.JobType, "LOD2") || strings.Contains(job.JobType, "LOD3") {
		var lod int
		// Extract LOD level from job type for LOD-specific jobs
		if strings.Contains(job.JobType, "LOD2") {
			lod = 2
		}
		if strings.Contains(job.JobType, "LOD3") {
			lod = 3
		}
		if err := utils.ExecuteSQLScript(sqlScript, r.config, conn, lod, job.Params.BuildingIDs); err != nil {
			return fmt.Errorf("job %s failed (SQL file: %s): %w", job.JobType, job.SQLFile, err)
		}
	} else {
		// For other job types, no LOD-specific processing is needed
		if err := utils.ExecuteSQLScript(sqlScript, r.config, conn, 0, nil); err != nil {
			return fmt.Errorf("job %s failed (SQL file: %s): %w", job.JobType, job.SQLFile, err)
		}
	}

	utils.Debug.Printf("[Worker %d] Successfully executed SQL file: %s", workerID, job.SQLFile)
	return nil
}

func (r *Runner) getSQLScript(path string) (string, error) {
	// Read the sql script from the file
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
	// Base delay increases with each attempt
	baseDelay := time.Duration(50+attempt*25) * time.Millisecond
	// Add random jitter to prevent thundering herd
	jitter := time.Duration(rand.Intn(100)) * time.Millisecond

	totalDelay := baseDelay + jitter

	// Cap the maximum delay
	maxDelay := 2 * time.Second
	if totalDelay > maxDelay {
		totalDelay = maxDelay
	}

	return totalDelay
}
