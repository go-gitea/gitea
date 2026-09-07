// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/lfs"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCopiedContentRequest(t *testing.T) {
	parent := strings.Repeat("b", 40)
	valid := &FinalizeCopiedContentRequest{
		RequestID: uuid.NewString(),
		UserID:    2,
		Destination: CopiedContentDestination{
			RepositoryID: 3,
			Branch:       "master",
			Commit:       strings.Repeat("c", 40),
			Parent:       &parent,
		},
		Files: []CopiedContentFileMapping{{
			SourceRepositoryID: 1,
			SourceCommit:       strings.Repeat("a", 40),
			SourcePath:         "src/a.txt",
			DestinationPath:    "dst/a.txt",
			Representation:     CopiedContentRepresentationGit,
		}},
	}
	require.NoError(t, validateCopiedContentRequest(valid))

	t.Run("TooManyFiles", func(t *testing.T) {
		req := *valid
		req.Files = make([]CopiedContentFileMapping, CopiedContentMaxFiles+1)
		for i := range req.Files {
			req.Files[i] = CopiedContentFileMapping{
				SourceRepositoryID: 1,
				SourceCommit:       strings.Repeat("a", 40),
				SourcePath:         "a.txt",
				DestinationPath:    "f" + strings.Repeat("x", i) + ".txt",
				Representation:     CopiedContentRepresentationGit,
			}
		}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "too_large", copied.Code)
	})

	t.Run("DuplicateDestination", func(t *testing.T) {
		req := *valid
		req.Files = []CopiedContentFileMapping{valid.Files[0], valid.Files[0]}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("AncestorPath", func(t *testing.T) {
		req := *valid
		req.Files = []CopiedContentFileMapping{
			{SourceRepositoryID: 1, SourceCommit: strings.Repeat("a", 40), SourcePath: "a.txt", DestinationPath: "dir", Representation: CopiedContentRepresentationGit},
			{SourceRepositoryID: 1, SourceCommit: strings.Repeat("a", 40), SourcePath: "b.txt", DestinationPath: "dir/b.txt", Representation: CopiedContentRepresentationGit},
		}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		req := *valid
		req.Files[0].DestinationPath = "../escape.txt"
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		req := *valid
		req.RequestID = "not-a-uuid"
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})
}

func TestCopiedRepoPathAndBranch(t *testing.T) {
	assert.True(t, isCopiedRepoPath("dir/file.txt"))
	assert.False(t, isCopiedRepoPath("/abs"))
	assert.False(t, isCopiedRepoPath("foo/../bar"))
	assert.False(t, isCopiedRepoPath("foo\\bar"))
	assert.True(t, isCopiedBranchName("master"))
	assert.True(t, isCopiedBranchName("feature/x"))
	assert.False(t, isCopiedBranchName("refs/heads/master"))
	assert.False(t, isCopiedBranchName(""))
}

func TestCopiedContentModes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	dir := t.TempDir()
	gitcmd := func(args ...string) string {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	gitcmd("init", "-q")
	gitcmd("config", "user.name", "Review")
	gitcmd("config", "user.email", "review@example.invalid")
	payload := "#!/bin/sh\necho copied\n"
	pointer, err := lfs.GeneratePointer(strings.NewReader(payload))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dir+"/source.sh", []byte(payload), 0o755))
	require.NoError(t, os.WriteFile(dir+"/pointer-exec", []byte(pointer.StringContent()), 0o755))
	require.NoError(t, os.WriteFile(dir+"/pointer-plain", []byte(pointer.StringContent()), 0o644))
	for name, content := range map[string]string{"empty": "", "binary": "\x00\xff\x01\x80"} {
		require.NoError(t, os.WriteFile(dir+"/"+name, []byte(content), 0o644))
		pointer, err := lfs.GeneratePointer(strings.NewReader(content))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(dir+"/"+name+"-lfs", []byte(pointer.StringContent()), 0o644))
	}
	require.NoError(t, os.Symlink("source.sh", dir+"/symlink"))
	gitcmd("add", ".")
	gitcmd("commit", "-qm", "mode reproduction")
	sha := gitcmd("rev-parse", "HEAD")
	repo, err := git.OpenRepository(t.Context(), dir)
	require.NoError(t, err)
	defer repo.Close()
	commit, err := repo.GetCommit(sha)
	require.NoError(t, err)
	_, err = git_model.NewLFSMetaObject(t.Context(), 1, pointer)
	require.NoError(t, err)
	sourceModel := &repo_model.Repository{ID: 1, ObjectFormatName: "sha1"}
	destModel := &repo_model.Repository{ID: 2, ObjectFormatName: "sha1"}
	for _, name := range []string{"empty", "binary"} {
		t.Run(name, func(t *testing.T) {
			for _, representation := range []string{CopiedContentRepresentationGit, CopiedContentRepresentationLFS} {
				destination := name
				if representation == CopiedContentRepresentationLFS {
					destination += "-lfs"
				}
				_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit, CopiedContentFileMapping{SourcePath: name, DestinationPath: destination, Representation: representation}, 0)
				require.NoError(t, err)
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit, CopiedContentFileMapping{SourcePath: "symlink", DestinationPath: "symlink", Representation: CopiedContentRepresentationGit}, 0)
		var failure *CopiedContentError
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, http.StatusUnprocessableEntity, failure.Status)
	})
	t.Run("preserved_executable_mode_should_succeed", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit,
			CopiedContentFileMapping{SourcePath: "source.sh", DestinationPath: "pointer-exec", Representation: CopiedContentRepresentationLFS}, 0)
		if err != nil {
			t.Errorf("valid 100755 -> 100755 regular-to-LFS copy rejected: %v", err)
		}
	})
	t.Run("existing_lfs_executable", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit,
			CopiedContentFileMapping{SourcePath: "pointer-exec", DestinationPath: "pointer-exec", Representation: CopiedContentRepresentationLFS}, 0)
		require.NoError(t, err)
	})
	t.Run("existing_lfs_mode_downgrade", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit,
			CopiedContentFileMapping{SourcePath: "pointer-exec", DestinationPath: "pointer-plain", Representation: CopiedContentRepresentationLFS}, 0)
		require.Error(t, err)
	})
	t.Run("git_executable", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit,
			CopiedContentFileMapping{SourcePath: "source.sh", DestinationPath: "source.sh", Representation: CopiedContentRepresentationGit}, 0)
		require.NoError(t, err)
	})
	t.Run("mode_downgrade_should_fail", func(t *testing.T) {
		_, _, err := prepareCopiedFile(t.Context(), destModel, sourceModel, commit, commit,
			CopiedContentFileMapping{SourcePath: "source.sh", DestinationPath: "pointer-plain", Representation: CopiedContentRepresentationLFS}, 0)
		if err == nil {
			t.Error("100755 -> 100644 mode downgrade accepted")
		}
	})
}

func TestCopiedContentActorRevalidation(t *testing.T) {
	for _, state := range []string{"inactive", "prohibited", "deleted", "admin_demoted", "destination_archived"} {
		t.Run(state, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			actorID := int64(2)
			if state == "admin_demoted" {
				actorID = 1
			}
			actor := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: actorID})
			dest := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
			source := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
			sources := map[int64]*repo_model.Repository{source.ID: source}
			require.NoError(t, reverifyCopiedAccess(t.Context(), actor, dest, sources))
			var err error
			engine := db.GetEngine(t.Context())
			switch state {
			case "inactive":
				_, err = engine.ID(actor.ID).Cols("is_active").Update(&user_model.User{IsActive: false})
			case "prohibited":
				_, err = engine.ID(actor.ID).Cols("prohibit_login").Update(&user_model.User{ProhibitLogin: true})
			case "deleted":
				_, err = engine.ID(actor.ID).Delete(new(user_model.User))
			case "admin_demoted":
				_, err = engine.ID(actor.ID).Cols("is_admin").Update(&user_model.User{IsAdmin: false})
			case "destination_archived":
				_, err = engine.ID(dest.ID).Cols("is_archived").Update(&repo_model.Repository{IsArchived: true})
			}
			require.NoError(t, err)
			var failure *CopiedContentError
			require.ErrorAs(t, reverifyCopiedAccess(t.Context(), actor, dest, sources), &failure)
			assert.Equal(t, http.StatusForbidden, failure.Status)
		})
	}
}

func TestCopiedContentAncestorCollision(t *testing.T) {
	if !hasDuplicateOrAncestorPath([]string{"a", "a.txt", "a/b"}) {
		t.Error("ancestor collision hidden by lexically intervening sibling")
	}
}

func TestCopiedContentPublicationProof(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Copy test")
	run("config", "user.email", "copy@example.invalid")
	require.NoError(t, os.WriteFile(dir+"/first", []byte("first"), 0o644))
	run("add", ".")
	run("commit", "-qm", "root")
	rootID := run("rev-parse", "HEAD")
	repo, err := git.OpenRepository(t.Context(), dir)
	require.NoError(t, err)
	defer repo.Close()
	root, err := repo.GetCommit(rootID)
	require.NoError(t, err)
	model := &repo_model.Repository{ObjectFormatName: "sha1"}
	require.NoError(t, requireCommitParent(root, nil, model))
	require.NoError(t, requireExactCreates(repo, root, []CopiedContentFileMapping{{DestinationPath: "first"}}))
	require.Error(t, requireExactCreates(repo, root, []CopiedContentFileMapping{{DestinationPath: "other"}}))
	require.NoError(t, os.WriteFile(dir+"/second", []byte("second"), 0o755))
	run("add", ".")
	run("commit", "-qm", "copy")
	copyID := run("rev-parse", "HEAD")
	copied, err := repo.GetCommit(copyID)
	require.NoError(t, err)
	require.NoError(t, requireCommitParent(copied, &rootID, model))
	require.Error(t, requireCommitParent(copied, nil, model))
	require.NoError(t, requireExactCreates(repo, copied, []CopiedContentFileMapping{{DestinationPath: "second"}}))
	require.Error(t, requireExactCreates(repo, copied, []CopiedContentFileMapping{{DestinationPath: "first"}, {DestinationPath: "second"}}))
	require.NoError(t, os.WriteFile(dir+"/first", []byte("changed"), 0o644))
	run("add", ".")
	run("commit", "-qm", "later update")
	later, err := repo.GetCommit(run("rev-parse", "HEAD"))
	require.NoError(t, err)
	require.Error(t, requireExactCreates(repo, later, []CopiedContentFileMapping{{DestinationPath: "first"}}))
	require.NoError(t, requireCommitOnBranch(repo, "main", copied))
	run("reset", "--hard", rootID)
	var failure *CopiedContentError
	require.ErrorAs(t, requireCommitOnBranch(repo, "main", copied), &failure)
	assert.Equal(t, http.StatusConflict, failure.Status)
	run("checkout", "--detach")
	run("branch", "-D", "main")
	require.ErrorAs(t, requireCommitOnBranch(repo, "main", copied), &failure)
	assert.Equal(t, http.StatusConflict, failure.Status)
	// Repository diff preferences must not hide a changed gitlink from the proof.
	run("update-index", "--add", "--cacheinfo", "160000,"+rootID+",module")
	run("commit", "-qm", "baseline with submodule")
	run("update-index", "--cacheinfo", "160000,"+copyID+",module")
	require.NoError(t, os.WriteFile(dir+"/third", []byte("third"), 0o644))
	run("add", "third")
	run("commit", "-qm", "extra gitlink modification")
	run("config", "diff.ignoreSubmodules", "all")
	changed, err := repo.GetCommit(run("rev-parse", "HEAD"))
	require.NoError(t, err)
	require.Error(t, requireExactCreates(repo, changed, []CopiedContentFileMapping{{DestinationPath: "third"}}))
}

func TestCopiedContentDiffLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	writer := &copiedDiffWriter{remaining: 16, cancel: cancel}
	_, err := writer.Write([]byte(strings.Repeat("x", 17)))
	require.Error(t, err)
	assert.Zero(t, writer.buffer.Len())
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}
