package mirror

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	pushmirror_service "code.gitea.io/gitea/services/pushmirror"
)

type mirrorNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &mirrorNotifier{}

// NewNotifier create a new mirrorNotifier notifier
func NewNotifier() base.Notifier {
	return &mirrorNotifier{}
}

func (m *mirrorNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("mirrorNotifier.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	pushMirrors, err := repo_model.GetPushMirrorsByRepoID(repo.ID)
	if err != nil {
		log.Error("repo_model.GetPushMirrorsByRepoID failed: %v", err)
		return
	}

	for _, mirror := range pushMirrors {
		if mirror.SyncOnPush {
			// TODO: push mirror likely will benefit from using a queue
			pushmirror_service.SyncPushMirror(ctx, mirror.ID)
		}
	}
	return
}
