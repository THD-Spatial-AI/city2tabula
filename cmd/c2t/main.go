package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/flags"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
	"github.com/thd-spatial-ai/city2tabula/internal/version"
)

func main() {
	// Parse command-line flags
	f := flags.ParseFlags()
	flagMessages := flags.AllMessages
	// Display current version
	if f.ShowV || f.ShowVersion {
		fmt.Printf("%s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		os.Exit(0)
	}
	if f.Relink && !f.LinkPylovo {
		utils.Error.Fatal("-relink only applies together with -link-pylovo")
	}

	// Start timing
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		utils.Info.Println(strings.Repeat("=", 40))
		utils.Info.Printf("Total runtime: %v", duration)
		utils.Info.Println(strings.Repeat("=", 40))
	}()

	// Initialize logger and config
	utils.InitLogger()
	config := config.LoadConfig()

	if err := config.Validate(); err != nil {
		utils.Error.Fatal("Invalid configuration:", err)
	}

	// Connect to database
	pool, err := db.ConnectPool(&config)
	if err != nil {
		utils.Error.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.ClosePool(pool)
	utils.Info.Println("Database connection established")

	// Execute commands based on flags
	if f.CreateDB {
		utils.Info.Println(flagMessages.CreateDB.Progress)
		if err := db.CreateCompleteDatabase(&config, pool, "", ""); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				utils.Error.Println(flagMessages.CreateDB.Error)
				utils.Info.Println(flagMessages.CreateDB.Custom)
				os.Exit(1)
			}
			utils.Error.Fatalf(flagMessages.CreateDB.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.CreateDB.Success)
	}

	if f.ResetDB {
		utils.Info.Println(flagMessages.ResetDB.Progress)
		if err := db.ResetCompleteDatabase(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.ResetDB.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ResetDB.Success)
		return
	}

	if f.ImportData {
		utils.Info.Println(flagMessages.ImportData.Progress)
		if err := db.ImportAllData(&config, pool, f.Bbox, f.BboxMode); err != nil {
			utils.Error.Fatalf(flagMessages.ImportData.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ImportData.Success)
	}

	if f.ResetC2T {
		utils.Info.Println(flagMessages.ResetC2T.Progress)
		if err := db.ResetCity2TabulaSchemas(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.ResetC2T.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ResetC2T.Success)
	}

	if f.SyncAttribution {
		utils.Info.Println(flagMessages.SyncAttribution.Progress)
		n, err := importer.SyncAttribution(context.Background(), pool, &config)
		if err != nil {
			utils.Error.Fatalf(flagMessages.SyncAttribution.Error+": %v", err)
		}
		utils.Info.Printf("%s: %d rows inserted or changed", flagMessages.SyncAttribution.Success, n)
	}

	if f.CheckAttribution {
		utils.Info.Println(flagMessages.CheckAttribution.Progress)
		failures, err := importer.CheckAttributionURLs(context.Background(), pool, config.DB.Schemas.City2Tabula, attribution.NewURLCheckClient())
		if err != nil {
			utils.Error.Fatalf(flagMessages.CheckAttribution.Error+": %v", err)
		}
		for _, fail := range failures {
			utils.Error.Printf("%s %s: %v", fail.DatasetID, fail.Field, fail.Err)
		}
		if len(failures) > 0 {
			utils.Error.Fatalf("%d attribution URLs failed", len(failures))
		}
		utils.Info.Println(flagMessages.CheckAttribution.Success)
	}

	if f.ExtractFeatures {
		utils.Info.Println(flagMessages.ExtractFeatures.Progress)
		if err := process.RunFeatureExtraction(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.ExtractFeatures.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ExtractFeatures.Success)
	}

	if f.LinkPylovo {
		utils.Info.Println(flagMessages.LinkPylovo.Progress)
		link := process.RunPyLovoLinkBuild
		if f.Relink {
			link = process.RunPyLovoRelink
		}
		if err := link(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.LinkPylovo.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.LinkPylovo.Success)
	}
}
