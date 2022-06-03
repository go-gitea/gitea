package convert

import (
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPushMirror converts a PushMirror to api.PushMirror
func ToPushMirror(p *repo_model.PushMirror, mode perm.AccessMode) *api.PushMirror {
	return &api.PushMirror{
		ID:             p.ID,
		RepoID:         p.RepoID,
		Repo:           ToRepo(p.Repo, mode),
		RemoteName:     p.RemoteName,
		Interval:       p.Interval,
		CreatedUnix:    int64(p.CreatedUnix),
		LastUpdateUnix: int64(p.LastUpdateUnix),
		LastError:      p.LastError,
	}
}
