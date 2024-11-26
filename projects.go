package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateProjects() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("projects")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

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
			insertedIds = append(insertedIds, m.buildRecordId(item.baseModel))

			record := m.initRecordToMigrate(collection, item.baseModel)
			if record == nil {
				continue // already migrated
			}

			createdAt, _ := types.ParseDateTime(item.CreatedAt)
			record.SetRaw("created", createdAt)

			updatedAt, _ := types.ParseDateTime(item.UpdatedAt)
			record.SetRaw("updated", updatedAt)

			record.Set("archived", item.Archived)
			record.Set("title", item.Title)

			userIds, err := m.getPrefixedProjectUserIds(item.Id)
			if err != nil {
				return err
			}
			record.Set("users", userIds)

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
