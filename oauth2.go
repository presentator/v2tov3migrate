package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/list"
	"github.com/pocketbase/pocketbase/tools/types"
)

func (m *Migrator) MigrateUsersOAuth2() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("users")
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

			var ea *core.ExternalAuth
			if ea, _ = m.pbApp.FindFirstExternalAuthByExpr(dbx.HashExp{"id": itemId}); ea != nil {
				// already migrated -> check its updated date for changes
				updated, _ := types.ParseDateTime(item.UpdatedAt)
				if updated.Time().Unix() == ea.GetDateTime("updated").Time().Unix() {
					continue
				}
			} else {
				ea = core.NewExternalAuth(m.pbApp)
				ea.MarkAsNew()
				ea.Id = itemId
			}

			createdAt, _ := types.ParseDateTime(item.CreatedAt)
			ea.SetRaw("created", createdAt)

			updatedAt, _ := types.ParseDateTime(item.UpdatedAt)
			ea.SetRaw("updated", updatedAt)

			ea.SetCollectionRef(collection.Id)
			ea.SetRecordRef(fmt.Sprintf("%s%d", v2Prefix, item.UserId))
			ea.SetProvider(item.Source)
			ea.SetProviderId(item.SourceId)

			if err := m.pbApp.SaveNoValidate(ea); err != nil {
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
