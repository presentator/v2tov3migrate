package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

func (m *Migrator) MigrateUsers() error {
	collection, err := m.pbApp.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	hasOldRecords := m.isNotEmptyCollection(collection)
	insertedIds := make([]string, 0, 1000)

	limit := 1000
	items := make([]*v2User, 0, limit)
	for i := 0; ; i++ {
		q := m.oldDB.Select("*").
			From("User").
			OrderBy("id asc").
			Limit(int64(limit)).
			Offset(int64(i * limit))
		if err := q.All(&items); err != nil {
			return err
		}

		filesToCopy := make(map[string]string, len(items))

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

			record.SetVerified(item.Status == "active")
			record.SetEmail(item.Email)
			record.SetEmailVisibility(false)
			record.SetRaw("password", item.PasswordHash)
			record.RefreshTokenKey()

			// generate username
			localPart, _, _ := strings.Cut(item.Email, "@")
			username := suggestUniqueAuthRecordUsername(m.pbApp, "users", localPart)
			record.Set("username", username)

			record.Set("name", strings.TrimSpace(cast.ToString(item.FirstName)+" "+cast.ToString(item.LastName)))

			// enable by default as the existing user options are not exactly similar to the new one
			record.Set("allowEmailNotifications", true)

			oldAvatarKey := cast.ToString(*item.AvatarFilePath)
			if oldAvatarKey != "" {
				record.Set("avatar", path.Base(oldAvatarKey))

				// copy later on batches
				filesToCopy[oldAvatarKey] = record.BaseFilesPath() + "/" + record.GetString("avatar")
			}

			if err := m.pbApp.SaveNoValidate(record); err != nil {
				// try to copy batched files so that we can continue from where we left
				if copyErr := m.batchCopyFiles(filesToCopy, 500, "user_avatars"); copyErr != nil {
					return fmt.Errorf("failed to save %q and to copy all user avatars: %w; %w", record.Id, err, copyErr)
				}

				return fmt.Errorf("failed to save %q: %w", record.Id, err)
			}
		}

		if err := m.batchCopyFiles(filesToCopy, 500, "user_avatars"); err != nil {
			return err
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

func suggestUniqueAuthRecordUsername(
	app core.App,
	collectionNameOrId string,
	baseUsername string,
) string {
	username := baseUsername

	for i := 0; i < 100; i++ { // max 100 attempts
		total, err := app.CountRecords(
			collectionNameOrId,
			dbx.NewExp("LOWER([[username]])={:username}", dbx.Params{"username": strings.ToLower(username)}),
		)
		if total == 0 && err == nil {
			break // already unique
		}
		username = baseUsername + security.RandomStringWithAlphabet(3+i, "123456789")
	}

	return username
}
