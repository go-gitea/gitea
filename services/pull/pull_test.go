// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

// TODO TestPullRequest_PushToBaseRepo

func TestPullRequest_CommitMessageTrailersPattern(t *testing.T) {
	// Not a valid trailer section
	assert.False(t, commitMessageTrailersPattern.MatchString(""))
	assert.False(t, commitMessageTrailersPattern.MatchString("No trailer."))
	assert.False(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>\nNot a trailer due to following text."))
	assert.False(t, commitMessageTrailersPattern.MatchString("Message body not correctly separated from trailer section by empty line.\nSigned-off-by: Bob <bob@example.com>"))
	// Valid trailer section
	assert.True(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>\nOther-Trailer: Value"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Message body correctly separated from trailer section by empty line.\n\nSigned-off-by: Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Multiple trailers.\n\nSigned-off-by: Bob <bob@example.com>\nOther-Trailer: Value"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Newline after trailer section.\n\nSigned-off-by: Bob <bob@example.com>\n"))
	assert.True(t, commitMessageTrailersPattern.MatchString("No space after colon is accepted.\n\nSigned-off-by:Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Additional whitespace is accepted.\n\nSigned-off-by \t :  \tBob   <bob@example.com>   "))
	assert.True(t, commitMessageTrailersPattern.MatchString("Folded value.\n\nFolded-trailer: This is\n a folded\n   trailer value\nOther-Trailer: Value"))
}

func TestPullRequest_GetDefaultMergeMessage_InternalTracker(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})

	assert.NoError(t, pr.LoadBaseRepo(db.DefaultContext))
	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	mergeMessage, _, err := GetDefaultMergeMessage(db.DefaultContext, gitRepo, pr, "")
	assert.NoError(t, err)
	assert.Equal(t, "Merge pull request 'issue3' (#3) from branch2 into master", mergeMessage)

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	mergeMessage, _, err = GetDefaultMergeMessage(db.DefaultContext, gitRepo, pr, "")
	assert.NoError(t, err)
	assert.Equal(t, "Merge pull request 'issue3' (#3) from user2/repo1:branch2 into master", mergeMessage)
}

func TestPullRequest_GetDefaultMergeMessage_ExternalTracker(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	externalTracker := repo_model.RepoUnit{
		Type: unit.TypeExternalTracker,
		Config: &repo_model.ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}
	baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	baseRepo.Units = []*repo_model.RepoUnit{&externalTracker}

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2, BaseRepo: baseRepo})

	assert.NoError(t, pr.LoadBaseRepo(db.DefaultContext))
	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	mergeMessage, _, err := GetDefaultMergeMessage(db.DefaultContext, gitRepo, pr, "")
	assert.NoError(t, err)

	assert.Equal(t, "Merge pull request 'issue3' (!3) from branch2 into master", mergeMessage)

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	pr.BaseRepo = nil
	pr.HeadRepo = nil
	mergeMessage, _, err = GetDefaultMergeMessage(db.DefaultContext, gitRepo, pr, "")
	assert.NoError(t, err)

	assert.Equal(t, "Merge pull request 'issue3' (#3) from user2/repo2:branch2 into master", mergeMessage)
}
