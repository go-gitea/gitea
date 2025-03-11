// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestGiteaUploadRepo(t *testing.T) {
	// FIXME: Since no accesskey or user/password will trigger rate limit of github, just skip
	t.Skip()

	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	var (
		ctx        = t.Context()
		downloader = NewGithubDownloaderV3(ctx, "https://github.com", "", "", "", "go-xorm", "builder")
		repoName   = "builder-" + time.Now().Format("2006-01-02-15-04-05")
		uploader   = NewGiteaLocalUploader(graceful.GetManager().HammerContext(), user, user.Name, repoName)
	)

	err := migrateRepository(db.DefaultContext, user, downloader, uploader, base.MigrateOptions{
		CloneAddr:    "https://github.com/go-xorm/builder",
		RepoName:     repoName,
		AuthUsername: "",

		Wiki:         true,
		Issues:       true,
		Milestones:   true,
		Labels:       true,
		Releases:     true,
		Comments:     true,
		PullRequests: true,
		Private:      true,
		Mirror:       false,
	}, nil)
	assert.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName})
	assert.True(t, repo.HasWiki())
	assert.EqualValues(t, repo_model.RepositoryReady, repo.Status)

	milestones, err := db.Find[issues_model.Milestone](db.DefaultContext, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(false),
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 1)

	milestones, err = db.Find[issues_model.Milestone](db.DefaultContext, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(true),
	})
	assert.NoError(t, err)
	assert.Empty(t, milestones)

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, labels, 12)

	releases, err := db.Find[repo_model.Release](db.DefaultContext, repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: true,
		RepoID:      repo.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 8)

	releases, err = db.Find[repo_model.Release](db.DefaultContext, repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: false,
		RepoID:      repo.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	issues, err := issues_model.Issues(db.DefaultContext, &issues_model.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsPull:   optional.Some(false),
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 15)
	assert.NoError(t, issues[0].LoadDiscussComments(db.DefaultContext))
	assert.Empty(t, issues[0].Comments)

	pulls, _, err := issues_model.PullRequests(db.DefaultContext, repo.ID, &issues_model.PullRequestsOptions{
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, pulls, 30)
	assert.NoError(t, pulls[0].LoadIssue(db.DefaultContext))
	assert.NoError(t, pulls[0].Issue.LoadDiscussComments(db.DefaultContext))
	assert.Len(t, pulls[0].Issue.Comments, 2)
}

func TestGiteaUploadRemapLocalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	ctx := t.Context()
	repoName := "migrated"
	uploader := NewGiteaLocalUploader(ctx, doer, doer.Name, repoName)
	// call remapLocalUser
	uploader.sameApp = true

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// The externalID does not match any existing user, everything
	// belongs to the doer
	//
	target := repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err := uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID matches a known user but the name does not match,
	// everything belongs to the doer
	//
	source.PublisherID = user.ID
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID and externalName match an existing user, everything
	// belongs to the existing user
	//
	source.PublisherName = user.Name
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, user.ID, target.GetUserID())
}

func TestGiteaUploadRemapExternalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	ctx := t.Context()
	repoName := "migrated"
	uploader := NewGiteaLocalUploader(ctx, doer, doer.Name, repoName)
	uploader.gitServiceType = structs.GiteaService
	// call remapExternalUser
	uploader.sameApp = false

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// When there is no user linked to the external ID, the migrated data is authored
	// by the doer
	//
	uploader.userMap = make(map[int64]int64)
	target := repo_model.Release{}
	err := uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// Link the external ID to an existing user
	//
	linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    strconv.FormatInt(externalID, 10),
		UserID:        linkedUser.ID,
		LoginSourceID: 0,
		Provider:      structs.GiteaService.Name(),
	}
	err = user_model.LinkExternalToUser(db.DefaultContext, linkedUser, externalLoginUser)
	assert.NoError(t, err)

	//
	// When a user is linked to the external ID, it becomes the author of
	// the migrated data
	//
	uploader.userMap = make(map[int64]int64)
	target = repo_model.Release{}
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, linkedUser.ID, target.GetUserID())
}

func TestGiteaUploadUpdateGitForPullRequest(t *testing.T) {
	unittest.PrepareTestEnv(t)

	//
	// fromRepo master
	//
	fromRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	baseRef := "master"
	assert.NoError(t, git.InitRepository(git.DefaultContext, fromRepo.RepoPath(), false, fromRepo.ObjectFormatName))
	err := git.NewCommand("symbolic-ref").AddDynamicArguments("HEAD", git.BranchPrefix+baseRef).Run(git.DefaultContext, &git.RunOpts{Dir: fromRepo.RepoPath()})
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(fromRepo.RepoPath(), "README.md"), []byte(fmt.Sprintf("# Testing Repository\n\nOriginally created in: %s", fromRepo.RepoPath())), 0o644))
	assert.NoError(t, git.AddChanges(fromRepo.RepoPath(), true))
	signature := git.Signature{
		Email: "test@example.com",
		Name:  "test",
		When:  time.Now(),
	}
	assert.NoError(t, git.CommitChanges(fromRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "Initial Commit",
	}))
	fromGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, fromRepo)
	assert.NoError(t, err)
	defer fromGitRepo.Close()
	baseSHA, err := fromGitRepo.GetBranchCommitID(baseRef)
	assert.NoError(t, err)

	//
	// fromRepo branch1
	//
	headRef := "branch1"
	_, _, err = git.NewCommand("checkout", "-b").AddDynamicArguments(headRef).RunStdString(git.DefaultContext, &git.RunOpts{Dir: fromRepo.RepoPath()})
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(fromRepo.RepoPath(), "README.md"), []byte("SOMETHING"), 0o644))
	assert.NoError(t, git.AddChanges(fromRepo.RepoPath(), true))
	signature.When = time.Now()
	assert.NoError(t, git.CommitChanges(fromRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "Pull request",
	}))
	assert.NoError(t, err)
	headSHA, err := fromGitRepo.GetBranchCommitID(headRef)
	assert.NoError(t, err)

	fromRepoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: fromRepo.OwnerID})

	//
	// forkRepo branch2
	//
	forkHeadRef := "branch2"
	forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})
	assert.NoError(t, git.CloneWithArgs(git.DefaultContext, nil, fromRepo.RepoPath(), forkRepo.RepoPath(), git.CloneRepoOptions{
		Branch: headRef,
	}))
	_, _, err = git.NewCommand("checkout", "-b").AddDynamicArguments(forkHeadRef).RunStdString(git.DefaultContext, &git.RunOpts{Dir: forkRepo.RepoPath()})
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(forkRepo.RepoPath(), "README.md"), []byte(fmt.Sprintf("# branch2 %s", forkRepo.RepoPath())), 0o644))
	assert.NoError(t, git.AddChanges(forkRepo.RepoPath(), true))
	assert.NoError(t, git.CommitChanges(forkRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "branch2 commit",
	}))
	forkGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, forkRepo)
	assert.NoError(t, err)
	defer forkGitRepo.Close()
	forkHeadSHA, err := forkGitRepo.GetBranchCommitID(forkHeadRef)
	assert.NoError(t, err)

	toRepoName := "migrated"
	ctx := t.Context()
	uploader := NewGiteaLocalUploader(ctx, fromRepoOwner, fromRepoOwner.Name, toRepoName)
	uploader.gitServiceType = structs.GiteaService

	assert.NoError(t, repo_service.Init(t.Context()))
	assert.NoError(t, uploader.CreateRepo(ctx, &base.Repository{
		Description: "description",
		OriginalURL: fromRepo.RepoPath(),
		CloneURL:    fromRepo.RepoPath(),
		IsPrivate:   false,
		IsMirror:    true,
	}, base.MigrateOptions{
		GitServiceType: structs.GiteaService,
		Private:        false,
		Mirror:         true,
	}))

	for _, testCase := range []struct {
		name        string
		head        string
		logFilter   []string
		logFiltered []bool
		pr          base.PullRequest
	}{
		{
			name: "fork, good Head.SHA",
			head: fmt.Sprintf("%s/%s", forkRepo.OwnerName, forkHeadRef),
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  forkRepo.RepoPath(),
					Ref:       forkHeadRef,
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: forkRepo.OwnerName,
				},
			},
		},
		{
			name: "fork, invalid Head.Ref",
			head: "unknown repository",
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  forkRepo.RepoPath(),
					Ref:       "INVALID",
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: forkRepo.OwnerName,
				},
			},
			logFilter:   []string{"Fetch branch from"},
			logFiltered: []bool{true},
		},
		{
			name: "invalid fork CloneURL",
			head: "unknown repository",
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  "UNLIKELY",
					Ref:       forkHeadRef,
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: "WRONG",
				},
			},
			logFilter:   []string{"AddRemote"},
			logFiltered: []bool{true},
		},
		{
			name: "no fork, good Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       headSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
		},
		{
			name: "no fork, empty Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Empty reference", "Cannot remove local head"},
			logFiltered: []bool{true, false},
		},
		{
			name: "no fork, invalid Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "brokenSHA",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Deprecated local head"},
			logFiltered: []bool{true},
		},
		{
			name: "no fork, not found Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				PatchURL: "",
				Number:   1,
				State:    "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "2697b352310fcd01cbd1f3dbd43b894080027f68",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Deprecated local head", "Cannot remove local head"},
			logFiltered: []bool{true, false},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stopMark := fmt.Sprintf(">>>>>>>>>>>>>STOP: %s<<<<<<<<<<<<<<<", testCase.name)

			logChecker, cleanup := test.NewLogChecker(log.DEFAULT)
			logChecker.Filter(testCase.logFilter...).StopMark(stopMark)
			defer cleanup()

			testCase.pr.EnsuredSafe = true

			head, err := uploader.updateGitForPullRequest(ctx, &testCase.pr)
			assert.NoError(t, err)
			assert.EqualValues(t, testCase.head, head)

			log.Info(stopMark)

			logFiltered, logStopped := logChecker.Check(5 * time.Second)
			assert.True(t, logStopped)
			if len(testCase.logFilter) > 0 {
				assert.EqualValues(t, testCase.logFiltered, logFiltered, "for log message filters: %v", testCase.logFilter)
			}
		})
	}
}
