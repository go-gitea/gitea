package files

import (
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/services/contexttest"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUpdateRename(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	repo := ctx.Repo.Repository
	branch := repo.DefaultBranch

	temp, _ := NewTemporaryUploadRepository(repo)
	_ = temp.Clone(ctx, branch, true)
	_ = temp.SetDefaultIndex(ctx)

	filesBeforeRename, _ := temp.LsFiles(ctx, "README.txt", "README.md")
	assert.Equal(t, []string{"README.md", ""}, filesBeforeRename)

	file := &ChangeRepoFile{
		Operation:     "rename",
		FromTreePath:  "README.md",
		TreePath:      "README.txt",
		ContentReader: nil,
		SHA:           "",
		Options: &RepoFileOptions{
			fromTreePath: "README.md",
			treePath:     "README.txt",
			executable:   false,
		},
	}
	contentStore := lfs.NewContentStore()

	err := CreateOrUpdateFile(ctx, temp, file, contentStore, 1, true)
	assert.NoError(t, err)

	filesAfterRename, _ := temp.LsFiles(ctx, "README.txt", "README.md")
	assert.Equal(t, []string{"README.txt", ""}, filesAfterRename)
}
