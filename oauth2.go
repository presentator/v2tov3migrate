package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/list"
	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateUsersOAuth2() error {
	collection, err := m.pbDao.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	// check for older duplicates to ignore from the import
	// (in PocketBase a single auth record can be linked to only one OAuth2 account from the same provider).
	toIgnore := []string{}
	toIgnoreErr := m.oldDB.Select("min(id)").
		From("UserAuth").
		GroupBy("userId", "source").
		Having(dbx.NewExp("count(id) > 1")).
		Column(&toIgnore)
	if toIgnoreErr != nil {
		return fmt.Errorf("failed to fetch UserAuth duplicates: %w", toIgnoreErr)
	}

	limit := 1000
	items := make([]*v2UserAuth, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("UserAuth").
			OrderBy("id asc").
			Where(dbx.NotIn("id", list.ToInterfaceSlice(toIgnore)...)).
			Limit(int64(limit)).
			Offset(int64(i * limit))
		if err := q.All(&items); err != nil {
			return err
		}

		for _, item := range items {
			itemId := fmt.Sprintf("%s%d", v2Prefix, item.Id)

			var ea *models.ExternalAuth
			if ea, _ = m.pbDao.FindFirstExternalAuthByExpr(dbx.HashExp{"id": itemId}); ea != nil {
				// already migrated -> check its updated date for changes
				updated, _ := types.ParseDateTime(item.UpdatedAt)
				if updated.Time().Unix() == ea.Updated.Time().Unix() {
					continue
				}
			} else {
				ea = &models.ExternalAuth{}
				ea.MarkAsNew()
				ea.Id = itemId
			}

			ea.Created, _ = types.ParseDateTime(item.CreatedAt)
			ea.Updated, _ = types.ParseDateTime(item.UpdatedAt)
			ea.CollectionId = collection.Id
			ea.Provider = item.Source
			ea.ProviderId = item.SourceId
			ea.RecordId = fmt.Sprintf("%s%d", v2Prefix, item.UserId)

			if err := m.pbDao.SaveExternalAuth(ea); err != nil {
				return fmt.Errorf("failed to save %q: %w", ea.Id, err)
			}
		}

		if len(items) < limit {
			break // no more items
		}

		items = items[:0]
	}

	return nil
}
