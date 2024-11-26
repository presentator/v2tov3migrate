package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateScreenComments() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("comments")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2ScreenComment, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("ScreenComment").
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

			record.Set("message", item.Message)
			record.Set("left", item.Left)
			record.Set("top", item.Top)
			record.Set("resolved", item.Status == "resolved")
			record.Set("screen", fmt.Sprintf("%s%d", v2Prefix, item.ScreenId))

			if item.ReplyTo != nil {
				record.Set("replyTo", fmt.Sprintf("%s%d", v2Prefix, *item.ReplyTo))
			}

			// determine if the item.From is a project user or a guest
			// ---
			projectUsers, err := m.getProjectUsersByScreenId(item.ScreenId)
			if err != nil {
				return fmt.Errorf("failed to retrieve info for the comment %q author: %q", item.Id, err)
			}

			var matchingUserId int
			for _, u := range projectUsers {
				if u.Email == item.From {
					matchingUserId = u.Id
					break
				}
			}

			if matchingUserId != 0 {
				record.Set("user", fmt.Sprintf("%s%d", v2Prefix, matchingUserId))
			} else {
				record.Set("guestEmail", item.From)
			}
			// ---

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

func (m *Migrator) getProjectUsersByScreenId(screenId int) ([]*v2User, error) {
	var result []*v2User

	err := m.oldDB.NewQuery(`
		SELECT DISTINCT u.*
		FROM {{User}} u
		INNER JOIN {{Screen}} s ON s.id = {:screenId}
		INNER JOIN {{Prototype}} p ON p.id = s.prototypeId
		INNER JOIN {{UserProjectRel}} rel ON rel.userId = u.id AND rel.projectId = p.projectId
	`).Bind(dbx.Params{
		"screenId": screenId,
	}).
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
