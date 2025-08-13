package group

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// UploadAvatar saves custom icon for group.
func UploadAvatar(ctx context.Context, g *group_model.Group, data []byte) error {
	avatarData, err := avatar.ProcessAvatarImage(data)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	g.Avatar = avatar.HashAvatar(g.ID, data)
	if err = UpdateGroup(ctx, g, &UpdateOptions{}); err != nil {
		return fmt.Errorf("updateGroup: %w", err)
	}

	if err = storage.SaveFrom(storage.Avatars, g.CustomAvatarRelativePath(), func(w io.Writer) error {
		_, err = w.Write(avatarData)
		return err
	}); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", g.CustomAvatarRelativePath(), err)
	}

	return committer.Commit()
}

// DeleteAvatar deletes the user's custom avatar.
func DeleteAvatar(ctx context.Context, g *group_model.Group) error {
	aPath := g.CustomAvatarRelativePath()
	log.Trace("DeleteAvatar[%d]: %s", g.ID, aPath)

	return db.WithTx(ctx, func(ctx context.Context) error {
		hasAvatar := len(g.Avatar) > 0
		g.Avatar = ""
		if _, err := db.GetEngine(ctx).ID(g.ID).Cols("avatar, use_custom_avatar").Update(g); err != nil {
			return fmt.Errorf("DeleteAvatar: %w", err)
		}

		if hasAvatar {
			if err := storage.Avatars.Delete(aPath); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to remove %s: %w", aPath, err)
				}
				log.Warn("Deleting avatar %s but it doesn't exist", aPath)
			}
		}

		return nil
	})
}
