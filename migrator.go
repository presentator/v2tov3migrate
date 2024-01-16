package main

import (
	"bytes"
	"database/sql"
	"errors"
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

	color.Yellow("Migrating OAuth2 rels...")
	if err := m.MigrateUsersOAuth2(); err != nil {
		return fmt.Errorf("failed to migrate OAuth2 rels: %w", err)
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

// buildRecordId constructs a Record id from the provided base model and optional prefixes.
func (m *Migrator) buildRecordId(item baseModel, optIdPrefixes ...string) string {
	itemId := v2Prefix
	for _, p := range optIdPrefixes {
		itemId += p
	}
	itemId += fmt.Sprintf("%d", item.Id)

	return itemId
}

// initRecordToMigrate initializes a models.Record for migration (either new or for update).
//
// Returns nil if the record is already migrated and doesn't need resave.
func (m *Migrator) initRecordToMigrate(collection *models.Collection, item baseModel, optIdPrefixes ...string) *models.Record {
	id := m.buildRecordId(item, optIdPrefixes...)

	record, _ := m.pbDao.FindRecordById(collection.Id, id)
	if record != nil {
		// already migrated -> check its updated date for changes
		updated, _ := types.ParseDateTime(item.UpdatedAt)
		if updated.Time().Unix() == record.GetDateTime("updated").Time().Unix() {
			return nil
		}
	} else {
		record = models.NewRecord(collection)
		record.MarkAsNew()
		record.SetId(id)
	}

	return record
}

// dropTempIdsTable drops the temp_ids table used to store the currently
// inserted migration script records id (see also [createTempIdsTable()]).
func (m *Migrator) dropTempIdsTable() error {
	_, err := m.pbDao.DB().NewQuery("DROP TABLE If EXISTS temp_ids").Execute()
	return err
}

// createTempIdsTable creates a temp_ids table and populate it with the provided ids.
//
// If the temp table already exists it will be dropped before the creation.
func (m *Migrator) createTempIdsTable(insertedIds []string) error {
	if err := m.dropTempIdsTable(); err != nil {
		return fmt.Errorf("failed to drop temp_ids table: %w", err)
	}

	_, createErr := m.pbDao.DB().NewQuery("CREATE TEMP TABLE temp_ids (id TEXT PRIMARY KEY NOT NULL)").Execute()
	if createErr != nil {
		return fmt.Errorf("failed to create temp_ids table: %w", createErr)
	}

	for _, id := range insertedIds {
		_, err := m.pbDao.DB().Insert("temp_ids", dbx.Params{"id": id}).Execute()
		if err != nil {
			return fmt.Errorf("failed to insert temp id %q: %w", id, err)
		}
	}

	return nil
}

// isNotEmptyCollection checks if the provided collection has at least 1 record.
func (m *Migrator) isNotEmptyCollection(collection *models.Collection) bool {
	var exists bool

	err := m.pbDao.RecordQuery(collection).
		Select("(1)").
		Limit(1).
		Row(&exists)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		color.Yellow("--WARN[%s]: failed to check whether the collection has any records - %s", collection.Name, err)
	}

	return err == nil && exists
}

// deleteMissingRecords deletes all records from the provided collection
// that doesn't exist in the insertedIds slice.
//
// This method is no-op if the insertedIds slice is empty.
//
// Note that in case of an individual delete Record error,
// the error is considered non-critical and will be just logged to the stderr.
func (m *Migrator) deleteMissingRecords(collection *models.Collection, insertedIds []string) error {
	if len(insertedIds) == 0 {
		return nil // nothing previously inserted to compare with
	}

	if err := m.createTempIdsTable(insertedIds); err != nil {
		return err
	}
	defer m.dropTempIdsTable()

	var records []*models.Record

	err := m.pbDao.RecordQuery(collection).
		AndWhere(dbx.NewExp("id NOT IN (SELECT temp_ids.id FROM temp_ids)")).
		All(&records)
	if err != nil {
		return fmt.Errorf("failed to fetch records to remove: %w", err)
	}

	for _, r := range records {
		if err := m.pbDao.DeleteRecord(r); err != nil {
			// ignore the error and log only for debug
			color.Yellow("--WARN[%s]: Failed to delete previously inserted record %q (raw error: %s)", collection.Name, r.Id, err)
		}
	}

	return nil
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
