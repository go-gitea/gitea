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
	"github.com/stretchr/testify/require"
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
	defer test.MockVariableValue(&setting.ImportLocalPaths, true)()
	for _, tc := range []struct {
		name         string
		repoComments bool
		want         []string
	}{
		{
			name: "per-item comments",
			want: []string{
				"labels [bug help]", "labels [feature]",
				"issues [1 2]", "comments [1 1]", "comments [2]",
				"issues [3]", "comments [3]",
				"pulls [4]", "comments [4]", "reviews [4]", "sync branches", "finish", "close",
			},
		},
		{
			name:         "repository comments",
			repoComments: true,
			want: []string{
				"labels [bug help]", "labels [feature]",
				"issues [1 2]", "issues [3]", "pulls [4]", "reviews [4]",
				"sync branches", "comments [1]", "comments [4]", "finish", "close",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cloneAddr := t.TempDir()
			downloader := &issueMigrationDownloader{cloneAddr: cloneAddr, repoComments: tc.repoComments}
			uploader := &issueMigrationUploader{}
			opts := base.MigrateOptions{CloneAddr: cloneAddr, Labels: true, Issues: true, PullRequests: true, Comments: true}

			require.NoError(t, migrateRepository(t.Context(), &user_model.User{IsAdmin: true}, downloader, uploader, opts, nil))
			assert.Equal(t, tc.want, uploader.calls)
		})
	}
}

type issueMigrationDownloader struct {
	base.NullDownloader
	cloneAddr    string
	repoComments bool
}

func (d *issueMigrationDownloader) GetRepoInfo(context.Context) (*base.Repository, error) {
	return &base.Repository{CloneURL: d.cloneAddr}, nil
}

func (d *issueMigrationDownloader) GetLabels(context.Context) ([]*base.Label, error) {
	return []*base.Label{{Name: "bug"}, {Name: "help"}, {Name: "feature"}}, nil
}

func (d *issueMigrationDownloader) GetIssues(_ context.Context, page, _ int) ([]*base.Issue, bool, error) {
	if page == 1 {
		return []*base.Issue{{Number: 1}, {Number: 2}}, false, nil
	}
	return []*base.Issue{{Number: 2}, {Number: 3}}, true, nil
}

func (d *issueMigrationDownloader) GetPullRequests(context.Context, int, int) ([]*base.PullRequest, bool, error) {
	return []*base.PullRequest{{Number: 4}}, true, nil
}

func (d *issueMigrationDownloader) SupportGetRepoComments() bool {
	return d.repoComments
}

func (d *issueMigrationDownloader) GetComments(_ context.Context, item base.Commentable) ([]*base.Comment, bool, error) {
	if d.repoComments {
		return nil, false, errors.New("unexpected per-item comment request")
	}
	index := item.GetLocalIndex()
	comments := []*base.Comment{{IssueIndex: index}}
	if index == 1 {
		comments = append(comments, &base.Comment{IssueIndex: index})
	}
	return comments, true, nil
}

func (d *issueMigrationDownloader) GetAllComments(_ context.Context, page, _ int) ([]*base.Comment, bool, error) {
	if !d.repoComments {
		return nil, false, errors.New("unexpected repository comment request")
	}
	if page == 1 {
		return []*base.Comment{{IssueIndex: 1}, {IssueIndex: 5}}, false, nil
	}
	return []*base.Comment{{IssueIndex: 4}}, true, nil
}

func (d *issueMigrationDownloader) GetReviews(_ context.Context, item base.Reviewable) ([]*base.Review, error) {
	return []*base.Review{{IssueIndex: item.GetLocalIndex()}}, nil
}

type issueMigrationUploader struct {
	base.Uploader
	calls []string
}

func (u *issueMigrationUploader) MaxBatchInsertSize(string) int { return 2 }

func (u *issueMigrationUploader) CreateRepo(context.Context, *base.Repository, base.MigrateOptions) error {
	return nil
}

func (u *issueMigrationUploader) CreateLabels(_ context.Context, labels ...*base.Label) error {
	u.calls = append(u.calls, migrationCall("labels", labels, func(label *base.Label) string { return label.Name }))
	return nil
}

func (u *issueMigrationUploader) CreateIssues(_ context.Context, issues ...*base.Issue) error {
	u.calls = append(u.calls, migrationCall("issues", issues, (*base.Issue).GetLocalIndex))
	return nil
}

func (u *issueMigrationUploader) CreatePullRequests(_ context.Context, pulls ...*base.PullRequest) error {
	u.calls = append(u.calls, migrationCall("pulls", pulls, (*base.PullRequest).GetLocalIndex))
	return nil
}

func (u *issueMigrationUploader) CreateComments(_ context.Context, comments ...*base.Comment) error {
	u.calls = append(u.calls, migrationCall("comments", comments, func(comment *base.Comment) int64 { return comment.IssueIndex }))
	return nil
}

func (u *issueMigrationUploader) CreateReviews(_ context.Context, reviews ...*base.Review) error {
	u.calls = append(u.calls, migrationCall("reviews", reviews, func(review *base.Review) int64 { return review.IssueIndex }))
	return nil
}

func (u *issueMigrationUploader) SyncBranches(context.Context) error {
	u.calls = append(u.calls, "sync branches")
	return nil
}

func (u *issueMigrationUploader) Finish(context.Context) error {
	u.calls = append(u.calls, "finish")
	return nil
}

func (u *issueMigrationUploader) Close() {
	u.calls = append(u.calls, "close")
}

func migrationCall[T, V any](name string, items []T, value func(T) V) string {
	values := make([]V, 0, len(items))
	for _, item := range items {
		values = append(values, value(item))
	}
	return fmt.Sprintf("%s %v", name, values)
}
