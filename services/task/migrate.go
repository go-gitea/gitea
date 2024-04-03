// Copyright 2019 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/migrations"
	notify_service "code.gitea.io/gitea/services/notify"
)

func handleCreateError(owner *user_model.User, err error) error {
	switch {
	case repo_model.IsErrReachLimitOfRepo(err):
		return fmt.Errorf("you have already reached your limit of %d repositories", owner.MaxCreationLimit())
	case repo_model.IsErrRepoAlreadyExist(err):
		return errors.New("the repository name is already used")
	case db.IsErrNameReserved(err):
		return fmt.Errorf("the repository name '%s' is reserved", err.(db.ErrNameReserved).Name)
	case db.IsErrNamePatternNotAllowed(err):
		return fmt.Errorf("the pattern '%s' is not allowed in a repository name", err.(db.ErrNamePatternNotAllowed).Pattern)
	default:
		return err
	}
}

func runMigrateTask(ctx context.Context, t *admin_model.Task) (err error) {
	defer func(ctx context.Context) {
		if e := recover(); e != nil {
			err = fmt.Errorf("PANIC whilst trying to do migrate task: %v", e)
			log.Critical("PANIC during runMigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d]: %v\nStacktrace: %v", t.ID, t.DoerID, t.RepoID, t.OwnerID, e, log.Stack(2))
		}
		if err == nil {
			err = admin_model.FinishMigrateTask(ctx, t)
			if err == nil {
				notify_service.MigrateRepository(ctx, t.Doer, t.Owner, t.Repo)
				return
			}

			log.Error("FinishMigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d] failed: %v", t.ID, t.DoerID, t.RepoID, t.OwnerID, err)
		}

		log.Error("runMigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d] failed: %v", t.ID, t.DoerID, t.RepoID, t.OwnerID, err)

		t.EndTime = timeutil.TimeStampNow()
		t.Status = structs.TaskStatusFailed
		t.Message = err.Error()
		if err := t.UpdateCols(ctx, "status", "message", "end_time"); err != nil {
			log.Error("Task UpdateCols failed: %v", err)
		}

		// then, do not delete the repository, otherwise the users won't be able to see the last error
	}(graceful.GetManager().ShutdownContext()) // even if the parent ctx is canceled, this defer-function still needs to update the task record in database

	if err = t.LoadRepo(ctx); err != nil {
		return err
	}

	// if repository is ready, then just finish the task
	if t.Repo.Status == repo_model.RepositoryReady {
		return nil
	}

	if err = t.LoadDoer(ctx); err != nil {
		return err
	}
	if err = t.LoadOwner(ctx); err != nil {
		return err
	}

	var opts *migration.MigrateOptions
	opts, err = t.MigrateConfig()
	if err != nil {
		return err
	}

	opts.MigrateToRepoID = t.RepoID

	pm := process.GetManager()
	ctx, cancel, finished := pm.AddContext(graceful.GetManager().ShutdownContext(), fmt.Sprintf("MigrateTask: %s/%s", t.Owner.Name, opts.RepoName))
	defer finished()

	t.StartTime = timeutil.TimeStampNow()
	t.Status = structs.TaskStatusRunning
	if err = t.UpdateCols(ctx, "start_time", "status"); err != nil {
		return err
	}

	// check whether the task should be canceled, this goroutine is also managed by process manager
	go func() {
		for {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
			task, _ := admin_model.GetMigratingTask(ctx, t.RepoID)
			if task != nil && task.Status != structs.TaskStatusRunning {
				log.Debug("MigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d] is canceled due to status is not 'running'", t.ID, t.DoerID, t.RepoID, t.OwnerID)
				cancel()
				return
			}
		}
	}()

	t.Repo, err = migrations.MigrateRepository(ctx, t.Doer, t.Owner.Name, *opts, func(format string, args ...any) {
		message := admin_model.TranslatableMessage{
			Format: format,
			Args:   args,
		}
		bs, _ := json.Marshal(message)
		t.Message = string(bs)
		_ = t.UpdateCols(ctx, "message")
	})

	if err == nil {
		log.Trace("Repository migrated [%d]: %s/%s", t.Repo.ID, t.Owner.Name, t.Repo.Name)
		return nil
	}

	if repo_model.IsErrRepoAlreadyExist(err) {
		return errors.New("the repository name is already used")
	}

	// remoteAddr may contain credentials, so we sanitize it
	err = util.SanitizeErrorCredentialURLs(err)
	if strings.Contains(err.Error(), "Authentication failed") ||
		strings.Contains(err.Error(), "could not read Username") {
		return fmt.Errorf("authentication failed: %w", err)
	} else if strings.Contains(err.Error(), "fatal:") {
		return fmt.Errorf("migration failed: %w", err)
	}

	// do not be tempted to coalesce this line with the return
	err = handleCreateError(t.Owner, err)
	return err
}
