// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mcuadros/go-version"
)

// Merge merges pull request to base repository.
// FIXME: add repoWorkingPull make sure two merges does not happen at same time.
func Merge(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository, mergeStyle models.MergeStyle, message string) (err error) {
	if err = pr.GetHeadRepo(); err != nil {
		return fmt.Errorf("GetHeadRepo: %v", err)
	} else if err = pr.GetBaseRepo(); err != nil {
		return fmt.Errorf("GetBaseRepo: %v", err)
	}

	prUnit, err := pr.BaseRepo.GetUnit(models.UnitTypePullRequests)
	if err != nil {
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	if err := pr.CheckUserAllowedToMerge(doer); err != nil {
		return fmt.Errorf("CheckUserAllowedToMerge: %v", err)
	}

	// Check if merge style is correct and allowed
	if !prConfig.IsMergeStyleAllowed(mergeStyle) {
		return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	defer func() {
		go models.AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false)
	}()

	// Clone base repo.
	tmpBasePath, err := models.CreateTemporaryPath("merge")
	if err != nil {
		return err
	}

	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	headRepoPath := pr.HeadRepo.RepoPath()

	if err := git.InitRepository(tmpBasePath, false); err != nil {
		return fmt.Errorf("git init: %v", err)
	}

	remoteRepoName := "head_repo"
	baseBranch := "base"

	// Add head repo remote.
	addCacheRepo := func(staging, cache string) error {
		p := filepath.Join(staging, ".git", "objects", "info", "alternates")
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		data := filepath.Join(cache, "objects")
		if _, err := fmt.Fprintln(f, data); err != nil {
			return err
		}
		return nil
	}

	if err := addCacheRepo(tmpBasePath, baseGitRepo.Path); err != nil {
		return fmt.Errorf("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
	}

	var errbuf strings.Builder
	if err := git.NewCommand("remote", "add", "-t", pr.BaseBranch, "-m", pr.BaseBranch, "origin", baseGitRepo.Path).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git remote add [%s -> %s]: %s", baseGitRepo.Path, tmpBasePath, errbuf.String())
	}

	if err := git.NewCommand("fetch", "origin", "--no-tags", pr.BaseBranch+":"+baseBranch, pr.BaseBranch+":original_"+baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}

	if err := git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git symbolic-ref HEAD base [%s]: %s", tmpBasePath, errbuf.String())
	}

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		return fmt.Errorf("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
	}

	if err := git.NewCommand("remote", "add", remoteRepoName, headRepoPath).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git remote add [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}

	trackingBranch := "tracking"
	// Fetch head branch
	if err := git.NewCommand("fetch", "--no-tags", remoteRepoName, pr.HeadBranch+":"+trackingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}

	stagingBranch := "staging"

	// Enable sparse-checkout
	sparseCheckoutList, err := getDiffTree(tmpBasePath, baseBranch, trackingBranch)
	if err != nil {
		return fmt.Errorf("getDiffTree: %v", err)
	}

	infoPath := filepath.Join(tmpBasePath, ".git", "info")
	if err := os.MkdirAll(infoPath, 0700); err != nil {
		return fmt.Errorf("creating directory failed [%s]: %v", infoPath, err)
	}
	sparseCheckoutListPath := filepath.Join(infoPath, "sparse-checkout")
	if err := ioutil.WriteFile(sparseCheckoutListPath, []byte(sparseCheckoutList), 0600); err != nil {
		return fmt.Errorf("Writing sparse-checkout file to %s: %v", sparseCheckoutListPath, err)
	}

	gitConfigCommand := func() func() *git.Command {
		binVersion, err := git.BinVersion()
		if err != nil {
			log.Fatal("Error retrieving git version: %v", err)
		}

		if version.Compare(binVersion, "1.8.0", ">=") {
			return func() *git.Command {
				return git.NewCommand("config", "--local")
			}
		}
		return func() *git.Command {
			return git.NewCommand("config")
		}
	}()

	// Switch off LFS process (set required, clean and smudge here also)
	if err := gitConfigCommand().AddArguments("filter.lfs.process", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.process -> <> ]: %v", errbuf.String())
	}
	if err := gitConfigCommand().AddArguments("filter.lfs.required", "false").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.required -> <false> ]: %v", errbuf.String())
	}
	if err := gitConfigCommand().AddArguments("filter.lfs.clean", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.clean -> <> ]: %v", errbuf.String())
	}
	if err := gitConfigCommand().AddArguments("filter.lfs.smudge", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.smudge -> <> ]: %v", errbuf.String())
	}

	if err := gitConfigCommand().AddArguments("core.sparseCheckout", "true").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [core.sparsecheckout -> true]: %v", errbuf.String())
	}

	// Read base branch index
	if err := git.NewCommand("read-tree", "HEAD").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git read-tree HEAD: %s", errbuf.String())
	}

	// Merge commits.
	switch mergeStyle {
	case models.MergeStyleMerge:
		if err := git.NewCommand("merge", "--no-ff", "--no-commit", trackingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git merge --no-ff --no-commit [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		}

		sig := doer.NewGitSig()
		if err := git.NewCommand("commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email), "-m", message).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git commit [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		}
	case models.MergeStyleRebase:
		// Checkout head branch
		if err := git.NewCommand("checkout", "-b", stagingBranch, trackingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git checkout: %s", errbuf.String())
		}
		// Rebase before merging
		if err := git.NewCommand("rebase", "-q", baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git rebase [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
		// Checkout base branch again
		if err := git.NewCommand("checkout", baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git checkout: %s", errbuf.String())
		}
		// Merge fast forward
		if err := git.NewCommand("merge", "--ff-only", "-q", stagingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git merge --ff-only [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
	case models.MergeStyleRebaseMerge:
		// Checkout head branch
		if err := git.NewCommand("checkout", "-b", stagingBranch, trackingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git checkout: %s", errbuf.String())
		}
		// Rebase before merging
		if err := git.NewCommand("rebase", "-q", baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git rebase [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
		// Checkout base branch again
		if err := git.NewCommand("checkout", baseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git checkout: %s", errbuf.String())
		}
		// Prepare merge with commit
		if err := git.NewCommand("merge", "--no-ff", "--no-commit", "-q", stagingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git merge --no-ff [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}

		// Set custom message and author and create merge commit
		sig := doer.NewGitSig()
		if err := git.NewCommand("commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email), "-m", message).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git commit [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		}

	case models.MergeStyleSquash:
		// Merge with squash
		if err := git.NewCommand("merge", "-q", "--squash", trackingBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git merge --squash [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
		sig := pr.Issue.Poster.NewGitSig()
		if err := git.NewCommand("commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email), "-m", message).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git commit [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		}
	default:
		return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	// OK we should cache our current head and origin/headbranch
	mergeHeadSHA, err := git.GetFullCommitID(tmpBasePath, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for HEAD: %v", err)
	}
	mergeBaseSHA, err := git.GetFullCommitID(tmpBasePath, "original_"+baseBranch)
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for origin/%s: %v", pr.BaseBranch, err)
	}

	// Now it's questionable about where this should go - either after or before the push
	// I think in the interests of data safety - failures to push to the lfs should prevent
	// the merge as you can always remerge.
	if setting.LFS.StartServer {
		if err := LFSPush(tmpBasePath, mergeHeadSHA, mergeBaseSHA, pr); err != nil {
			return err
		}
	}

	var headUser *models.User
	err = pr.HeadRepo.GetOwner()
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("Can't find user: %d for head repository - %v", pr.HeadRepo.OwnerID, err)
			return err
		}
		log.Error("Can't find user: %d for head repository - defaulting to doer: %s - %v", pr.HeadRepo.OwnerID, doer.Name, err)
		headUser = doer
	} else {
		headUser = pr.HeadRepo.Owner
	}

	env := models.FullPushingEnvironment(
		headUser,
		doer,
		pr.BaseRepo,
		pr.BaseRepo.Name,
		pr.ID,
	)

	// Push back to upstream.
	if err := git.NewCommand("push", "origin", baseBranch+":"+pr.BaseBranch).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git push: %s", errbuf.String())
	}

	pr.MergedCommitID, err = baseGitRepo.GetBranchCommitID(pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommit: %v", err)
	}

	pr.MergedUnix = timeutil.TimeStampNow()
	pr.Merger = doer
	pr.MergerID = doer.ID

	if err = pr.SetMerged(); err != nil {
		log.Error("setMerged [%d]: %v", pr.ID, err)
	}

	if err = models.MergePullRequestAction(doer, pr.Issue.Repo, pr.Issue); err != nil {
		log.Error("MergePullRequestAction [%d]: %v", pr.ID, err)
	}

	// Reset cached commit count
	cache.Remove(pr.Issue.Repo.GetCommitsCountCacheKey(pr.BaseBranch, true))

	// Reload pull request information.
	if err = pr.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return nil
	}

	mode, _ := models.AccessLevel(doer, pr.Issue.Repo)
	if err = models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueClosed,
		Index:       pr.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}

	return nil
}

var escapedSymbols = regexp.MustCompile(`([*[?! \\])`)

func getDiffTree(repoPath, baseBranch, headBranch string) (string, error) {
	getDiffTreeFromBranch := func(repoPath, baseBranch, headBranch string) (string, error) {
		var outbuf, errbuf strings.Builder
		// Compute the diff-tree for sparse-checkout
		if err := git.NewCommand("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "--root", baseBranch, headBranch, "--").RunInDirPipeline(repoPath, &outbuf, &errbuf); err != nil {
			return "", fmt.Errorf("git diff-tree [%s base:%s head:%s]: %s", repoPath, baseBranch, headBranch, errbuf.String())
		}
		return outbuf.String(), nil
	}

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

	list, err := getDiffTreeFromBranch(repoPath, baseBranch, headBranch)
	if err != nil {
		return "", err
	}

	// Prefixing '/' for each entry, otherwise all files with the same name in subdirectories would be matched.
	out := bytes.Buffer{}
	scanner := bufio.NewScanner(strings.NewReader(list))
	scanner.Split(scanNullTerminatedStrings)
	for scanner.Scan() {
		filepath := scanner.Text()
		// escape '*', '?', '[', spaces and '!' prefix
		filepath = escapedSymbols.ReplaceAllString(filepath, `\$1`)
		// no necessary to escape the first '#' symbol because the first symbol is '/'
		fmt.Fprintf(&out, "/%s\n", filepath)
	}

	return out.String(), nil
}
