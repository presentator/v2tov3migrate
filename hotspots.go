package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"
)

func (m *Migrator) MigrateHotspots() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("hotspots")
	if err != nil {
		return err
	}

	limit := 1000
	items := make([]*v2Hotspot, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("Hotspot").
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
			record.Set("left", item.Left)
			record.Set("top", item.Top)
			record.Set("width", item.Width)
			record.Set("height", item.Height)
			record.Set("type", item.Type)

			if item.ScreenId != nil {
				record.Set("screen", fmt.Sprintf("%s%d", v2Prefix, *item.ScreenId))
			}

			if item.HotspotTemplateId != nil {
				record.Set("hotspotTemplate", fmt.Sprintf("%s%d", v2Prefix, *item.HotspotTemplateId))
			}

			if item.Settings != nil && *item.Settings != "" {
				settings := map[string]any{}
				if err := json.Unmarshal([]byte(*item.Settings), &settings); err != nil {
					return fmt.Errorf("failed to read hotspot %q settings: %w", item.Id, err)
				}

				if cast.ToString(settings["transition"]) == "none" {
					settings["transition"] = ""
				}

				if screenId := cast.ToString(settings["screenId"]); screenId != "" {
					delete(settings, "screenId")
					settings["screen"] = fmt.Sprintf("%s%s", v2Prefix, screenId)
				}

				record.Set("settings", settings)
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

	return nil
}
