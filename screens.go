package main

import (
	"fmt"
	"path"
)

func (m *Migrator) MigrateScreens() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("screens")
	if err != nil {
		return err
	}

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
			record := m.initRecordToMigrate(collection, item.baseModel)
			if record == nil {
				continue // already migrated
			}

			record.Set("created", item.CreatedAt)
			record.Set("updated", item.UpdatedAt)
			record.Set("prototype", fmt.Sprintf("%s%d", v2Prefix, item.PrototypeId))
			record.Set("title", item.Title)
			record.Set("alignment", item.Alignment)
			record.Set("background", item.Background)
			record.Set("fixedHeader", item.FixedHeader)
			record.Set("fixedFooter", item.FixedFooter)
			record.Set("file", path.Base(item.FilePath))

			if err := m.pbDao.SaveRecord(record); err != nil {
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

	return nil
}
