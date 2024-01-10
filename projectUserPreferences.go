package main

import (
	"fmt"
)

func (m *Migrator) MigrateProjectUserPreferences() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("projectUserPreferences")
	if err != nil {
		return err
	}

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
			record := m.initRecordToMigrate(collection, item.baseModel)
			if record == nil {
				continue // already migrated
			}

			record.Set("created", item.CreatedAt)
			record.Set("updated", item.UpdatedAt)
			record.Set("user", fmt.Sprintf("%s%d", v2Prefix, item.UserId))
			record.Set("project", fmt.Sprintf("%s%d", v2Prefix, item.ProjectId))
			record.Set("watch", true)
			record.Set("favorite", item.Pinned)

			if err := m.pbDao.SaveRecord(record); err != nil {
				return fmt.Errorf("failed to save %q: %w", record.Id, err)
			}
		}

		if len(items) < limit {
			break // no more items
		}

		items = items[:0]
	}

	return nil
}
