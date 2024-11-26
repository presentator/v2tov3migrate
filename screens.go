package main

import (
	"fmt"
	"path"

	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateScreens() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("screens")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2Screen, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("Screen").
			OrderBy("id asc").
			Limit(int64(limit)).
			Offset(int64(i * limit))
		if err := q.All(&items); err != nil {
			return err
		}

		filesToCopy := make(map[string]string, len(items))

		for _, item := range items {
			insertedIds = append(insertedIds, m.buildRecordId(item.baseModel))

			record := m.initRecordToMigrate(collection, item.baseModel)
			if record == nil {
				continue // already migrated
			}

			createdAt, _ := types.ParseDateTime(item.CreatedAt)
			record.SetRaw("created", createdAt)

			updatedAt, _ := types.ParseDateTime(item.UpdatedAt)
			record.SetRaw("updated", updatedAt)

			record.Set("prototype", fmt.Sprintf("%s%d", v2Prefix, item.PrototypeId))
			record.Set("title", item.Title)
			record.Set("alignment", item.Alignment)
			record.Set("background", item.Background)
			record.Set("fixedHeader", item.FixedHeader)
			record.Set("fixedFooter", item.FixedFooter)
			record.Set("file", path.Base(item.FilePath))

			if err := m.pbApp.SaveNoValidate(record); err != nil {
				// try to copy batched files so that we can continue from where we left
				if copyErr := m.batchCopyFiles(filesToCopy, 500, "screen_file"); copyErr != nil {
					return fmt.Errorf("failed to save %q and to copy all screen files: %w; %w", record.Id, err, copyErr)
				}

				return fmt.Errorf("failed to save %q: %w", record.Id, err)
			}

			// copy later on batches
			filesToCopy[item.FilePath] = record.BaseFilesPath() + "/" + record.GetString("file")
		}

		if err := m.batchCopyFiles(filesToCopy, 500, "screen_file"); err != nil {
			return err
		}

		if len(items) < limit {
			break // no more items
		}

		items = items[:0]
	}

	if hasOldRecords {
		return m.deleteMissingRecords(collection, insertedIds)
	}

	return nil
}
