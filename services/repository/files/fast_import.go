// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
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
	FileSize      int64
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
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
	Signoff      bool
}

type RepoFileOptions struct {
	treePath     string
	fromTreePath string
	executable   bool
}

// UpdateRepoBranch updates the specified branch in the given repository with the provided file changes.
// It uses the fast-import command to perform the update efficiently. So that we can avoid to clone the whole repo.
// TODO: add support for LFS
// TODO: add support for GPG signing
func UpdateRepoBranch(ctx context.Context, doer *user_model.User, repoPath string, opts ChangeRepoFilesOptions) error {
	fPath, cancel, err := generateFastImportFile(doer, opts)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
	}()

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

const commitFileHead = `commit refs/heads/%s
author %s <%s> %d %s
committer %s <%s> %d %s
data %d
%s

%s`

func getReadSeekerSize(r io.ReadSeeker) (int64, error) {
	if file, ok := r.(*os.File); ok {
		stat, err := file.Stat()
		if err != nil {
			return 0, err
		}
		return stat.Size(), nil
	}

	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	return size, nil
}

func getZoneOffsetStr(t time.Time) string {
	// Get the timezone offset in hours and minutes
	_, offset := t.Zone()
	return fmt.Sprintf("%+03d%02d", offset/3600, (offset%3600)/60)
}

func writeCommit(f io.Writer, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "commit refs/heads/%s\n", util.Iif(opts.NewBranch != "", opts.NewBranch, opts.OldBranch))
	return err
}

func writeAuthor(f io.Writer, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "author %s <%s> %d %s\n",
		opts.Author.GitUserName,
		opts.Author.GitUserEmail,
		opts.Dates.Author.Unix(),
		getZoneOffsetStr(opts.Dates.Author))
	return err
}

func writeCommitter(f io.Writer, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	_, err := fmt.Fprintf(f, "committer %s <%s> %d %s\n",
		opts.Committer.GitUserName,
		opts.Committer.GitUserEmail,
		opts.Dates.Committer.Unix(),
		getZoneOffsetStr(opts.Dates.Committer))
	return err
}

func writeMessage(f io.Writer, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
	messageBytes := new(bytes.Buffer)
	if _, err := messageBytes.WriteString(opts.Message); err != nil {
		return err
	}

	committerSig := makeGitUserSignature(doer, opts.Committer, opts.Author)

	if opts.Signoff {
		// Signed-off-by
		_, _ = messageBytes.WriteString("\n")
		_, _ = messageBytes.WriteString("Signed-off-by: ")
		_, _ = messageBytes.WriteString(committerSig.String())
	}
	_, err := fmt.Fprintf(f, "data %d\n%s\n", messageBytes.Len()+1, messageBytes.String())
	return err
}

func writeFrom(f io.Writer, doer *user_model.User, opts *ChangeRepoFilesOptions) error {
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

// generateFastImportFile generates a fast-import file based on the provided options.
func generateFastImportFile(doer *user_model.User, opts ChangeRepoFilesOptions) (fPath string, cancel func(), err error) {
	if opts.OldBranch == "" && opts.NewBranch == "" {
		return "", nil, fmt.Errorf("both old and new branches are empty")
	}
	if opts.OldBranch == opts.NewBranch {
		opts.NewBranch = ""
	}

	writeFuncs := []func(io.Writer, *user_model.User, *ChangeRepoFilesOptions) error{
		writeCommit,
		writeAuthor,
		writeCommitter,
		writeMessage,
		// TODO: add support for Gpg signing
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
		if err := writeFunc(f, doer, &opts); err != nil {
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

			fileMask := "100644"
			if file.Options != nil && file.Options.executable {
				fileMask = "100755"
			}

			if _, err := fmt.Fprintf(f, "M %s inline %s\n", fileMask, file.TreePath); err != nil {
				return "", nil, err
			}
			size := file.FileSize
			if size == 0 {
				size, err = getReadSeekerSize(file.ContentReader)
				if err != nil {
					return "", nil, err
				}
			}
			if _, err := fmt.Fprintf(f, "data %d\n", size+1); err != nil {
				return "", nil, err
			}
			if _, err := io.Copy(f, file.ContentReader); err != nil {
				return "", nil, err
			}
			if _, err := fmt.Fprintln(f); err != nil {
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
