// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
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
		if err := pr.UpdateCols("status"); err != nil {
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
		if err := pr.UpdateCols("status, conflicted_files"); err != nil {
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

	indexTmpPath := filepath.Join(os.TempDir(), "gitea-"+pr.BaseRepo.Name+"-"+strconv.Itoa(time.Now().Nanosecond()))
	defer os.Remove(indexTmpPath)

	headFile := pr.GetGitRefName()

	// Check if a pull request is merged into BaseBranch
	_, err := git.NewCommand("merge-base", "--is-ancestor", headFile, pr.BaseBranch).RunInDirWithEnv(pr.BaseRepo.RepoPath(), []string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()})
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

		if err = pr.SetMerged(); err != nil {
			log.Error("PullRequest[%d].setMerged : %v", pr.ID, err)
			return false
		}
		log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commit.ID.String())
		return true
	}
	return false
}

// patchConflicts is a list of conflict description from Git.
var patchConflicts = []string{
	"patch does not apply",
	"already exists in working directory",
	"unrecognized input",
	"error:",
}

// testPatch checks if patch can be merged to base repository without conflict.
func testPatch(pr *models.PullRequest, ctx models.DBContext) (err error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	patchPath, err := pr.BaseRepo.PatchPath(ctx, pr.Index)
	if err != nil {
		return fmt.Errorf("BaseRepo.PatchPath: %v", err)
	}

	// Fast fail if patch does not exist, this assumes data is corrupted.
	if !com.IsFile(patchPath) {
		log.Trace("PullRequest[%d].testPatch: ignored corrupted data", pr.ID)
		return nil
	}

	models.RepoWorkingPool.CheckIn(com.ToStr(pr.BaseRepoID))
	defer models.RepoWorkingPool.CheckOut(com.ToStr(pr.BaseRepoID))

	log.Trace("PullRequest[%d].testPatch (patchPath): %s", pr.ID, patchPath)

	pr.Status = models.PullRequestStatusChecking

	indexTmpPath := filepath.Join(os.TempDir(), "gitea-"+pr.BaseRepo.Name+"-"+strconv.Itoa(time.Now().Nanosecond()))
	defer os.Remove(indexTmpPath)

	_, err = git.NewCommand("read-tree", pr.BaseBranch).RunInDirWithEnv("", []string{"GIT_DIR=" + pr.BaseRepo.RepoPath(), "GIT_INDEX_FILE=" + indexTmpPath})
	if err != nil {
		return fmt.Errorf("git read-tree --index-output=%s %s: %v", indexTmpPath, pr.BaseBranch, err)
	}

	prUnit, err := models.GetRepoUnit(ctx, pr.BaseRepo, models.UnitTypePullRequests)
	if err != nil {
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	args := []string{"apply", "--check", "--cached"}
	if prConfig.IgnoreWhitespaceConflicts {
		args = append(args, "--ignore-whitespace")
	}
	args = append(args, patchPath)
	pr.ConflictedFiles = []string{}

	stderrBuilder := new(strings.Builder)
	err = git.NewCommand(args...).RunInDirTimeoutEnvPipeline(
		[]string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()},
		-1,
		"",
		nil,
		stderrBuilder)
	stderr := stderrBuilder.String()

	if err != nil {
		for i := range patchConflicts {
			if strings.Contains(stderr, patchConflicts[i]) {
				log.Trace("PullRequest[%d].testPatch (apply): has conflict: %s", pr.ID, stderr)
				const prefix = "error: patch failed:"
				pr.Status = models.PullRequestStatusConflict
				pr.ConflictedFiles = make([]string, 0, 5)
				scanner := bufio.NewScanner(strings.NewReader(stderr))
				for scanner.Scan() {
					line := scanner.Text()

					if strings.HasPrefix(line, prefix) {
						var found bool
						var filepath = strings.TrimSpace(strings.Split(line[len(prefix):], ":")[0])
						for _, f := range pr.ConflictedFiles {
							if f == filepath {
								found = true
								break
							}
						}
						if !found {
							pr.ConflictedFiles = append(pr.ConflictedFiles, filepath)
						}
					}
					// only list 10 conflicted files
					if len(pr.ConflictedFiles) >= 10 {
						break
					}
				}

				if len(pr.ConflictedFiles) > 0 {
					log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
				}

				return nil
			}
		}

		return fmt.Errorf("git apply --check: %v - %s", err, stderr)
	}
	return nil
}

// TestPullRequests checks and tests untested patches of pull requests.
// TODO: test more pull requests at same time.
func TestPullRequests() {
	prs, err := models.GetPullRequestsByCheckStatus(models.PullRequestStatusChecking)
	if err != nil {
		log.Error("Find Checking PRs: %v", err)
		return
	}

	var checkedPRs = make(map[int64]struct{})

	// Update pull request status.
	for _, pr := range prs {
		checkedPRs[pr.ID] = struct{}{}
		if err := pr.GetBaseRepo(); err != nil {
			log.Error("GetBaseRepo: %v", err)
			continue
		}
		if manuallyMerged(pr) {
			continue
		}
		if err := testPatch(pr, models.DefaultDBContext()); err != nil {
			log.Error("testPatch: %v", err)
			continue
		}

		checkAndUpdateStatus(pr)
	}

	// Start listening on new test requests.
	for prID := range pullRequestQueue.Queue() {
		log.Trace("TestPullRequests[%v]: processing test task", prID)
		pullRequestQueue.Remove(prID)

		id := com.StrTo(prID).MustInt64()
		if _, ok := checkedPRs[id]; ok {
			continue
		}

		pr, err := models.GetPullRequestByID(id)
		if err != nil {
			log.Error("GetPullRequestByID[%s]: %v", prID, err)
			continue
		} else if manuallyMerged(pr) {
			continue
		} else if err = testPatch(pr, models.DefaultDBContext()); err != nil {
			log.Error("testPatch[%d]: %v", pr.ID, err)
			continue
		}

		checkAndUpdateStatus(pr)
	}
}

// Init runs the task queue to test all the checking status pull requests
func Init() {
	go TestPullRequests()
}
