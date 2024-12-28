package group

import (
	"code.gitea.io/gitea/models/avatars"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"context"
	"fmt"
	"image/png"
	"io"
	"net/url"
)

func (g *Group) CustomAvatarRelativePath() string {
	return g.Avatar
}
func generateRandomAvatar(ctx context.Context, group *Group) error {
	idToString := fmt.Sprintf("%d", group.ID)

	seed := idToString
	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %w", err)
	}

	group.Avatar = idToString

	if err = storage.SaveFrom(storage.RepoAvatars, group.CustomAvatarRelativePath(), func(w io.Writer) error {
		if err = png.Encode(w, img); err != nil {
			log.Error("Encode: %v", err)
		}
		return err
	}); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", group.CustomAvatarRelativePath(), err)
	}

	log.Info("New random avatar created for repository: %d", group.ID)

	if _, err = db.GetEngine(ctx).ID(group.ID).Cols("avatar").NoAutoTime().Update(group); err != nil {
		return err
	}

	return nil
}
func (g *Group) relAvatarLink(ctx context.Context) string {
	// If no avatar - path is empty
	avatarPath := g.CustomAvatarRelativePath()
	if len(avatarPath) == 0 {
		switch mode := setting.RepoAvatar.Fallback; mode {
		case "image":
			return setting.RepoAvatar.FallbackImage
		case "random":
			if err := generateRandomAvatar(ctx, g); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	return setting.AppSubURL + "/group-avatars/" + url.PathEscape(g.Avatar)
}

func (g *Group) AvatarLink(ctx context.Context) string {
	relLink := g.relAvatarLink(ctx)
	if relLink != "" {
		return httplib.MakeAbsoluteURL(ctx, relLink)
	}
	return ""
}
func (g *Group) AvatarLinkWithSize(size int) string {
	if g.Avatar == "" {
		return avatars.DefaultAvatarLink()
	}
	return avatars.GenerateUserAvatarImageLink(g.Avatar, size)
}
