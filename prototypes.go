package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

func (m *Migrator) MigratePrototypes() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("prototypes")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2Prototype, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("Prototype").
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

			record.Set("project", fmt.Sprintf("%s%d", v2Prefix, item.ProjectId))
			record.Set("scale", item.ScaleFactor)
			if item.Type != "desktop" {
				record.Set("size", fmt.Sprintf("%dx%d", cast.ToInt(item.Width), cast.ToInt(item.Height)))
			}

			screensOrder, err := m.getPrefixedScreensOrder(item.Id)
			if err != nil {
				return err
			}
			record.Set("screensOrder", screensOrder)

			record.Set("title", item.Title)
			// try to append a counter if there is already an existing
			// prototype with the same title in the project
			// (in the old version we allowed duplicates)
			for i := 2; i <= 10; i++ {
				_, err := m.pbApp.FindFirstRecordByFilter(collection.Id, "title={:title} && project={:project}", dbx.Params{
					"title":   record.GetString("title"),
					"project": record.GetString("project"),
				})
				if err == nil {
					record.Set("title", fmt.Sprintf("%s %d", item.Title, i))
				} else {
					break
				}
			}

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

func (m *Migrator) getPrefixedScreensOrder(prototypeId int) ([]string, error) {
	var screenIds []int

	err := m.oldDB.Select("id").
		From("Screen").
		AndWhere(dbx.HashExp{"prototypeId": prototypeId}).
		OrderBy("[[order]] ASC").
		Column(&screenIds)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(screenIds))

	for i, id := range screenIds {
		result[i] = fmt.Sprintf("%s%d", v2Prefix, id)
	}

	return result, nil
}
