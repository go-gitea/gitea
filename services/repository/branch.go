// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/queue"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	notify_service "code.gitea.io/gitea/services/notify"
	release_service "code.gitea.io/gitea/services/release"
	files_service "code.gitea.io/gitea/services/repository/files"

	"xorm.io/builder"
)

// CreateNewBranch creates a new repository branch
func CreateNewBranch(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, oldBranchName, branchName string) (err error) {
	branch, err := git_model.GetBranch(ctx, repo.ID, oldBranchName)
	if err != nil {
		return err
	}

	return CreateNewBranchFromCommit(ctx, doer, repo, gitRepo, branch.CommitID, branchName)
}

// Branch contains the branch information
type Branch struct {
	DBBranch          *git_model.Branch
	IsProtected       bool
	IsIncluded        bool
	CommitsAhead      int
	CommitsBehind     int
	LatestPullRequest *issues_model.PullRequest
	MergeMovedOn      bool
}

// LoadBranches loads branches from the repository limited by page & pageSize.
func LoadBranches(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, isDeletedBranch optional.Option[bool], keyword string, page, pageSize int) (*Branch, []*Branch, int64, error) {
	defaultDBBranch, err := git_model.GetBranch(ctx, repo.ID, repo.DefaultBranch)
	if err != nil {
		return nil, nil, 0, err
	}

	branchOpts := git_model.FindBranchOptions{
		RepoID:          repo.ID,
		IsDeletedBranch: isDeletedBranch,
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
		Keyword:            keyword,
		ExcludeBranchNames: []string{repo.DefaultBranch},
	}

	dbBranches, totalNumOfBranches, err := db.FindAndCount[git_model.Branch](ctx, branchOpts)
	if err != nil {
		return nil, nil, 0, err
	}

	if err := git_model.BranchList(dbBranches).LoadDeletedBy(ctx); err != nil {
		return nil, nil, 0, err
	}
	if err := git_model.BranchList(dbBranches).LoadPusher(ctx); err != nil {
		return nil, nil, 0, err
	}

	rules, err := git_model.FindRepoProtectedBranchRules(ctx, repo.ID)
	if err != nil {
		return nil, nil, 0, err
	}

	repoIDToRepo := map[int64]*repo_model.Repository{}
	repoIDToRepo[repo.ID] = repo

	repoIDToGitRepo := map[int64]*git.Repository{}
	repoIDToGitRepo[repo.ID] = gitRepo

	branches := make([]*Branch, 0, len(dbBranches))
	for i := range dbBranches {
		branch, err := loadOneBranch(ctx, repo, dbBranches[i], &rules, repoIDToRepo, repoIDToGitRepo)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("loadOneBranch: %v", err)
		}
		branches = append(branches, branch)
	}

	// Always add the default branch
	log.Debug("loadOneBranch: load default: '%s'", defaultDBBranch.Name)
	defaultBranch, err := loadOneBranch(ctx, repo, defaultDBBranch, &rules, repoIDToRepo, repoIDToGitRepo)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("loadOneBranch: %v", err)
	}
	return defaultBranch, branches, totalNumOfBranches, nil
}

func getDivergenceCacheKey(repoID int64, branchName string) string {
	return fmt.Sprintf("%d-%s", repoID, branchName)
}

// getDivergenceFromCache gets the divergence from cache
func getDivergenceFromCache(repoID int64, branchName string) (*git.DivergeObject, bool) {
	data, ok := cache.GetCache().Get(getDivergenceCacheKey(repoID, branchName))
	res := git.DivergeObject{
		Ahead:  -1,
		Behind: -1,
	}
	if !ok || data == "" {
		return &res, false
	}
	if err := json.Unmarshal(util.UnsafeStringToBytes(data), &res); err != nil {
		log.Error("json.UnMarshal failed: %v", err)
		return &res, false
	}
	return &res, true
}

func putDivergenceFromCache(repoID int64, branchName string, divergence *git.DivergeObject) error {
	bs, err := json.Marshal(divergence)
	if err != nil {
		return err
	}
	return cache.GetCache().Put(getDivergenceCacheKey(repoID, branchName), util.UnsafeBytesToString(bs), 30*24*60*60)
}

func DelDivergenceFromCache(repoID int64, branchName string) error {
	return cache.GetCache().Delete(getDivergenceCacheKey(repoID, branchName))
}

// DelRepoDivergenceFromCache deletes all divergence caches of a repository
func DelRepoDivergenceFromCache(ctx context.Context, repoID int64) error {
	dbBranches, err := db.Find[git_model.Branch](ctx, git_model.FindBranchOptions{
		RepoID:      repoID,
		ListOptions: db.ListOptionsAll,
	})
	if err != nil {
		return err
	}
	for i := range dbBranches {
		if err := DelDivergenceFromCache(repoID, dbBranches[i].Name); err != nil {
			log.Error("DelDivergenceFromCache: %v", err)
		}
	}
	return nil
}

func loadOneBranch(ctx context.Context, repo *repo_model.Repository, dbBranch *git_model.Branch, protectedBranches *git_model.ProtectedBranchRules,
	repoIDToRepo map[int64]*repo_model.Repository,
	repoIDToGitRepo map[int64]*git.Repository,
) (*Branch, error) {
	log.Trace("loadOneBranch: '%s'", dbBranch.Name)

	branchName := dbBranch.Name
	p := protectedBranches.GetFirstMatched(branchName)
	isProtected := p != nil

	var divergence *git.DivergeObject

	// it's not default branch
	if repo.DefaultBranch != dbBranch.Name && !dbBranch.IsDeleted {
		var cached bool
		divergence, cached = getDivergenceFromCache(repo.ID, dbBranch.Name)
		if !cached {
			var err error
			divergence, err = files_service.CountDivergingCommits(ctx, repo, git.BranchPrefix+branchName)
			if err != nil {
				log.Error("CountDivergingCommits: %v", err)
			} else {
				if err = putDivergenceFromCache(repo.ID, dbBranch.Name, divergence); err != nil {
					log.Error("putDivergenceFromCache: %v", err)
				}
			}
		}
	}

	if divergence == nil {
		// tolerate the error that we cannot get divergence
		divergence = &git.DivergeObject{Ahead: -1, Behind: -1}
	}

	pr, err := issues_model.GetLatestPullRequestByHeadInfo(ctx, repo.ID, branchName)
	if err != nil {
		return nil, fmt.Errorf("GetLatestPullRequestByHeadInfo: %v", err)
	}
	headCommit := dbBranch.CommitID

	mergeMovedOn := false
	if pr != nil {
		pr.HeadRepo = repo
		if err := pr.LoadIssue(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssue: %v", err)
		}
		if repo, ok := repoIDToRepo[pr.BaseRepoID]; ok {
			pr.BaseRepo = repo
		} else if err := pr.LoadBaseRepo(ctx); err != nil {
			return nil, fmt.Errorf("LoadBaseRepo: %v", err)
		} else {
			repoIDToRepo[pr.BaseRepoID] = pr.BaseRepo
		}
		pr.Issue.Repo = pr.BaseRepo

		if pr.HasMerged {
			baseGitRepo, ok := repoIDToGitRepo[pr.BaseRepoID]
			if !ok {
				baseGitRepo, err = gitrepo.OpenRepository(ctx, pr.BaseRepo)
				if err != nil {
					return nil, fmt.Errorf("OpenRepository: %v", err)
				}
				defer baseGitRepo.Close()
				repoIDToGitRepo[pr.BaseRepoID] = baseGitRepo
			}
			pullCommit, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil && !git.IsErrNotExist(err) {
				return nil, fmt.Errorf("GetBranchCommitID: %v", err)
			}
			if err == nil && headCommit != pullCommit {
				// the head has moved on from the merge - we shouldn't delete
				mergeMovedOn = true
			}
		}
	}

	isIncluded := divergence.Ahead == 0 && repo.DefaultBranch != branchName
	return &Branch{
		DBBranch:          dbBranch,
		IsProtected:       isProtected,
		IsIncluded:        isIncluded,
		CommitsAhead:      divergence.Ahead,
		CommitsBehind:     divergence.Behind,
		LatestPullRequest: pr,
		MergeMovedOn:      mergeMovedOn,
	}, nil
}

// checkBranchName validates branch name with existing repository branches
func checkBranchName(ctx context.Context, repo *repo_model.Repository, name string) error {
	_, err := gitrepo.WalkReferences(ctx, repo, func(_, refName string) error {
		branchRefName := strings.TrimPrefix(refName, git.BranchPrefix)
		switch {
		case branchRefName == name:
			return git_model.ErrBranchAlreadyExists{
				BranchName: name,
			}
		// If branchRefName like a/b but we want to create a branch named a then we have a conflict
		case strings.HasPrefix(branchRefName, name+"/"):
			return git_model.ErrBranchNameConflict{
				BranchName: branchRefName,
			}
			// Conversely if branchRefName like a but we want to create a branch named a/b then we also have a conflict
		case strings.HasPrefix(name, branchRefName+"/"):
			return git_model.ErrBranchNameConflict{
				BranchName: branchRefName,
			}
		case refName == git.TagPrefix+name:
			return release_service.ErrTagAlreadyExists{
				TagName: name,
			}
		}
		return nil
	})

	return err
}

// SyncBranchesToDB sync the branch information in the database.
// It will check whether the branches of the repository have never been synced before.
// If so, it will sync all branches of the repository.
// Otherwise, it will sync the branches that need to be updated.
func SyncBranchesToDB(ctx context.Context, repoID, pusherID int64, branchNames, commitIDs []string, getCommit func(commitID string) (*git.Commit, error)) error {
	// Some designs that make the code look strange but are made for performance optimization purposes:
	// 1. Sync branches in a batch to reduce the number of DB queries.
	// 2. Lazy load commit information since it may be not necessary.
	// 3. Exit early if synced all branches of git repo when there's no branch in DB.
	// 4. Check the branches in DB if they are already synced.
	//
	// If the user pushes many branches at once, the Git hook will call the internal API in batches, rather than all at once.
	// See https://github.com/go-gitea/gitea/blob/cb52b17f92e2d2293f7c003649743464492bca48/cmd/hook.go#L27
	// For the first batch, it will hit optimization 3.
	// For other batches, it will hit optimization 4.

	if len(branchNames) != len(commitIDs) {
		return fmt.Errorf("branchNames and commitIDs length not match")
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		branches, err := git_model.GetBranches(ctx, repoID, branchNames, true)
		if err != nil {
			return fmt.Errorf("git_model.GetBranches: %v", err)
		}

		if len(branches) == 0 {
			// if user haven't visit UI but directly push to a branch after upgrading from 1.20 -> 1.21,
			// we cannot simply insert the branch but need to check we have branches or not
			hasBranch, err := db.Exist[git_model.Branch](ctx, git_model.FindBranchOptions{
				RepoID:          repoID,
				IsDeletedBranch: optional.Some(false),
			}.ToConds())
			if err != nil {
				return err
			}
			if !hasBranch {
				if _, err = repo_module.SyncRepoBranches(ctx, repoID, pusherID); err != nil {
					return fmt.Errorf("repo_module.SyncRepoBranches %d failed: %v", repoID, err)
				}
				return nil
			}
		}

		branchMap := make(map[string]*git_model.Branch, len(branches))
		for _, branch := range branches {
			branchMap[branch.Name] = branch
		}

		newBranches := make([]*git_model.Branch, 0, len(branchNames))

		for i, branchName := range branchNames {
			commitID := commitIDs[i]
			branch, exist := branchMap[branchName]
			if exist && branch.CommitID == commitID && !branch.IsDeleted {
				continue
			}

			commit, err := getCommit(commitID)
			if err != nil {
				return fmt.Errorf("get commit of %s failed: %v", branchName, err)
			}

			if exist {
				if _, err := git_model.UpdateBranch(ctx, repoID, pusherID, branchName, commit); err != nil {
					return fmt.Errorf("git_model.UpdateBranch %d:%s failed: %v", repoID, branchName, err)
				}
				continue
			}

			// if database have branches but not this branch, it means this is a new branch
			newBranches = append(newBranches, &git_model.Branch{
				RepoID:        repoID,
				Name:          branchName,
				CommitID:      commit.ID.String(),
				CommitMessage: commit.Summary(),
				PusherID:      pusherID,
				CommitTime:    timeutil.TimeStamp(commit.Committer.When.Unix()),
			})
		}

		if len(newBranches) > 0 {
			return db.Insert(ctx, newBranches)
		}
		return nil
	})
}

// CreateNewBranchFromCommit creates a new repository branch
func CreateNewBranchFromCommit(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitID, branchName string) (err error) {
	err = repo.MustNotBeArchived()
	if err != nil {
		return err
	}

	// Check if branch name can be used
	if err := checkBranchName(ctx, repo, branchName); err != nil {
		return err
	}

	if err := git.Push(ctx, repo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", commitID, git.BranchPrefix, branchName),
		Env:    repo_module.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("push: %w", err)
	}
	return nil
}

// RenameBranch rename a branch
func RenameBranch(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, gitRepo *git.Repository, from, to string) (string, error) {
	err := repo.MustNotBeArchived()
	if err != nil {
		return "", err
	}

	if from == to {
		return "target_exist", nil
	}

	if gitRepo.IsBranchExist(to) {
		return "target_exist", nil
	}

	if !gitRepo.IsBranchExist(from) {
		return "from_not_exist", nil
	}

	if err := git_model.RenameBranch(ctx, repo, from, to, func(ctx context.Context, isDefault bool) error {
		err2 := gitRepo.RenameBranch(from, to)
		if err2 != nil {
			return err2
		}

		if isDefault {
			// if default branch changed, we need to delete all schedules and cron jobs
			if err := actions_model.DeleteScheduleTaskByRepo(ctx, repo.ID); err != nil {
				log.Error("DeleteCronTaskByRepo: %v", err)
			}
			// cancel running cron jobs of this repository and delete old schedules
			if err := actions_model.CancelPreviousJobs(
				ctx,
				repo.ID,
				from,
				"",
				webhook_module.HookEventSchedule,
			); err != nil {
				log.Error("CancelPreviousJobs: %v", err)
			}

			err2 = gitrepo.SetDefaultBranch(ctx, repo, to)
			if err2 != nil {
				return err2
			}
		}

		return nil
	}); err != nil {
		return "", err
	}
	refNameTo := git.RefNameFromBranch(to)
	refID, err := gitRepo.GetRefCommitID(refNameTo.String())
	if err != nil {
		return "", err
	}

	notify_service.DeleteRef(ctx, doer, repo, git.RefNameFromBranch(from))
	notify_service.CreateRef(ctx, doer, repo, refNameTo, refID)

	return "", nil
}

// enmuerates all branch related errors
var (
	ErrBranchIsDefault = errors.New("branch is default")
)

func CanDeleteBranch(ctx context.Context, repo *repo_model.Repository, branchName string, doer *user_model.User) error {
	if branchName == repo.DefaultBranch {
		return ErrBranchIsDefault
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWrite(unit.TypeCode) {
		return util.NewPermissionDeniedErrorf("permission denied to access repo %d unit %s", repo.ID, unit.TypeCode.LogString())
	}

	isProtected, err := git_model.IsBranchProtected(ctx, repo.ID, branchName)
	if err != nil {
		return err
	}
	if isProtected {
		return git_model.ErrBranchIsProtected
	}
	return nil
}

// DeleteBranch delete branch
func DeleteBranch(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, branchName string, pr *issues_model.PullRequest) error {
	err := repo.MustNotBeArchived()
	if err != nil {
		return err
	}

	if err := CanDeleteBranch(ctx, repo, branchName, doer); err != nil {
		return err
	}

	rawBranch, err := git_model.GetBranch(ctx, repo.ID, branchName)
	if err != nil && !git_model.IsErrBranchNotExist(err) {
		return fmt.Errorf("GetBranch: %vc", err)
	}

	// database branch record not exist or it's a deleted branch
	notExist := git_model.IsErrBranchNotExist(err) || rawBranch.IsDeleted

	commit, err := gitRepo.GetBranchCommit(branchName)
	if err != nil {
		return err
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if !notExist {
			if err := git_model.AddDeletedBranch(ctx, repo.ID, branchName, doer.ID); err != nil {
				return err
			}
		}

		if pr != nil {
			if err := issues_model.AddDeletePRBranchComment(ctx, doer, pr.BaseRepo, pr.Issue.ID, pr.HeadBranch); err != nil {
				return fmt.Errorf("DeleteBranch: %v", err)
			}
		}

		return gitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
			Force: true,
		})
	}); err != nil {
		return err
	}

	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)

	// Don't return error below this
	if err := PushUpdate(
		&repo_module.PushUpdateOptions{
			RefFullName:  git.RefNameFromBranch(branchName),
			OldCommitID:  commit.ID.String(),
			NewCommitID:  objectFormat.EmptyObjectID().String(),
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.OwnerName,
			RepoName:     repo.Name,
		}); err != nil {
		log.Error("Update: %v", err)
	}

	return nil
}

type BranchSyncOptions struct {
	RepoID int64
}

// branchSyncQueue represents a queue to handle branch sync jobs.
var branchSyncQueue *queue.WorkerPoolQueue[*BranchSyncOptions]

func handlerBranchSync(items ...*BranchSyncOptions) []*BranchSyncOptions {
	for _, opts := range items {
		_, err := repo_module.SyncRepoBranches(graceful.GetManager().ShutdownContext(), opts.RepoID, 0)
		if err != nil {
			log.Error("syncRepoBranches [%d] failed: %v", opts.RepoID, err)
		}
	}
	return nil
}

func addRepoToBranchSyncQueue(repoID int64) error {
	return branchSyncQueue.Push(&BranchSyncOptions{
		RepoID: repoID,
	})
}

func initBranchSyncQueue(ctx context.Context) error {
	branchSyncQueue = queue.CreateUniqueQueue(ctx, "branch_sync", handlerBranchSync)
	if branchSyncQueue == nil {
		return errors.New("unable to create branch_sync queue")
	}
	go graceful.GetManager().RunWithCancel(branchSyncQueue)

	return nil
}

func AddAllRepoBranchesToSyncQueue(ctx context.Context) error {
	if err := db.Iterate(ctx, builder.Eq{"is_empty": false}, func(ctx context.Context, repo *repo_model.Repository) error {
		return addRepoToBranchSyncQueue(repo.ID)
	}); err != nil {
		return fmt.Errorf("run sync all branches failed: %v", err)
	}
	return nil
}

func SetRepoDefaultBranch(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, newBranchName string) error {
	if repo.DefaultBranch == newBranchName {
		return nil
	}

	if !gitRepo.IsBranchExist(newBranchName) {
		return git_model.ErrBranchNotExist{
			BranchName: newBranchName,
		}
	}

	oldDefaultBranchName := repo.DefaultBranch
	repo.DefaultBranch = newBranchName
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := repo_model.UpdateDefaultBranch(ctx, repo); err != nil {
			return err
		}

		if err := actions_model.DeleteScheduleTaskByRepo(ctx, repo.ID); err != nil {
			log.Error("DeleteCronTaskByRepo: %v", err)
		}
		// cancel running cron jobs of this repository and delete old schedules
		if err := actions_model.CancelPreviousJobs(
			ctx,
			repo.ID,
			oldDefaultBranchName,
			"",
			webhook_module.HookEventSchedule,
		); err != nil {
			log.Error("CancelPreviousJobs: %v", err)
		}

		return gitrepo.SetDefaultBranch(ctx, repo, newBranchName)
	}); err != nil {
		return err
	}

	if !repo.IsEmpty {
		if err := AddRepoToLicenseUpdaterQueue(&LicenseUpdaterOptions{
			RepoID: repo.ID,
		}); err != nil {
			log.Error("AddRepoToLicenseUpdaterQueue: %v", err)
		}
	}

	notify_service.ChangeDefaultBranch(ctx, repo)

	return nil
}
