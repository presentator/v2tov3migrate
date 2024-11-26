package main

import (
	"fmt"

	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateNotifications() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("notifications")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2UserScreenCommentRel, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("UserScreenCommentRel").
			OrderBy("id asc").
			Limit(int64(limit)).
			Offset(int64(i * limit))
		if err := q.All(&items); err != nil {
			return err
		}

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

			record.Set("user", fmt.Sprintf("%s%d", v2Prefix, item.UserId))
			record.Set("comment", fmt.Sprintf("%s%d", v2Prefix, item.ScreenCommentId))
			record.Set("read", item.IsRead)
			record.Set("processed", item.IsProcessed)

			if err := m.pbApp.SaveNoValidate(record); err != nil {
				return fmt.Errorf("failed to save %q: %w", record.Id, err)
			}
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
