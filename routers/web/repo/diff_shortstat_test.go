// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	gocontext "context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureDiffShortStatRender struct {
	status int
	name   templates.TplName
	data   reqctx.ContextData
}

func (r *captureDiffShortStatRender) TemplateLookup(string, gocontext.Context) (templates.TemplateExecutor, error) {
	return nil, nil //nolint:nilnil // test renderer does not support template lookup
}

func (r *captureDiffShortStatRender) HTML(w io.Writer, status int, name templates.TplName, data any, _ gocontext.Context) error {
	r.status = status
	r.name = name
	r.data = data.(reqctx.ContextData)
	if resp, ok := w.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
	return nil
}

func TestBuildDiffShortStatURL(t *testing.T) {
	diffShortStatURL := buildDiffShortStatURL("/user/repo/diff-shortstat", "base", "head", "detail")

	assert.Equal(t, "/user/repo/diff-shortstat?after=head&before=base&target=detail", diffShortStatURL)
	assert.NotContains(t, diffShortStatURL, "files=")
}

func TestSetDiffShortStatPlaceholderData(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "/")

	setDiffShortStatPlaceholderData(ctx, 3, "/detail", "/tab")

	assert.Equal(t, &gitdiff.DiffShortStat{NumFiles: 3}, ctx.Data["DiffShortStat"])
	assert.Equal(t, "/detail", ctx.Data["DiffShortStatDetailURL"])
	assert.Equal(t, "/tab", ctx.Data["DiffShortStatTabURL"])
	assert.NotContains(t, ctx.Data, "DiffShortStatReady")
}

func TestRenderDiffShortStatComputesAndRenders(t *testing.T) {
	repoStorage, baseCommitID, headCommitID := prepareShortStatRenderRepo(t)

	for _, tc := range []struct {
		target string
		tpl    templates.TplName
	}{
		{"detail", tplDiffShortStatDetail},
		{"tab", tplDiffShortStatTab},
	} {
		t.Run(tc.target, func(t *testing.T) {
			render := &captureDiffShortStatRender{}
			ctx, resp := contexttest.MockContext(t, "/user/repo/diff-shortstat", contexttest.MockContextOption{Render: render})

			renderDiffShortStat(ctx, repoStorage, baseCommitID, headCommitID, tc.target)

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, http.StatusOK, render.status)
			assert.Equal(t, tc.tpl, render.name)
			assert.Equal(t, &gitdiff.DiffShortStat{
				NumFiles:      2,
				TotalAddition: 3,
				TotalDeletion: 1,
			}, render.data["DiffShortStat"])
			assert.NotContains(t, render.data, "DiffShortStatReady")
			assert.NotContains(t, render.data, "DiffShortStatDetailURL")
			assert.NotContains(t, render.data, "DiffShortStatTabURL")
		})
	}
}

func prepareShortStatRenderRepo(t *testing.T) (repoStorage repo_model.StorageRepo, baseCommitID, headCommitID string) {
	t.Helper()

	repoRoot := t.TempDir()
	t.Cleanup(test.MockVariableValue(&setting.RepoRootPath, repoRoot))

	repoRelativePath := "user/repo.git"
	repoDir := filepath.Join(repoRoot, filepath.FromSlash(repoRelativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(repoDir), 0o755))
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.email", "user@example.com").WithDir(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.name", "User").WithDir(repoDir).Run(t.Context()))

	writeFile := func(name, content string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(repoDir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0o644))
	}
	head := func() string {
		t.Helper()
		stdout, _, err := gitcmd.NewCommand("rev-parse", "HEAD").WithDir(repoDir).RunStdString(t.Context())
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}
	commit := func(message string) string {
		t.Helper()
		require.NoError(t, gitcmd.NewCommand("add", "-A").WithDir(repoDir).Run(t.Context()))
		require.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments(message).WithDir(repoDir).Run(t.Context()))
		return head()
	}

	writeFile("file.txt", "old\n")
	baseCommitID = commit("base")

	writeFile("file.txt", "new\nadded\n")
	writeFile("added.txt", "added\n")
	headCommitID = commit("change files")

	return repo_model.StorageRepo(repoRelativePath), baseCommitID, headCommitID
}

func TestIsPullDiffShortStatRangeValidAllowsPRCommitPairsWithoutAncestry(t *testing.T) {
	gitRepo, mergeBaseCommitID, headCommitID, sideCommitID, mainCommitID, outsideCommitID := prepareShortStatRangeTestRepo(t)

	ok, err := isPullDiffShortStatRangeValid(gitRepo, mergeBaseCommitID, headCommitID, sideCommitID, mainCommitID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = isPullDiffShortStatRangeValid(gitRepo, mergeBaseCommitID, headCommitID, outsideCommitID, mainCommitID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func prepareShortStatRangeTestRepo(t *testing.T) (gitRepo *git.Repository, mergeBaseCommitID, headCommitID, sideCommitID, mainCommitID, outsideCommitID string) {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, gitcmd.NewCommand("init").AddDynamicArguments(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.email", "user@example.com").WithDir(repoDir).Run(t.Context()))
	require.NoError(t, gitcmd.NewCommand("config", "user.name", "User").WithDir(repoDir).Run(t.Context()))

	writeFile := func(name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0o644))
	}
	head := func() string {
		t.Helper()
		stdout, _, err := gitcmd.NewCommand("rev-parse", "HEAD").WithDir(repoDir).RunStdString(t.Context())
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}
	commit := func(message string) string {
		t.Helper()
		require.NoError(t, gitcmd.NewCommand("add", "-A").WithDir(repoDir).Run(t.Context()))
		require.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments(message).WithDir(repoDir).Run(t.Context()))
		return head()
	}

	writeFile("base.txt", "base\n")
	mergeBaseCommitID = commit("base")

	stdout, _, runErr := gitcmd.NewCommand("branch", "--show-current").WithDir(repoDir).RunStdString(t.Context())
	require.NoError(t, runErr)
	mainBranch := strings.TrimSpace(stdout)

	require.NoError(t, gitcmd.NewCommand("checkout", "-b", "side").WithDir(repoDir).Run(t.Context()))
	writeFile("side.txt", "side\n")
	sideCommitID = commit("side")

	require.NoError(t, gitcmd.NewCommand("checkout").AddDynamicArguments(mainBranch).WithDir(repoDir).Run(t.Context()))
	writeFile("main.txt", "main\n")
	mainCommitID = commit("main")

	require.NoError(t, gitcmd.NewCommand("merge", "--no-ff", "side", "-m", "merge side").WithDir(repoDir).Run(t.Context()))
	headCommitID = head()

	require.NoError(t, gitcmd.NewCommand("checkout", "-b", "outside").WithDir(repoDir).Run(t.Context()))
	writeFile("outside.txt", "outside\n")
	outsideCommitID = commit("outside")

	var err error
	gitRepo, err = git.OpenRepository(t.Context(), repoDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = gitRepo.Close()
	})

	return gitRepo, mergeBaseCommitID, headCommitID, sideCommitID, mainCommitID, outsideCommitID
}
