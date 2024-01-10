package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
	"golang.org/x/sync/errgroup"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const v2Prefix string = "pr2_"

// NewMigrator creates and setup a new Migrator instance.
//
// NB! Don't forget to call [Migrator.Close()] after you are done working with the Migrator.
//
// Example
//
//	m, err := NewMigrator(app, config)
//	if err != nil {
//		return err
//	}
//	defer m.Close()
//
//	if err := m.MigrateAll(); err != nil {
//		return err
//	}
func NewMigrator(app core.App, config *Config) (*Migrator, error) {
	m := &Migrator{
		pbDao: app.Dao(),
	}

	var errOldDB error
	m.oldDB, errOldDB = dbx.MustOpen(config.V2DBDriver, config.V2DBConnection)
	if errOldDB != nil {
		m.Close()
		return nil, errOldDB
	}

	var errOldFS error
	if config.V2S3Storage.Bucket != "" {
		m.oldFS, errOldFS = filesystem.NewS3(
			config.V2S3Storage.Bucket,
			config.V2S3Storage.Region,
			config.V2S3Storage.Endpoint,
			config.V2S3Storage.AccessKey,
			config.V2S3Storage.Secret,
			config.V2S3Storage.ForcePathStyle,
		)
	} else {
		m.oldFS, errOldFS = filesystem.NewLocal(config.V2LocalStorage)
	}
	if errOldFS != nil {
		m.Close()
		return nil, errOldFS
	}

	var errNewFS error
	m.newFS, errNewFS = app.NewFilesystem()
	if errNewFS != nil {
		m.Close()
		return nil, errNewFS
	}

	return m, nil
}

type Migrator struct {
	oldDB *dbx.DB
	pbDao *daos.Dao
	oldFS *filesystem.System
	newFS *filesystem.System
}

// Close takes care to cleanup migrator related resources.
func (m *Migrator) Close() {
	if m.oldDB != nil {
		m.oldDB.Close()
	}

	if m.oldFS != nil {
		m.oldFS.Close()
	}

	if m.newFS != nil {
		m.newFS.Close()
	}
}

// MigrateAll executes all available model migrations.
func (m *Migrator) MigrateAll() error {
	start := time.Now()

	color.Green("Presentator v2 to v3 migration started...")

	color.Yellow("Migrating users...")
	if err := m.MigrateUsers(); err != nil {
		return fmt.Errorf("failed to migrate users: %w", err)
	}

	color.Yellow("Migrating projects...")
	if err := m.MigrateProjects(); err != nil {
		return fmt.Errorf("failed to migrate projects: %w", err)
	}

	color.Yellow("Migrating project user preferences...")
	if err := m.MigrateProjectUserPreferences(); err != nil {
		return fmt.Errorf("failed to migrate project user preferences: %w", err)
	}

	color.Yellow("Migrating prototypes...")
	if err := m.MigratePrototypes(); err != nil {
		return fmt.Errorf("failed to migrate prototypes: %w", err)
	}

	color.Yellow("Migrating screens...")
	if err := m.MigrateScreens(); err != nil {
		return fmt.Errorf("failed to migrate screens: %w", err)
	}

	color.Yellow("Migrating screen comments...")
	if err := m.MigrateScreenComments(); err != nil {
		return fmt.Errorf("failed to migrate screen comments: %w", err)
	}

	color.Yellow("Migrating hotspot templates...")
	if err := m.MigrateHotspotTemplates(); err != nil {
		return fmt.Errorf("failed to migrate hotspot templates: %w", err)
	}

	color.Yellow("Migrating hotspots...")
	if err := m.MigrateHotspots(); err != nil {
		return fmt.Errorf("failed to migrate hotspots: %w", err)
	}

	color.Yellow("Migrating project links...")
	if err := m.MigrateLinks(); err != nil {
		return fmt.Errorf("failed to migrate project links: %w", err)
	}

	color.Yellow("Migrating unread notifications...")
	if err := m.MigrateNotifications(); err != nil {
		return fmt.Errorf("failed to migrate notifications: %w", err)
	}

	color.Green("Migration completed successfully (%v).", time.Since(start))

	return nil
}

// Initializes a models.Record for migration (either new or for update).
//
// Returns nil if the record is already migrated and doesn't need resave.
func (m *Migrator) initRecordToMigrate(collection *models.Collection, item baseModel, optIdPrefixes ...string) *models.Record {
	itemId := v2Prefix
	for _, p := range optIdPrefixes {
		itemId += p
	}
	itemId += fmt.Sprintf("%d", item.Id)

	record, _ := m.pbDao.FindRecordById(collection.Id, itemId)
	if record != nil {
		// already migrated -> check its updated date for changes
		updated, _ := types.ParseDateTime(item.UpdatedAt)
		if updated.Time().Unix() == record.GetDateTime("updated").Time().Unix() {
			return nil
		}
	} else {
		record = models.NewRecord(collection)
		record.MarkAsNew()
		record.SetId(itemId)
	}

	return record
}

// batchCopyFiles copies all the specified files from the configured v2 to v3 storage location.
//
// Note: copy errors are treated as non-critical and only logged because it is possible
// that there could be some missing/removed files as a result from v1/v2 failed screen upload/delete.
func (m *Migrator) batchCopyFiles(files map[string]string, batchSize int, errLogGroup string) error {
	var copyGroup errgroup.Group

	copyGroup.SetLimit(batchSize)

	for old, new := range files {
		old := old
		new := new
		copyGroup.Go(func() error {
			if err := m.copyFile(old, new); err != nil {
				// ignore the error and log only for debug
				color.Yellow("--WARN[%s]: Failed to copy file %q to %q (raw error: %s)", errLogGroup, old, new, err)
			}
			return nil
		})
	}

	return copyGroup.Wait()
}

// copyFile copies a single file from oldKey to newKey location.
func (m *Migrator) copyFile(oldKey string, newKey string) error {
	oldFile, err := m.oldFS.GetFile(oldKey)
	if err != nil {
		return err
	}
	defer oldFile.Close()

	var buf bytes.Buffer

	if _, err := oldFile.WriteTo(&buf); err != nil {
		return err
	}

	return m.newFS.Upload(buf.Bytes(), newKey)
}
