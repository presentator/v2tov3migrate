package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
)

func (m *Migrator) MigrateHotspotTemplates() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("hotspotTemplates")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2HotspotTemplate, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("HotspotTemplate").
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

			record.Set("created", item.CreatedAt)
			record.Set("updated", item.UpdatedAt)
			record.Set("prototype", fmt.Sprintf("%s%d", v2Prefix, item.PrototypeId))

			screenIds, err := m.getPrefixedTemplateScreenIds(item.Id)
			if err != nil {
				return fmt.Errorf("failed to fetch template screens: %w", err)
			}
			record.Set("screens", screenIds)

			record.Set("title", item.Title)
			// try to append a counter if there is already an existing
			// template with the same title in the prototype
			// (in the old version we allowed duplicates)
			for i := 2; i <= 10; i++ {
				_, err := m.pbDao.FindFirstRecordByFilter(collection.Id, "title={:title} && prototype={:prototype}", dbx.Params{
					"title":     record.GetString("title"),
					"prototype": record.GetString("prototype"),
				})
				if err == nil {
					record.Set("title", fmt.Sprintf("%s %d", item.Title, i))
				} else {
					break
				}
			}

			if err := m.pbDao.SaveRecord(record); err != nil {
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

func (m *Migrator) getPrefixedTemplateScreenIds(templateId int) ([]string, error) {
	var ids []int

	err := m.oldDB.Select("screenId").
		From("HotspotTemplateScreenRel").
		AndWhere(dbx.HashExp{"hotspotTemplateId": templateId}).
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
