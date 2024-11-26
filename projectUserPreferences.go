package main

import (
	"fmt"

	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateProjectUserPreferences() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("projectUserPreferences")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2UserProjectRel, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("UserProjectRel").
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
			record.Set("project", fmt.Sprintf("%s%d", v2Prefix, item.ProjectId))
			record.Set("watch", true)
			record.Set("favorite", item.Pinned)

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
