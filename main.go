package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/pocketbase/pocketbase"
)

func main() {
	if err := run(); err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}
}

func run() error {
	// Load config from user specified config.json file
	// ---------------------------------------------------------------
	var configPath string
	flag.StringVar(&configPath, "config", "./config.json", "Path to the migration config json file")
	flag.Parse()

	config, err := NewConfigFromJson(configPath)
	if err != nil {
		return err
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("[config error] %w", err)
	}

	// Init Presentator v3 app
	// ---------------------------------------------------------------
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: config.V3DataDir,
	})
	defer app.ResetBootstrapState()

	if err := app.Bootstrap(); err != nil {
		return err
	}

	// Migrate
	// ---------------------------------------------------------------
	migrator, err := NewMigrator(app, config)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}
	defer migrator.Close()

	return migrator.MigrateAll()
}
