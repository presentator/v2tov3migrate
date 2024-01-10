package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
)

func (m *Migrator) MigrateProjects() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}

	limit := 1000
	items := make([]*v2Project, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("Project").
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
			record.Set("archived", item.Archived)
			record.Set("title", item.Title)

			userIds, err := m.getPrefixedProjectUserIds(item.Id)
			if err != nil {
				return err
			}
			record.Set("users", userIds)

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

func (m *Migrator) getPrefixedProjectUserIds(projectId int) ([]string, error) {
	var ids []int

	err := m.oldDB.Select("userId").
		From("UserProjectRel").
		AndWhere(dbx.HashExp{"projectId": projectId}).
		Column(&ids)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(ids))

	for i, id := range ids {
		result[i] = fmt.Sprintf("%s%d", v2Prefix, id)
	}

	return result, nil
}
