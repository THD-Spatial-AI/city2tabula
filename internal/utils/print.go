package utils

import (
	"strings"
	"time"
)

// PrintJobInfo prints detailed information about a job using basic types
func PrintJobInfo(jobID, jobType string, createdAt time.Time, buildingIDs []int64, tableNames, schemaNames []string) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Job Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Job ID               : %s", jobID)
	Info.Printf("Job Type             : %s", jobType)
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

func PrintPipelineInfo(pipelineID string, buildingIDs []int64, jobCount int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Pipeline Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Pipeline ID            : %s", pipelineID)
	Info.Printf("Total Building IDs     : %d", len(buildingIDs))
	Info.Printf("Total Jobs             : %d", jobCount)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}

func PrintPipelineQueueInfo(totalPipelines int, totalJobsInPipeline int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Pipeline Queue Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Total Pipelines        : %d", totalPipelines)
	Info.Printf("Total Jobs per Pipeline: %d", totalJobsInPipeline)
	Info.Printf("Total Jobs             : %d", totalPipelines*totalJobsInPipeline)
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

func PrintRunnerInfo(jobID, pipelineID string, totalJobsInPipeline int) {
	Info.Printf("") // extra spacing before block
	Info.Printf("Runner Details:")
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("Job ID:                %s", jobID)
	Info.Printf("Pipeline ID:           %s", pipelineID)
	Info.Printf("Total Jobs in Pipeline: %d", totalJobsInPipeline)
	Info.Printf("%s", strings.Repeat("-", 40))
	Info.Printf("")
}
