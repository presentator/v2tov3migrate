package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
)

func (m *Migrator) MigrateLinks() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("links")
	if err != nil {
		return err
	}

	limit := 1000
	items := make([]*v2ProjectLink, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("ProjectLink").
			OrderBy("id asc").
			Limit(int64(limit)).
			Offset(int64(i * limit))
		if err := q.All(&items); err != nil {
			return err
		}

		for _, item := range items {
			record := m.initRecordToMigrate(collection, item.baseModel, "link")
			if record == nil {
				continue // already migrated
			}

			record.Set("created", item.CreatedAt)
			record.Set("updated", item.UpdatedAt)
			record.Set("project", fmt.Sprintf("%s%d", v2Prefix, item.ProjectId))
			record.Set("username", item.Slug)
			record.Set("allowComments", item.AllowComments)

			record.RefreshTokenKey()
			if item.PasswordHash != nil && *item.PasswordHash != "" {
				record.Set("passwordHash", *item.PasswordHash)
				record.Set("passwordProtect", true)
			} else {
				// pb123456 (the value doesn't matter in this case)
				record.Set("passwordHash", "$2a$10$MpFMEPzs4zLHtrUKZybjL.VTDActqyMbkQNcuP4oRHt7m6f9u71Ny")
				record.Set("passwordProtect", false)
			}

			prototypeIds, err := m.getPrefixedLinkPrototypeIds(item.Id)
			if err != nil {
				return fmt.Errorf("failed to retrieve link %d prototypes: %w", item.Id, err)
			}
			record.Set("onlyPrototypes", prototypeIds)

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

func (m *Migrator) getPrefixedLinkPrototypeIds(linkId int) ([]string, error) {
	var ids []int

	err := m.oldDB.Select("prototypeId").
		From("ProjectLinkPrototypeRel").
		AndWhere(dbx.HashExp{"projectLinkId": linkId}).
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
