package utils

import (
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// PrintTaskInfo prints detailed information about a task
func PrintTaskInfo(taskID, taskType string, createdAt time.Time, buildingIDs []int64, tableNames, schemaNames []string) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Task Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Task ID              : %s", taskID)
	Info.Printf("Task Type            : %s", taskType)
	Info.Printf("Created At           : %s", createdAt.Format("2006-01-02 15:04:05"))
	Info.Printf("Total Building IDs   : %d", len(buildingIDs))

	// Show first 5 IDs only (avoid log spam)
	if len(buildingIDs) > 5 {
		Info.Printf("Building IDs:        %v...", buildingIDs[:5])
	} else {
		Info.Printf("Building IDs:        %v", buildingIDs)
	}

	Info.Printf("Table Names           : %v", tableNames)
	Info.Printf("Schema Names          : %v", schemaNames)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}

// PrintJobInfo prints detailed information about a job
func PrintJobInfo(jobID string, buildingIDs []int64, taskCount int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Job Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Job ID                 : %s", jobID)
	Info.Printf("Total Building IDs     : %d", len(buildingIDs))
	Info.Printf("Total Tasks            : %d", taskCount)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}

// PrintJobQueueInfo prints summary information about the job queue before processing
func PrintJobQueueInfo(totalJobs int, totalTasksInJob int, batchConfig *config.BatchConfig) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Job Queue Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Total Jobs              : %d", totalJobs)
	Info.Printf("Total Tasks per Job     : %d", totalTasksInJob)
	Info.Printf("Total Tasks             : %d", totalJobs*totalTasksInJob)
	Info.Printf("Total Workers           : %d", batchConfig.Threads)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
	Info.Printf("Extracting Features, this may take a while...")
	Info.Printf("")
}

func PrintWorkerInfo(workerID int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Worker Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Worker ID              : %d", workerID)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}

func PrintRunnerInfo(taskID, jobID string, totalTasksInJob int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Runner Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Task ID:               %s", taskID)
	Info.Printf("Job ID:                %s", jobID)
	Info.Printf("Total Tasks in Job:    %d", totalTasksInJob)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}
