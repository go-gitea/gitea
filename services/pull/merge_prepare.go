// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

type mergeContext struct {
	*prContext
	doer      *user_model.User
	sig       *git.Signature
	committer *git.Signature
	signKeyID string // empty for no-sign, non-empty to sign
	env       []string
}

func (ctx *mergeContext) RunOpts() *git.RunOpts {
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()
	return &git.RunOpts{
		Env:    ctx.env,
		Dir:    ctx.tmpBasePath,
		Stdout: ctx.outbuf,
		Stderr: ctx.errbuf,
	}
}

// ErrSHADoesNotMatch represents a "SHADoesNotMatch" kind of error.
type ErrSHADoesNotMatch struct {
	Path       string
	GivenSHA   string
	CurrentSHA string
}

// IsErrSHADoesNotMatch checks if an error is a ErrSHADoesNotMatch.
func IsErrSHADoesNotMatch(err error) bool {
	_, ok := err.(ErrSHADoesNotMatch)
	return ok
}

func (err ErrSHADoesNotMatch) Error() string {
	return fmt.Sprintf("sha does not match [given: %s, expected: %s]", err.GivenSHA, err.CurrentSHA)
}

func createTemporaryRepoForMerge(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, expectedHeadCommitID string) (mergeCtx *mergeContext, cancel context.CancelFunc, err error) {
	// Clone base repo.
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		log.Error("createTemporaryRepoForPR: %v", err)
		return nil, cancel, err
	}

	mergeCtx = &mergeContext{
		prContext: prCtx,
		doer:      doer,
	}

	if expectedHeadCommitID != "" {
		trackingCommitID, _, err := git.NewCommand("show-ref", "--hash").AddDynamicArguments(git.BranchPrefix+trackingBranch).RunStdString(ctx, &git.RunOpts{Dir: mergeCtx.tmpBasePath})
		if err != nil {
			defer cancel()
			log.Error("failed to get sha of head branch in %-v: show-ref[%s] --hash refs/heads/tracking: %v", mergeCtx.pr, mergeCtx.tmpBasePath, err)
			return nil, nil, fmt.Errorf("unable to get sha of head branch in %v %w", pr, err)
		}
		if strings.TrimSpace(trackingCommitID) != expectedHeadCommitID {
			defer cancel()
			return nil, nil, ErrSHADoesNotMatch{
				GivenSHA:   expectedHeadCommitID,
				CurrentSHA: trackingCommitID,
			}
		}
	}

	mergeCtx.outbuf.Reset()
	mergeCtx.errbuf.Reset()
	if err := prepareTemporaryRepoForMerge(mergeCtx); err != nil {
		defer cancel()
		return nil, nil, err
	}

	mergeCtx.sig = doer.NewGitSig()
	mergeCtx.committer = mergeCtx.sig

	// Determine if we should sign
	sign, keyID, signer, _ := asymkey_service.SignMerge(ctx, mergeCtx.pr, mergeCtx.doer, mergeCtx.tmpBasePath, "HEAD", trackingBranch)
	if sign {
		mergeCtx.signKeyID = keyID
		if pr.BaseRepo.GetTrustModel() == repo_model.CommitterTrustModel || pr.BaseRepo.GetTrustModel() == repo_model.CollaboratorCommitterTrustModel {
			mergeCtx.committer = signer
		}
	}

	commitTimeStr := time.Now().Format(time.RFC3339)

	// Because this may call hooks we should pass in the environment
	mergeCtx.env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+mergeCtx.sig.Name,
		"GIT_AUTHOR_EMAIL="+mergeCtx.sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+mergeCtx.committer.Name,
		"GIT_COMMITTER_EMAIL="+mergeCtx.committer.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	return mergeCtx, cancel, nil
}

// prepareTemporaryRepoForMerge takes a repository that has been created using createTemporaryRepo
// it then sets up the sparse-checkout and other things
func prepareTemporaryRepoForMerge(ctx *mergeContext) error {
	infoPath := filepath.Join(ctx.tmpBasePath, ".git", "info")
	if err := os.MkdirAll(infoPath, 0o700); err != nil {
		log.Error("%-v Unable to create .git/info in %s: %v", ctx.pr, ctx.tmpBasePath, err)
		return fmt.Errorf("Unable to create .git/info in tmpBasePath: %w", err)
	}

	// Enable sparse-checkout
	// Here we use the .git/info/sparse-checkout file as described in the git documentation
	sparseCheckoutListFile, err := os.OpenFile(filepath.Join(infoPath, "sparse-checkout"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Error("%-v Unable to write .git/info/sparse-checkout file in %s: %v", ctx.pr, ctx.tmpBasePath, err)
		return fmt.Errorf("Unable to write .git/info/sparse-checkout file in tmpBasePath: %w", err)
	}
	defer sparseCheckoutListFile.Close() // we will close it earlier but we need to ensure it is closed if there is an error

	if err := getDiffTree(ctx, ctx.tmpBasePath, baseBranch, trackingBranch, sparseCheckoutListFile); err != nil {
		log.Error("%-v getDiffTree(%s, %s, %s): %v", ctx.pr, ctx.tmpBasePath, baseBranch, trackingBranch, err)
		return fmt.Errorf("getDiffTree: %w", err)
	}

	if err := sparseCheckoutListFile.Close(); err != nil {
		log.Error("%-v Unable to close .git/info/sparse-checkout file in %s: %v", ctx.pr, ctx.tmpBasePath, err)
		return fmt.Errorf("Unable to close .git/info/sparse-checkout file in tmpBasePath: %w", err)
	}

	setConfig := func(key, value string) error {
		if err := git.NewCommand("config", "--local").AddDynamicArguments(key, value).
			Run(ctx, ctx.RunOpts()); err != nil {
			log.Error("git config [%s -> %q]: %v\n%s\n%s", key, value, err, ctx.outbuf.String(), ctx.errbuf.String())
			return fmt.Errorf("git config [%s -> %q]: %w\n%s\n%s", key, value, err, ctx.outbuf.String(), ctx.errbuf.String())
		}
		ctx.outbuf.Reset()
		ctx.errbuf.Reset()

		return nil
	}

	// Switch off LFS process (set required, clean and smudge here also)
	if err := setConfig("filter.lfs.process", ""); err != nil {
		return err
	}

	if err := setConfig("filter.lfs.required", "false"); err != nil {
		return err
	}

	if err := setConfig("filter.lfs.clean", ""); err != nil {
		return err
	}

	if err := setConfig("filter.lfs.smudge", ""); err != nil {
		return err
	}

	if err := setConfig("core.sparseCheckout", "true"); err != nil {
		return err
	}

	// Read base branch index
	if err := git.NewCommand("read-tree", "HEAD").
		Run(ctx, ctx.RunOpts()); err != nil {
		log.Error("git read-tree HEAD: %v\n%s\n%s", err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("Unable to read base branch in to the index: %w\n%s\n%s", err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()

	return nil
}

// getDiffTree returns a string containing all the files that were changed between headBranch and baseBranch
// the filenames are escaped so as to fit the format required for .git/info/sparse-checkout
func getDiffTree(ctx context.Context, repoPath, baseBranch, headBranch string, out io.Writer) error {
	diffOutReader, diffOutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create os.Pipe for %s", repoPath)
		return err
	}
	defer func() {
		_ = diffOutReader.Close()
		_ = diffOutWriter.Close()
	}()

	scanNullTerminatedStrings := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\x00'); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	err = git.NewCommand("diff-tree", "--no-commit-id", "--name-only", "-r", "-r", "-z", "--root").AddDynamicArguments(baseBranch, headBranch).
		Run(ctx, &git.RunOpts{
			Dir:    repoPath,
			Stdout: diffOutWriter,
			PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
				// Close the writer end of the pipe to begin processing
				_ = diffOutWriter.Close()
				defer func() {
					// Close the reader on return to terminate the git command if necessary
					_ = diffOutReader.Close()
				}()

				// Now scan the output from the command
				scanner := bufio.NewScanner(diffOutReader)
				scanner.Split(scanNullTerminatedStrings)
				for scanner.Scan() {
					filepath := scanner.Text()
					// escape '*', '?', '[', spaces and '!' prefix
					filepath = escapedSymbols.ReplaceAllString(filepath, `\$1`)
					// no necessary to escape the first '#' symbol because the first symbol is '/'
					fmt.Fprintf(out, "/%s\n", filepath)
				}
				return scanner.Err()
			},
		})
	return err
}

// ErrRebaseConflicts represents an error if rebase fails with a conflict
type ErrRebaseConflicts struct {
	Style     repo_model.MergeStyle
	CommitSHA string
	StdOut    string
	StdErr    string
	Err       error
}

// IsErrRebaseConflicts checks if an error is a ErrRebaseConflicts.
func IsErrRebaseConflicts(err error) bool {
	_, ok := err.(ErrRebaseConflicts)
	return ok
}

func (err ErrRebaseConflicts) Error() string {
	return fmt.Sprintf("Rebase Error: %v: Whilst Rebasing: %s\n%s\n%s", err.Err, err.CommitSHA, err.StdErr, err.StdOut)
}

// rebaseTrackingOnToBase checks out the tracking branch as staging and rebases it on to the base branch
// if there is a conflict it will return an ErrRebaseConflicts
func rebaseTrackingOnToBase(ctx *mergeContext, mergeStyle repo_model.MergeStyle) error {
	// Checkout head branch
	if err := git.NewCommand("checkout", "-b").AddDynamicArguments(stagingBranch, trackingBranch).
		Run(ctx, ctx.RunOpts()); err != nil {
		return fmt.Errorf("unable to git checkout tracking as staging in temp repo for %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()

	// Rebase before merging
	if err := git.NewCommand("rebase").AddDynamicArguments(baseBranch).
		Run(ctx, ctx.RunOpts()); err != nil {
		// Rebase will leave a REBASE_HEAD file in .git if there is a conflict
		if _, statErr := os.Stat(filepath.Join(ctx.tmpBasePath, ".git", "REBASE_HEAD")); statErr == nil {
			var commitSha string
			ok := false
			failingCommitPaths := []string{
				filepath.Join(ctx.tmpBasePath, ".git", "rebase-apply", "original-commit"), // Git < 2.26
				filepath.Join(ctx.tmpBasePath, ".git", "rebase-merge", "stopped-sha"),     // Git >= 2.26
			}
			for _, failingCommitPath := range failingCommitPaths {
				if _, statErr := os.Stat(failingCommitPath); statErr == nil {
					commitShaBytes, readErr := os.ReadFile(failingCommitPath)
					if readErr != nil {
						// Abandon this attempt to handle the error
						return fmt.Errorf("unable to git rebase staging on to base in temp repo for %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
					}
					commitSha = strings.TrimSpace(string(commitShaBytes))
					ok = true
					break
				}
			}
			if !ok {
				log.Error("Unable to determine failing commit sha for failing rebase in temp repo for %-v. Cannot cast as ErrRebaseConflicts.", ctx.pr)
				return fmt.Errorf("unable to git rebase staging on to base in temp repo for %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			}
			log.Debug("Conflict when rebasing staging on to base in %-v at %s: %v\n%s\n%s", ctx.pr, commitSha, err, ctx.outbuf.String(), ctx.errbuf.String())
			return ErrRebaseConflicts{
				CommitSHA: commitSha,
				Style:     mergeStyle,
				StdOut:    ctx.outbuf.String(),
				StdErr:    ctx.errbuf.String(),
				Err:       err,
			}
		}
		return fmt.Errorf("unable to git rebase staging on to base in temp repo for %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()
	return nil
}
