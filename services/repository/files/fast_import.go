// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/attribute"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"
	"code.gitea.io/gitea/modules/util"
)

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	GitUserName  string // to match "git config user.name"
	GitUserEmail string // to match "git config user.email"
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	Author    time.Time
	Committer time.Time
}

type ChangeRepoFile struct {
	Operation     string // "create", "update", or "delete"
	TreePath      string
	FromTreePath  string
	ContentReader io.ReadSeeker
	SHA           string
	Options       *RepoFileOptions
}

// ChangeRepoFilesOptions holds the repository files update options
type ChangeRepoFilesOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	Message      string
	Files        []*ChangeRepoFile
	Author       *git.Signature
	Committer    *git.Signature
	Signer       *git.Signature
	SignKey      string
	Signoff      bool
}

type RepoFileOptions struct {
	treePath     string
	fromTreePath string
	executable   bool
}

// UpdateRepoBranchWithLFS updates the specified branch in the given repository with the provided file changes.
func UpdateRepoBranchWithLFS(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, opts ChangeRepoFilesOptions) error {
	trustCommitter := repo.GetTrustModel() == repo_model.CommitterTrustModel || repo.GetTrustModel() == repo_model.CollaboratorCommitterTrustModel
	repoPath := repo.RepoPath()
	if !setting.LFS.StartServer {
		return UpdateRepoBranch(ctx, doer, repoPath, trustCommitter, opts)
	}

	// handle lfs files
	fileNames := make([]string, 0, len(opts.Files))
	for _, file := range opts.Files {
		if file.Operation == "create" || file.Operation == "update" {
			fileNames = append(fileNames, file.TreePath)
		}
	}
	if len(fileNames) == 0 {
		return UpdateRepoBranch(ctx, doer, repoPath, trustCommitter, opts)
	}

	attributesMap, err := attribute.CheckAttributes(ctx, gitRepo, "", attribute.CheckAttributeOpts{
		Attributes: []string{attribute.Filter},
		Filenames:  fileNames,
	})
	if err != nil {
		return err
	}

	contentStore := lfs.NewContentStore()

	// Upload the files to LFS Store and replace the content reader with the pointer
	for _, file := range opts.Files {
		if attributesMap[file.TreePath] == nil || !attributesMap[file.TreePath].MatchLFS() {
			continue
		}

		pointer, err := lfs.GeneratePointer(file.ContentReader)
		if err != nil {
			return err
		}
		file.ContentReader.Seek(0, io.SeekStart)

		// upload the file to LFS Store
		exist, err := contentStore.Exists(pointer)
		if err != nil {
			return err
		}
		if !exist {
			// FIXME: Put regenerates the hash and copies the file over.
			// I guess this strictly ensures the soundness of the store but this is inefficient.
			if err := contentStore.Put(pointer, file.ContentReader); err != nil {
				// OK Now we need to cleanup
				// Can't clean up the store, once uploaded there they're there.
				return err
			}

			// add the meta object to the database
			lfsMetaObject, err := git_model.NewLFSMetaObject(ctx, repo.ID, pointer)
			if err != nil {
				// OK Now we need to cleanup
				return err
			}
			defer func() {
				if err != nil {
					if _, err := git_model.RemoveLFSMetaObjectByOid(ctx, repo.ID, lfsMetaObject.Oid); err != nil {
						log.Error("Unable to delete LFS meta object: %v", err)
					}
				}
			}()
		}

		// TODO: should the content reader be closed?
		file.ContentReader = bytes.NewReader([]byte(pointer.StringContent()))
	}

	return UpdateRepoBranch(ctx, doer, repo.RepoPath(), trustCommitter, opts)
}

// UpdateRepoBranch updates the specified branch in the given repository with the provided file changes.
// It uses the fast-import command to perform the update efficiently. So that we can avoid to clone the whole repo.
func UpdateRepoBranch(ctx context.Context, doer *user_model.User, repoPath string, trustCommitter bool, opts ChangeRepoFilesOptions) error {
	fPath, cancel, err := generateFastImportFile(ctx, doer, repoPath, trustCommitter, opts)
	if err != nil {
		return err
	}
	defer cancel()

	f, err := os.Open(fPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, _, err = git.NewCommand("fast-import").
		RunStdString(ctx, &git.RunOpts{
			Stdin: f,
			Dir:   repoPath,
		})
	return err
}

func getZoneOffsetStr(t time.Time) string {
	// Get the timezone offset in hours and minutes
	_, offset := t.Zone()
	return fmt.Sprintf("%+03d%02d", offset/3600, (offset%3600)/60)
}

func writeCommit(ctx context.Context, f io.Writer, _ string, _ bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "commit refs/heads/%s\n", util.Iif(opts.NewBranch != "", opts.NewBranch, opts.OldBranch))
	return err
}

func writeAuthor(ctx context.Context, f io.Writer, _ string, _ bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "author %s <%s> %d %s\n",
		opts.Author.Name,
		opts.Author.Email,
		opts.Author.When.Unix(),
		getZoneOffsetStr(opts.Author.When))
	return err
}

func writeCommitter(ctx context.Context, f io.Writer, _ string, _ bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "committer %s <%s> %d %s\n",
		opts.Committer.Name,
		opts.Committer.Email,
		opts.Committer.When.Unix(),
		getZoneOffsetStr(opts.Committer.When))
	return err
}

func writeMessage(ctx context.Context, f io.Writer, repoPath string, trustCommitter bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	messageBytes := new(bytes.Buffer)
	if _, err := messageBytes.WriteString(opts.Message); err != nil {
		return err
	}

	committerSig := opts.Committer

	if opts.Signer != nil {
		if trustCommitter {
			if opts.Committer.Name != opts.Author.Name || opts.Committer.Email != opts.Author.Email {
				// Add trailers
				_, _ = messageBytes.WriteString("\n")
				_, _ = messageBytes.WriteString("Co-authored-by: ")
				_, _ = messageBytes.WriteString(opts.Author.String())
				_, _ = messageBytes.WriteString("\n")
				_, _ = messageBytes.WriteString("Co-committed-by: ")
				_, _ = messageBytes.WriteString(opts.Committer.String())
				_, _ = messageBytes.WriteString("\n")
			}
		}
		committerSig = opts.Signer
	}

	if opts.Signoff {
		// Signed-off-by
		_, _ = messageBytes.WriteString("\n")
		_, _ = messageBytes.WriteString("Signed-off-by: ")
		_, _ = messageBytes.WriteString(committerSig.String())
	}
	_, err := fmt.Fprintf(f, "data %d\n%s\n", messageBytes.Len()+1, messageBytes.String())
	return err
}

func writeFrom(ctx context.Context, f io.Writer, _ string, _ bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	var fromStatement string
	if opts.LastCommitID != "" && opts.LastCommitID != "HEAD" {
		fromStatement = fmt.Sprintf("from %s\n", opts.LastCommitID)
	} else if opts.OldBranch != "" {
		fromStatement = fmt.Sprintf("from refs/heads/%s^0\n", opts.OldBranch)
	} // if this is a new branch, so we cannot add from refs/heads/newbranch^0

	if len(fromStatement) == 0 {
		return nil
	}
	_, err := fmt.Fprint(f, fromStatement)
	return err
}

func writeGPGSign(ctx context.Context, f io.Writer, _ string, _ bool, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	if opts.SignKey == "" {
		return nil
	}

	// write the GPG signature
	if _, err := fmt.Fprintf(f, "gpgsig %s\n", opts.SignKey); err != nil {
		return err
	}
	return nil
}

// hashObject writes the provided content to the object db and returns its hash
func hashObject(ctx context.Context, repoPath string, content io.Reader) (string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand("hash-object", "-w", "--stdin").
		Run(ctx, &git.RunOpts{
			Dir:    repoPath,
			Stdin:  content,
			Stdout: stdOut,
			Stderr: stdErr,
		}); err != nil {
		log.Error("Unable to hash-object to temporary repo: %s Error: %v\nstdout: %s\nstderr: %s", repoPath, err, stdOut.String(), stdErr.String())
		return "", fmt.Errorf("unable to hash-object to temporary repo: %s Error: %w\nstdout: %s\nstderr: %s", repoPath, err, stdOut.String(), stdErr.String())
	}

	return strings.TrimSpace(stdOut.String()), nil
}

// generateFastImportFile generates a fast-import file based on the provided options.
func generateFastImportFile(ctx context.Context, doer *user_model.User, repoPath string, trustCommitter bool, opts ChangeRepoFilesOptions) (fPath string, cancel func(), err error) {
	if opts.OldBranch == "" && opts.NewBranch == "" {
		return "", nil, fmt.Errorf("both old and new branches are empty")
	}
	if opts.OldBranch == opts.NewBranch {
		opts.NewBranch = ""
	}

	writeFuncs := []func(context.Context, io.Writer, string, bool, *user_model.User, *ChangeRepoFilesOptions) error{
		writeCommit,
		writeAuthor,
		writeCommitter,
		writeGPGSign,
		writeMessage,
		writeFrom,
	}

	f, cancel, err := tempdir.OsTempDir("gitea-fast-import-").CreateTempFileRandom("fast-import-*.txt")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	for _, writeFunc := range writeFuncs {
		if err := writeFunc(ctx, f, repoPath, trustCommitter, doer, &opts); err != nil {
			return "", nil, err
		}
	}

	// Write the file changes to the fast-import file
	for _, file := range opts.Files {
		switch file.Operation {
		case "create", "update":
			// delete the old file if it exists
			if file.FromTreePath != file.TreePath && file.FromTreePath != "" {
				if _, err := fmt.Fprintf(f, "D %s\n", file.FromTreePath); err != nil {
					return "", nil, err
				}
			}

			fileMask := util.Iif(file.Options != nil && file.Options.executable, "100755", "100644")

			// Write the file to objects
			objectHash, err := hashObject(ctx, repoPath, file.ContentReader)
			if err != nil {
				return "", nil, err
			}

			if _, err := fmt.Fprintf(f, "M %s %s %s\n", fileMask, objectHash, file.TreePath); err != nil {
				return "", nil, err
			}
		case "delete":
			if file.FromTreePath == "" {
				return "", nil, fmt.Errorf("delete operation requires FromTreePath")
			}
			if _, err := fmt.Fprintf(f, "D %s\n", file.FromTreePath); err != nil {
				return "", nil, err
			}
		default:
			return "", nil, fmt.Errorf("unknown operation: %s", file.Operation)
		}
	}

	return f.Name(), cancel, nil
}
