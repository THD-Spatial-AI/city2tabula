package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/flags"
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
		if err := db.CreateCompleteDatabase(&config, pool); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				if strings.Contains(config.DB.Host, "docker") {
					utils.Error.Println(flagMessages.CreateDB.Error)
					utils.Info.Println(flagMessages.CreateDB.Custom)
					os.Exit(1)
				}
			}
			utils.Info.Println("Consider changing the database name in .env file or reset the existing database using the -reset-db flag.")
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

	if f.ResetCityDB {
		utils.Info.Println(flagMessages.ResetCityDB.Progress)
		if err := db.ResetCityDBOnly(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.ResetCityDB.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ResetCityDB.Success)
	}

	if f.ResetC2T {
		utils.Info.Println(flagMessages.ResetC2T.Progress)
		// Drop City2TABULA schemas
		c2tSchemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
		for _, schema := range c2tSchemas {
			if err := db.DropSchemaIfExists(pool, schema); err != nil {
				utils.Warn.Printf("Warning dropping schema %s: %v", schema, err)
			}
		}
		db.RunCity2TabulaDBSetup(&config, pool)
		utils.Info.Println(flagMessages.ResetC2T.Success)
	}

	if f.ExtractFeatures {
		utils.Info.Println(flagMessages.ExtractFeatures.Progress)
		if err := process.RunFeatureExtraction(&config, pool); err != nil {
			utils.Error.Fatalf(flagMessages.ExtractFeatures.Error+": %v", err)
		}
		utils.Info.Println(flagMessages.ExtractFeatures.Success)
	}
}
