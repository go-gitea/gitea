// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/unknwon/com"
)

// pullRequestQueue represents a queue to handle update pull request tests
var pullRequestQueue = sync.NewUniqueQueue(setting.Repository.PullRequestQueueLength)

// AddToTaskQueue adds itself to pull request test task queue.
func AddToTaskQueue(pr *models.PullRequest) {
	go pullRequestQueue.AddFunc(pr.ID, func() {
		pr.Status = models.PullRequestStatusChecking
		if err := pr.UpdateColsIfNotMerged("status"); err != nil {
			log.Error("AddToTaskQueue.UpdateCols[%d].(add to queue): %v", pr.ID, err)
		}
	})
}

// checkAndUpdateStatus checks if pull request is possible to leaving checking status,
// and set to be either conflict or mergeable.
func checkAndUpdateStatus(pr *models.PullRequest) {
	// Status is not changed to conflict means mergeable.
	if pr.Status == models.PullRequestStatusChecking {
		pr.Status = models.PullRequestStatusMergeable
	}

	// Make sure there is no waiting test to process before leaving the checking status.
	if !pullRequestQueue.Exist(pr.ID) {
		if err := pr.UpdateCols("merge_base", "status", "conflicted_files"); err != nil {
			log.Error("Update[%d]: %v", pr.ID, err)
		}
	}
}

// getMergeCommit checks if a pull request got merged
// Returns the git.Commit of the pull request if merged
func getMergeCommit(pr *models.PullRequest) (*git.Commit, error) {
	if pr.BaseRepo == nil {
		var err error
		pr.BaseRepo, err = models.GetRepositoryByID(pr.BaseRepoID)
		if err != nil {
			return nil, fmt.Errorf("GetRepositoryByID: %v", err)
		}
	}

	indexTmpPath, err := ioutil.TempDir(os.TempDir(), "gitea-"+pr.BaseRepo.Name)
	if err != nil {
		return nil, fmt.Errorf("Failed to create temp dir for repository %s: %v", pr.BaseRepo.RepoPath(), err)
	}
	defer os.RemoveAll(indexTmpPath)

	headFile := pr.GetGitRefName()

	// Check if a pull request is merged into BaseBranch
	_, err = git.NewCommand("merge-base", "--is-ancestor", headFile, pr.BaseBranch).RunInDirWithEnv(pr.BaseRepo.RepoPath(), []string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()})
	if err != nil {
		// Errors are signaled by a non-zero status that is not 1
		if strings.Contains(err.Error(), "exit status 1") {
			return nil, nil
		}
		return nil, fmt.Errorf("git merge-base --is-ancestor: %v", err)
	}

	commitIDBytes, err := ioutil.ReadFile(pr.BaseRepo.RepoPath() + "/" + headFile)
	if err != nil {
		return nil, fmt.Errorf("ReadFile(%s): %v", headFile, err)
	}
	commitID := string(commitIDBytes)
	if len(commitID) < 40 {
		return nil, fmt.Errorf(`ReadFile(%s): invalid commit-ID "%s"`, headFile, commitID)
	}
	cmd := commitID[:40] + ".." + pr.BaseBranch

	// Get the commit from BaseBranch where the pull request got merged
	mergeCommit, err := git.NewCommand("rev-list", "--ancestry-path", "--merges", "--reverse", cmd).RunInDirWithEnv("", []string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()})
	if err != nil {
		return nil, fmt.Errorf("git rev-list --ancestry-path --merges --reverse: %v", err)
	} else if len(mergeCommit) < 40 {
		// PR was fast-forwarded, so just use last commit of PR
		mergeCommit = commitID[:40]
	}

	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(mergeCommit[:40])
	if err != nil {
		return nil, fmt.Errorf("GetCommit: %v", err)
	}

	return commit, nil
}

// manuallyMerged checks if a pull request got manually merged
// When a pull request got manually merged mark the pull request as merged
func manuallyMerged(pr *models.PullRequest) bool {
	commit, err := getMergeCommit(pr)
	if err != nil {
		log.Error("PullRequest[%d].getMergeCommit: %v", pr.ID, err)
		return false
	}
	if commit != nil {
		pr.MergedCommitID = commit.ID.String()
		pr.MergedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		pr.Status = models.PullRequestStatusManuallyMerged
		merger, _ := models.GetUserByEmail(commit.Author.Email)

		// When the commit author is unknown set the BaseRepo owner as merger
		if merger == nil {
			if pr.BaseRepo.Owner == nil {
				if err = pr.BaseRepo.GetOwner(); err != nil {
					log.Error("BaseRepo.GetOwner[%d]: %v", pr.ID, err)
					return false
				}
			}
			merger = pr.BaseRepo.Owner
		}
		pr.Merger = merger
		pr.MergerID = merger.ID

		if merged, err := pr.SetMerged(); err != nil {
			log.Error("PullRequest[%d].setMerged : %v", pr.ID, err)
			return false
		} else if !merged {
			return false
		}

		baseGitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s] : %v", pr.BaseRepo.RepoPath(), err)
			return false
		}

		notification.NotifyMergePullRequest(pr, merger, baseGitRepo)

		log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commit.ID.String())
		return true
	}
	return false
}

// TestPullRequests checks and tests untested patches of pull requests.
// TODO: test more pull requests at same time.
func TestPullRequests(ctx context.Context) {

	go func() {
		prs, err := models.GetPullRequestIDsByCheckStatus(models.PullRequestStatusChecking)
		if err != nil {
			log.Error("Find Checking PRs: %v", err)
			return
		}
		for _, prID := range prs {
			select {
			case <-ctx.Done():
				return
			default:
				pullRequestQueue.Add(prID)
			}
		}
	}()

	// Start listening on new test requests.
	for {
		select {
		case prID := <-pullRequestQueue.Queue():
			log.Trace("TestPullRequests[%v]: processing test task", prID)
			pullRequestQueue.Remove(prID)

			id := com.StrTo(prID).MustInt64()

			pr, err := models.GetPullRequestByID(id)
			if err != nil {
				log.Error("GetPullRequestByID[%s]: %v", prID, err)
				continue
			} else if pr.HasMerged {
				continue
			} else if manuallyMerged(pr) {
				continue
			} else if err = TestPatch(pr); err != nil {
				log.Error("testPatch[%d]: %v", pr.ID, err)
				pr.Status = models.PullRequestStatusError
				if err := pr.UpdateCols("status"); err != nil {
					log.Error("Unable to update status of pr %d: %v", pr.ID, err)
				}
				continue
			}
			checkAndUpdateStatus(pr)
		case <-ctx.Done():
			pullRequestQueue.Close()
			log.Info("PID: %d Pull Request testing shutdown", os.Getpid())
			return
		}
	}
}

// Init runs the task queue to test all the checking status pull requests
func Init() {
	go graceful.GetManager().RunWithShutdownContext(TestPullRequests)
}
