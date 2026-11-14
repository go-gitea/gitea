// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
)

func TestIsAuthenticationError(t *testing.T) {
	errDummy := errors.New("dummy")
	cases := []struct {
		name string
		want bool
		err  error
	}{
		{"git authentication failed", true, gitcmd.NewRunStdError(errDummy, "fatal: Authentication failed for 'https://host/repo.git/'")},
		{"git could not read username", true, fmt.Errorf("%w", gitcmd.NewRunStdError(errDummy, "fatal: could not read Username for 'https://host'"))},
		{"github unauthorized", true, util.SanitizeErrorCredentialURLs(&github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusUnauthorized}})},
		{"github other", false, &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}}},
		{"github nil response", false, &github.ErrorResponse{}},
		{"bitbucket server unauthorized", true, fmt.Errorf("wrapped: %w", &bbsStatusError{Status: http.StatusUnauthorized})},
		{"bitbucket server forbidden", true, &bbsStatusError{Status: http.StatusForbidden}},
		{"bitbucket server not found", false, &bbsStatusError{Status: http.StatusNotFound}},
		{"unrelated error", false, errDummy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsAuthenticationError(c.err))
		})
	}
}

func TestMigrateWhiteBlocklist(t *testing.T) {
	adminUser := &user_model.User{IsAdmin: true}
	nonAdminUser := &user_model.User{}

	defer test.MockVariableValue(&setting.Migrations.AllowedHostList, "")()
	defer test.MockVariableValue(&setting.Migrations.BlockedHostList, "8.8.4.4")()
	assert.NoError(t, IsMigrateURLAllowed("https://8.8.8.8/go-gitea/gitea.git", nonAdminUser))
	assert.Error(t, IsMigrateURLAllowed("https://8.8.4.4/go-gitea/gitea.git", nonAdminUser))
	assert.Error(t, IsMigrateURLAllowed("https://[64:ff9b::a9fe:a9fe]/go-gitea/gitea.git", nonAdminUser))

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = false

	assert.Error(t, IsMigrateURLAllowed("/home/foo/bar/goo", adminUser))

	setting.ImportLocalPaths = true
	abs, err := filepath.Abs(".")
	assert.NoError(t, err)

	assert.NoError(t, IsMigrateURLAllowed(abs, adminUser))
	assert.Error(t, IsMigrateURLAllowed(abs, nonAdminUser))

	nonAdminUser.AllowImportLocal = true
	assert.NoError(t, IsMigrateURLAllowed(abs, nonAdminUser))

	setting.ImportLocalPaths = old
}

func TestIsMigrateURLAllowedForServiceLocalPath(t *testing.T) {
	defer test.MockVariableValue(&setting.ImportLocalPaths, true)()
	admin := &user_model.User{IsAdmin: true}
	for _, addr := range []string{"/tmp", "file:///tmp"} {
		assert.NoError(t, IsMigrateURLAllowedForService(addr, structs.PlainGitService, admin))
		for _, svc := range []structs.GitServiceType{structs.GithubService, structs.GiteaService, structs.GitlabService, structs.BitbucketServerService} {
			err := IsMigrateURLAllowedForService(addr, svc, admin)
			assert.ErrorAs(t, err, new(*git.ErrInvalidCloneAddr), "%s %d", addr, svc)
		}
	}
}

func TestMigrateRepositoryIssueData(t *testing.T) {
	for repoComments, want := range map[bool][]string{
		false: {"issues [1 2]", "comments [1 2]", "issues [3]", "comments [3]", "pulls [4]", "comments [4]", "reviews [4]", "sync branches"},
		true:  {"issues [1 2]", "issues [3]", "pulls [4]", "reviews [4]", "sync branches", "comments [1]", "comments [4]"},
	} {
		migration := &issueMigration{repoComments: repoComments}
		assert.NoError(t, migrateRepositoryIssueData(t.Context(), migration, migration, base.MigrateOptions{Issues: true, PullRequests: true, Comments: true}, base.NilMessenger))
		assert.Equal(t, want, migration.calls)
	}
}

type issueMigration struct {
	base.NullDownloader
	base.Uploader
	repoComments bool
	calls        []string
}

func (m *issueMigration) MaxBatchInsertSize(_tp string) int { return 2 }
func (m *issueMigration) SupportGetRepoComments() bool      { return m.repoComments }

func (m *issueMigration) GetIssues(_ctx context.Context, page, _perPage int) ([]*base.Issue, bool, error) {
	return [][]*base.Issue{{{Number: 1}, {Number: 2}}, {{Number: 2}, {Number: 3}}}[page-1], page == 2, nil
}

func (m *issueMigration) GetPullRequests(_ctx context.Context, _page, _perPage int) ([]*base.PullRequest, bool, error) {
	return []*base.PullRequest{{Number: 4}}, true, nil
}

func (m *issueMigration) GetComments(_ctx context.Context, item base.Commentable) ([]*base.Comment, bool, error) {
	return []*base.Comment{{IssueIndex: item.GetLocalIndex()}}, true, nil
}

func (m *issueMigration) GetAllComments(_ctx context.Context, page, _perPage int) ([]*base.Comment, bool, error) {
	return [][]*base.Comment{{{IssueIndex: 1}, {IssueIndex: 7}}, {{IssueIndex: 4}}}[page-1], page == 2, nil
}

func (m *issueMigration) GetReviews(_ctx context.Context, item base.Reviewable) ([]*base.Review, error) {
	return []*base.Review{{IssueIndex: item.GetLocalIndex()}}, nil
}

func (m *issueMigration) CreateIssues(_ctx context.Context, issues ...*base.Issue) error {
	return recordMigrationCall(m, "issues", issues, (*base.Issue).GetLocalIndex)
}

func (m *issueMigration) CreatePullRequests(_ctx context.Context, pulls ...*base.PullRequest) error {
	return recordMigrationCall(m, "pulls", pulls, (*base.PullRequest).GetLocalIndex)
}

func (m *issueMigration) CreateComments(_ctx context.Context, comments ...*base.Comment) error {
	return recordMigrationCall(m, "comments", comments, func(comment *base.Comment) int64 { return comment.IssueIndex })
}

func (m *issueMigration) CreateReviews(_ctx context.Context, reviews ...*base.Review) error {
	return recordMigrationCall(m, "reviews", reviews, func(review *base.Review) int64 { return review.IssueIndex })
}

func (m *issueMigration) SyncBranches(_ctx context.Context) error {
	m.calls = append(m.calls, "sync branches")
	return nil
}

func recordMigrationCall[Item any](migration *issueMigration, name string, items []Item, index func(Item) int64) error {
	indexes := make([]int64, len(items))
	for i, item := range items {
		indexes[i] = index(item)
	}
	migration.calls = append(migration.calls, fmt.Sprintf("%s %v", name, indexes))
	return nil
}
