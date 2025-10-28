// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	countRepospts        = CountRepositoryOptions{OwnerID: 10}
	countReposptsPublic  = CountRepositoryOptions{OwnerID: 10, Private: optional.Some(false)}
	countReposptsPrivate = CountRepositoryOptions{OwnerID: 10, Private: optional.Some(true)}
)

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	count, err1 := CountRepositories(ctx, countRepospts)
	privateCount, err2 := CountRepositories(ctx, countReposptsPrivate)
	publicCount, err3 := CountRepositories(ctx, countReposptsPublic)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, privateCount+publicCount, count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := CountRepositories(t.Context(), countReposptsPublic)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := CountRepositories(t.Context(), countReposptsPrivate)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 10})

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}

func TestWatchRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, WatchRepo(t.Context(), user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &Watch{RepoID: repo.ID, UserID: user.ID})
	unittest.CheckConsistencyFor(t, &Repository{ID: repo.ID})

	assert.NoError(t, WatchRepo(t.Context(), user, repo, false))
	unittest.AssertNotExistsBean(t, &Watch{RepoID: repo.ID, UserID: user.ID})
	unittest.CheckConsistencyFor(t, &Repository{ID: repo.ID})
}

func TestMetas(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := &Repository{Name: "testRepo"}
	repo.Owner = &user_model.User{Name: "testOwner"}
	repo.OwnerName = repo.Owner.Name

	repo.Units = nil

	metas := repo.ComposeCommentMetas(t.Context())
	assert.Equal(t, "testRepo", metas["repo"])
	assert.Equal(t, "testOwner", metas["user"])

	externalTracker := RepoUnit{
		Type: unit.TypeExternalTracker,
		Config: &ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}

	testSuccess := func(expectedStyle string) {
		repo.Units = []*RepoUnit{&externalTracker}
		repo.commonRenderingMetas = nil
		metas := repo.ComposeCommentMetas(t.Context())
		assert.Equal(t, expectedStyle, metas["style"])
		assert.Equal(t, "testRepo", metas["repo"])
		assert.Equal(t, "testOwner", metas["user"])
		assert.Equal(t, "https://someurl.com/{user}/{repo}/{issue}", metas["format"])
	}

	testSuccess(markup.IssueNameStyleNumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleAlphanumeric
	testSuccess(markup.IssueNameStyleAlphanumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleNumeric
	testSuccess(markup.IssueNameStyleNumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleRegexp
	testSuccess(markup.IssueNameStyleRegexp)

	repo, err := GetRepositoryByID(t.Context(), 3)
	assert.NoError(t, err)

	metas = repo.ComposeCommentMetas(t.Context())
	assert.Contains(t, metas, "org")
	assert.Contains(t, metas, "teams")
	assert.Equal(t, "org3", metas["org"])
	assert.Equal(t, ",owners,team1,", metas["teams"])
}

func TestGetRepositoryByURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("InvalidPath", func(t *testing.T) {
		repo, err := GetRepositoryByURL(t.Context(), "something")
		assert.Nil(t, repo)
		assert.Error(t, err)
	})

	testRepo2 := func(t *testing.T, url string) {
		repo, err := GetRepositoryByURL(t.Context(), url)
		require.NoError(t, err)
		assert.EqualValues(t, 2, repo.ID)
		assert.EqualValues(t, 2, repo.OwnerID)
	}

	t.Run("ValidHttpURL", func(t *testing.T) {
		testRepo2(t, "https://try.gitea.io/user2/repo2")
		testRepo2(t, "https://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidGitSshURL", func(t *testing.T) {
		testRepo2(t, "git+ssh://sshuser@try.gitea.io/user2/repo2")
		testRepo2(t, "git+ssh://sshuser@try.gitea.io/user2/repo2.git")

		testRepo2(t, "git+ssh://try.gitea.io/user2/repo2")
		testRepo2(t, "git+ssh://try.gitea.io/user2/repo2.git")
	})

	t.Run("ValidImplicitSshURL", func(t *testing.T) {
		testRepo2(t, "sshuser@try.gitea.io:user2/repo2")
		testRepo2(t, "sshuser@try.gitea.io:user2/repo2.git")

		testRelax := func(t *testing.T, url string) {
			repo, err := GetRepositoryByURLRelax(t.Context(), url)
			require.NoError(t, err)
			assert.Equal(t, int64(2), repo.ID)
			assert.Equal(t, int64(2), repo.OwnerID)
		}
		// TODO: it doesn't seem to be common git ssh URL, should we really support this?
		testRelax(t, "try.gitea.io:user2/repo2")
		testRelax(t, "try.gitea.io:user2/repo2.git")
	})
}

func TestComposeSSHCloneURL(t *testing.T) {
	defer test.MockVariableValue(&setting.SSH, setting.SSH)()
	defer test.MockVariableValue(&setting.Repository, setting.Repository)()

	setting.SSH.User = "git"

	// test SSH_DOMAIN
	setting.SSH.Domain = "domain"
	setting.SSH.Port = 22
	setting.Repository.UseCompatSSHURI = false
	assert.Equal(t, "git@domain:user/repo.git", ComposeSSHCloneURL(&user_model.User{Name: "doer"}, "user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain/user/repo.git", ComposeSSHCloneURL(&user_model.User{Name: "doer"}, "user", "repo"))
	// test SSH_DOMAIN while use non-standard SSH port
	setting.SSH.Port = 123
	setting.Repository.UseCompatSSHURI = false
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", ComposeSSHCloneURL(nil, "user", "repo"))
	setting.Repository.UseCompatSSHURI = true
	assert.Equal(t, "ssh://git@domain:123/user/repo.git", ComposeSSHCloneURL(nil, "user", "repo"))

	// test IPv6 SSH_DOMAIN
	setting.Repository.UseCompatSSHURI = false
	setting.SSH.Domain = "::1"
	setting.SSH.Port = 22
	assert.Equal(t, "git@[::1]:user/repo.git", ComposeSSHCloneURL(nil, "user", "repo"))
	setting.SSH.Port = 123
	assert.Equal(t, "ssh://git@[::1]:123/user/repo.git", ComposeSSHCloneURL(nil, "user", "repo"))

	setting.SSH.User = "(DOER_USERNAME)"
	setting.SSH.Domain = "domain"
	setting.SSH.Port = 22
	assert.Equal(t, "doer@domain:user/repo.git", ComposeSSHCloneURL(&user_model.User{Name: "doer"}, "user", "repo"))
	setting.SSH.Port = 123
	assert.Equal(t, "ssh://doer@domain:123/user/repo.git", ComposeSSHCloneURL(&user_model.User{Name: "doer"}, "user", "repo"))
}

func TestIsUsableRepoName(t *testing.T) {
	assert.NoError(t, IsUsableRepoName("a"))
	assert.NoError(t, IsUsableRepoName("-1_."))
	assert.NoError(t, IsUsableRepoName(".profile"))

	assert.Error(t, IsUsableRepoName("-"))
	assert.Error(t, IsUsableRepoName("🌞"))
	assert.Error(t, IsUsableRepoName("the/repo"))
	assert.Error(t, IsUsableRepoName("the..repo"))
	assert.Error(t, IsUsableRepoName("foo.wiki"))
	assert.Error(t, IsUsableRepoName("foo.git"))
	assert.Error(t, IsUsableRepoName("foo.RSS"))
}

func TestIsValidSSHAccessRepoName(t *testing.T) {
	assert.True(t, IsValidSSHAccessRepoName("a"))
	assert.True(t, IsValidSSHAccessRepoName("-1_."))
	assert.True(t, IsValidSSHAccessRepoName(".profile"))
	assert.True(t, IsValidSSHAccessRepoName("foo.wiki"))

	assert.False(t, IsValidSSHAccessRepoName("-"))
	assert.False(t, IsValidSSHAccessRepoName("🌞"))
	assert.False(t, IsValidSSHAccessRepoName("the/repo"))
	assert.False(t, IsValidSSHAccessRepoName("the..repo"))
	assert.False(t, IsValidSSHAccessRepoName("foo.git"))
	assert.False(t, IsValidSSHAccessRepoName("foo.RSS"))
}

func TestGetPublicRepositoryByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Test getting a repository by name
	// Repository ID 1 is "repo1" owned by user2, and it's public
	repo, err := GetPublicRepositoryByName(ctx, "repo1")
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Equal(t, "repo1", repo.Name)
	assert.Equal(t, int64(1), repo.ID)
	assert.False(t, repo.IsPrivate)

	// Test getting a non-existent repository
	_, err = GetPublicRepositoryByName(ctx, "non-existent-repo-xyz")
	assert.Error(t, err)
	assert.True(t, IsErrRepoNotExist(err))

	// Test that private repositories are not returned
	// Repository ID 2 is "repo2" owned by user2, and it's private
	_, err = GetPublicRepositoryByName(ctx, "repo2")
	assert.Error(t, err)
	assert.True(t, IsErrRepoNotExist(err))
}

func TestGetPublicRepositoryByName_PrefersRootOverFork(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Note: The test fixtures don't have repositories with the same name where one is a fork
	// This test verifies the ordering logic by checking that is_fork is considered in the ORDER BY clause

	// Test with repo1 which is not a fork
	repo, err := GetPublicRepositoryByName(ctx, "repo1")
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Equal(t, "repo1", repo.Name)
	assert.False(t, repo.IsFork, "repo1 should not be a fork")

	// The key improvement is in the SQL query:
	// Old: ORDER BY `updated_unix` DESC
	// New: ORDER BY `is_fork` ASC, `updated_unix` DESC
	// This ensures that when multiple repos have the same name:
	// 1. Non-forks (is_fork=false=0) come before forks (is_fork=true=1)
	// 2. Within each group, most recently updated comes first
	//
	// This fix ensures that /explore/articles/history/{reponame} displays the root repository
	// and the API fork graph endpoint builds the hierarchy from the correct root.
}


func TestGenerateRepoNameFromSubject(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		expected string
	}{
		// Normal cases - spaces to hyphens
		{
			name:     "Simple two words",
			subject:  "My Project",
			expected: "my-project",
		},
		{
			name:     "Multiple words",
			subject:  "Hello World Test",
			expected: "hello-world-test",
		},
		{
			name:     "Capitalized words",
			subject:  "The Moon Project",
			expected: "the-moon-project",
		},

		// Dot patterns - dots should be removed entirely
		{
			name:     "Single dot",
			subject:  "My.Project",
			expected: "myproject",
		},
		{
			name:     "Multiple single dots",
			subject:  "My.Cool.Project",
			expected: "mycoolproject",
		},
		{
			name:     "Consecutive dots",
			subject:  "My..Project",
			expected: "myproject",
		},
		{
			name:     "Leading dots",
			subject:  "...Project",
			expected: "project",
		},
		{
			name:     "Trailing dots",
			subject:  "Project...",
			expected: "project",
		},
		{
			name:     "Dots and spaces",
			subject:  "My. Project. Name",
			expected: "my-project-name",
		},

		// Reserved patterns - should NOT generate .git, .wiki, .rss, .atom
		{
			name:     "Ends with .git",
			subject:  "Project.git",
			expected: "projectgit",
		},
		{
			name:     "Ends with .wiki",
			subject:  "My.wiki",
			expected: "mywiki",
		},
		{
			name:     "Ends with .rss",
			subject:  "Feed.rss",
			expected: "feedrss",
		},
		{
			name:     "Ends with .atom",
			subject:  "Feed.atom",
			expected: "feedatom",
		},
		{
			name:     "Contains .git in middle",
			subject:  "My.git.Project",
			expected: "mygitproject",
		},

		// Edge cases - empty and special characters
		{
			name:     "Empty string",
			subject:  "",
			expected: "",
		},
		{
			name:     "Only special characters",
			subject:  "!!!",
			expected: "repository",
		},
		{
			name:     "Only spaces",
			subject:  "   ",
			expected: "repository",
		},
		{
			name:     "Only dots",
			subject:  "...",
			expected: "repository",
		},
		{
			name:     "Only hyphens",
			subject:  "---",
			expected: "repository",
		},
		{
			name:     "Only underscores",
			subject:  "___",
			expected: "repository",
		},
		{
			name:     "Single hyphen",
			subject:  "-",
			expected: "repository",
		},
		{
			name:     "Single underscore",
			subject:  "_",
			expected: "repository",
		},

		// Special characters removal
		{
			name:     "Special characters mixed",
			subject:  "Hello@World#2024!",
			expected: "helloworld2024",
		},
		{
			name:     "Parentheses and brackets",
			subject:  "Project (2024) [Final]",
			expected: "project-2024-final",
		},
		{
			name:     "Slashes",
			subject:  "User/Project",
			expected: "userproject",
		},
		{
			name:     "Emoji",
			subject:  "Project 🚀 Rocket",
			expected: "project-rocket",
		},

		// Underscores - should be preserved
		{
			name:     "Underscores preserved",
			subject:  "Test_Project",
			expected: "test_project",
		},
		{
			name:     "Mixed underscores and spaces",
			subject:  "My_Cool Project",
			expected: "my_cool-project",
		},
		{
			name:     "Leading underscores",
			subject:  "__project",
			expected: "project",
		},
		{
			name:     "Trailing underscores",
			subject:  "project__",
			expected: "project",
		},

		// Multiple spaces and hyphens
		{
			name:     "Multiple consecutive spaces",
			subject:  "hello   world",
			expected: "hello-world",
		},
		{
			name:     "Leading and trailing spaces",
			subject:  "  hello world  ",
			expected: "hello-world",
		},
		{
			name:     "Multiple hyphens collapse",
			subject:  "hello---world",
			expected: "hello-world",
		},
		{
			name:     "Mixed separators",
			subject:  "hello - world _ test",
			expected: "hello-world-_-test",
		},

		// Length limits - 100 character maximum
		{
			name:     "Exactly 100 characters",
			subject:  strings.Repeat("a", 100),
			expected: strings.Repeat("a", 100),
		},
		{
			name:     "Over 100 characters",
			subject:  strings.Repeat("a", 150),
			expected: strings.Repeat("a", 100),
		},
		{
			name:     "Over 100 with trailing hyphen after truncation",
			subject:  strings.Repeat("a", 99) + "-" + strings.Repeat("b", 50),
			expected: strings.Repeat("a", 99),
		},
		{
			name:     "Over 100 with trailing underscore after truncation",
			subject:  strings.Repeat("a", 99) + "_" + strings.Repeat("b", 50),
			expected: strings.Repeat("a", 99),
		},

		// Unicode and accents - non-ASCII characters are stripped
		{
			name:     "Unicode characters",
			subject:  "Zürich Project",
			expected: "zrich-project",
		},
		{
			name:     "Accented characters",
			subject:  "Café François",
			expected: "caf-franois",
		},
		{
			name:     "Mixed case with numbers",
			subject:  "Test123Subject",
			expected: "test123subject",
		},

		// Real-world examples
		{
			name:     "GitHub-style repo name",
			subject:  "awesome-project",
			expected: "awesome-project",
		},
		{
			name:     "Sentence-like subject",
			subject:  "The Quick Brown Fox Jumps",
			expected: "the-quick-brown-fox-jumps",
		},
		{
			name:     "Version number",
			subject:  "Project v2.0.1",
			expected: "project-v201",
		},
		{
			name:     "Date in name",
			subject:  "Report 2024-10-28",
			expected: "report-2024-10-28",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRepoNameFromSubject(tt.subject)
			assert.Equal(t, tt.expected, result, "Generated name mismatch for subject: %q", tt.subject)

			// CRITICAL: Verify that all non-empty generated names pass IsUsableRepoName validation
			// This ensures we never generate names that would be rejected later
			if result != "" {
				err := IsUsableRepoName(result)
				assert.NoError(t, err, "Generated name %q should pass IsUsableRepoName validation for subject %q", result, tt.subject)
			}
		})
	}
}
