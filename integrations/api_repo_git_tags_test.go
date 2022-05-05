// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestAPIGitTags(t *testing.T) {
	defer prepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	// Set up git config for the tagger
	_ = git.NewCommand(git.DefaultContext, "config", "user.name", user.Name).Run(&git.RunOpts{Dir: repo.RepoPath()})
	_ = git.NewCommand(git.DefaultContext, "config", "user.email", user.Email).Run(&git.RunOpts{Dir: repo.RepoPath()})

	gitRepo, _ := git.OpenRepository(git.DefaultContext, repo.RepoPath())
	defer gitRepo.Close()

	commit, _ := gitRepo.GetBranchCommit("master")
	lTagName := "lightweightTag"
	gitRepo.CreateTag(lTagName, commit.ID.String())

	aTagName := "annotatedTag"
	aTagMessage := "my annotated message"
	gitRepo.CreateAnnotatedTag(aTagName, aTagMessage, commit.ID.String())
	aTag, _ := gitRepo.GetTag(aTagName)

	// SHOULD work for annotated tags
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/tags/%s?token=%s", user.Name, repo.Name, aTag.ID.String(), token)
	res := session.MakeRequest(t, req, http.StatusOK)

	var tag *api.AnnotatedTag
	DecodeJSON(t, res, &tag)

	assert.Equal(t, aTagName, tag.Tag)
	assert.Equal(t, aTag.ID.String(), tag.SHA)
	assert.Equal(t, commit.ID.String(), tag.Object.SHA)
	assert.Equal(t, aTagMessage+"\n", tag.Message)
	assert.Equal(t, user.Name, tag.Tagger.Name)
	assert.Equal(t, user.Email, tag.Tagger.Email)
	assert.Equal(t, util.URLJoin(repo.APIURL(), "git/tags", aTag.ID.String()), tag.URL)

	// Should NOT work for lightweight tags
	badReq := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/tags/%s?token=%s", user.Name, repo.Name, commit.ID.String(), token)
	session.MakeRequest(t, badReq, http.StatusBadRequest)
}

func TestAPIDeleteTagByName(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
	session := loginUser(t, owner.LowerName)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/tags/delete-tag?token=%s",
		owner.Name, repo.Name, token)

	req := NewRequestf(t, http.MethodDelete, urlStr)
	_ = session.MakeRequest(t, req, http.StatusNoContent)

	// Make sure that actual releases can't be deleted outright
	createNewReleaseUsingAPI(t, session, token, owner, repo, "release-tag", "", "Release Tag", "test")
	urlStr = fmt.Sprintf("/api/v1/repos/%s/%s/tags/release-tag?token=%s",
		owner.Name, repo.Name, token)

	req = NewRequestf(t, http.MethodDelete, urlStr)
	_ = session.MakeRequest(t, req, http.StatusConflict)
}
