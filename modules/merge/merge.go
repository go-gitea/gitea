// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package merge

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
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
		go models.HookQueue.Add(pr.BaseRepo.ID)
		go models.AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false)
	}()

	// Clone base repo.
	tmpBasePath, err := models.CreateTemporaryPath("merge")
	if err != nil {
		return err
	}
	defer models.RemoveTemporaryPath(tmpBasePath)

	headRepoPath := models.RepoPath(pr.HeadUserName, pr.HeadRepo.Name)

	if err := git.Clone(baseGitRepo.Path, tmpBasePath, git.CloneRepoOptions{
		Shared:     true,
		NoCheckout: true,
		Branch:     pr.BaseBranch,
	}); err != nil {
		return fmt.Errorf("git clone: %v", err)
	}

	remoteRepoName := "head_repo"

	// Add head repo remote.
	addCacheRepo := func(staging, cache string) error {
		p := filepath.Join(staging, ".git", "objects", "info", "alternates")
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
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

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		return fmt.Errorf("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
	}

	var errbuf strings.Builder
	if err := git.NewCommand("remote", "add", remoteRepoName, headRepoPath).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git remote add [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}

	// Fetch head branch
	if err := git.NewCommand("fetch", remoteRepoName).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}

	trackingBranch := path.Join(remoteRepoName, pr.HeadBranch)
	stagingBranch := fmt.Sprintf("%s_%s", remoteRepoName, pr.HeadBranch)

	// Enable sparse-checkout
	sparseCheckoutList, err := getDiffTree(tmpBasePath, pr.BaseBranch, trackingBranch)
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

	// Switch off LFS process (set required, clean and smudge here also)
	if err := git.NewCommand("config", "--local", "filter.lfs.process", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.process -> <> ]: %v", errbuf.String())
	}
	if err := git.NewCommand("config", "--local", "filter.lfs.required", "false").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.required -> <false> ]: %v", errbuf.String())
	}
	if err := git.NewCommand("config", "--local", "filter.lfs.clean", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.clean -> <> ]: %v", errbuf.String())
	}
	if err := git.NewCommand("config", "--local", "filter.lfs.smudge", "").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git config [filter.lfs.smudge -> <> ]: %v", errbuf.String())
	}

	if err := git.NewCommand("config", "--local", "core.sparseCheckout", "true").RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
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
		if err := git.NewCommand("rebase", "-q", pr.BaseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git rebase [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
		// Checkout base branch again
		if err := git.NewCommand("checkout", pr.BaseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
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
		if err := git.NewCommand("rebase", "-q", pr.BaseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
			return fmt.Errorf("git rebase [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
		}
		// Checkout base branch again
		if err := git.NewCommand("checkout", pr.BaseBranch).RunInDirPipeline(tmpBasePath, nil, &errbuf); err != nil {
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

	if setting.LFS.StartServer {
		// Now we have to implement git lfs pre-push
		// git rev-list --objects --filter=blob:limit=1k HEAD --not base
		// pass blob shas in to git cat-file --batch-check (possibly unnecessary)
		// ensure only blobs and <=1k size then pass in to git cat-file --batch
		// to read each sha and check each as a pointer
		// Then if they are lfs -> add them to the baseRepo
		revListReader, revListWriter := io.Pipe()
		shasToCheckReader, shasToCheckWriter := io.Pipe()
		catFileCheckReader, catFileCheckWriter := io.Pipe()
		shasToBatchReader, shasToBatchWriter := io.Pipe()
		catFileBatchReader, catFileBatchWriter := io.Pipe()
		errChan := make(chan error, 1)
		wg := sync.WaitGroup{}
		wg.Add(6)
		go func() {
			defer wg.Done()
			defer catFileBatchReader.Close()

			bufferedReader := bufio.NewReader(catFileBatchReader)
			buf := make([]byte, 1025)
			for {
				// File descriptor line
				_, err := bufferedReader.ReadString(' ')
				if err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}
				// Throw away the blob
				if _, err := bufferedReader.ReadString(' '); err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}
				sizeStr, err := bufferedReader.ReadString('\n')
				if err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}
				size, err := strconv.Atoi(sizeStr[:len(sizeStr)-1])
				if err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}
				pointerBuf := buf[:size+1]
				if _, err := io.ReadFull(bufferedReader, pointerBuf); err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}
				pointerBuf = pointerBuf[:size]
				// now we need to check if the pointerBuf is an LFS pointer
				pointer := lfs.IsPointerFile(&pointerBuf)
				if pointer == nil {
					continue
				}
				pointer.RepositoryID = pr.BaseRepoID
				if _, err := models.NewLFSMetaObject(pointer); err != nil {
					catFileBatchReader.CloseWithError(err)
					break
				}

			}
		}()
		go func() {
			defer wg.Done()
			defer shasToBatchReader.Close()
			defer catFileBatchWriter.Close()

			stderr := new(bytes.Buffer)
			if err := git.NewCommand("cat-file", "--batch").RunInDirFullPipeline(tmpBasePath, catFileBatchWriter, stderr, shasToBatchReader); err != nil {
				shasToBatchReader.CloseWithError(fmt.Errorf("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
			}
		}()
		go func() {
			defer wg.Done()
			defer catFileCheckReader.Close()

			scanner := bufio.NewScanner(catFileCheckReader)
			defer shasToBatchWriter.CloseWithError(scanner.Err())
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) == 0 {
					continue
				}
				fields := strings.Split(line, " ")
				if len(fields) < 3 || fields[1] != "blob" {
					continue
				}
				size, _ := strconv.Atoi(string(fields[2]))
				if size > 1024 {
					continue
				}
				toWrite := []byte(fields[0] + "\n")
				for len(toWrite) > 0 {
					n, err := shasToBatchWriter.Write(toWrite)
					if err != nil {
						catFileCheckReader.CloseWithError(err)
						break
					}
					toWrite = toWrite[n:]
				}
			}
		}()
		go func() {
			defer wg.Done()
			defer shasToCheckReader.Close()
			defer catFileCheckWriter.Close()

			stderr := new(bytes.Buffer)
			cmd := git.NewCommand("cat-file", "--batch-check")
			if err := cmd.RunInDirFullPipeline(tmpBasePath, catFileCheckWriter, stderr, shasToCheckReader); err != nil {
				shasToCheckWriter.CloseWithError(fmt.Errorf("git cat-file --batch-check [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
			}
		}()
		go func() {
			defer wg.Done()
			defer revListReader.Close()
			defer shasToCheckWriter.Close()
			scanner := bufio.NewScanner(revListReader)
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) == 0 {
					continue
				}
				fields := strings.Split(line, " ")
				if len(fields) < 2 || len(fields[1]) == 0 {
					continue
				}
				toWrite := []byte(fields[0] + "\n")
				for len(toWrite) > 0 {
					n, err := shasToCheckWriter.Write(toWrite)
					if err != nil {
						revListReader.CloseWithError(err)
						break
					}
					toWrite = toWrite[n:]
				}
			}
			shasToCheckWriter.CloseWithError(scanner.Err())
		}()
		go func() {
			defer wg.Done()
			defer revListWriter.Close()
			stderr := new(bytes.Buffer)
			cmd := git.NewCommand("rev-list", "--objects", "HEAD", "--not", "origin/"+pr.BaseBranch)
			if err := cmd.RunInDirPipeline(tmpBasePath, revListWriter, stderr); err != nil {
				log.Error("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String())
				errChan <- fmt.Errorf("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String())
			}
		}()

		wg.Wait()
		select {
		case err, has := <-errChan:
			if has {
				return err
			}
		default:
		}
	}

	env := models.PushingEnvironment(doer, pr.BaseRepo)

	// Push back to upstream.
	if err := git.NewCommand("push", "origin", pr.BaseBranch).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, nil, &errbuf); err != nil {
		return fmt.Errorf("git push: %s", errbuf.String())
	}

	pr.MergedCommitID, err = baseGitRepo.GetBranchCommitID(pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommit: %v", err)
	}

	pr.MergedUnix = util.TimeStampNow()
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

	l, err := baseGitRepo.CommitsBetweenIDs(pr.MergedCommitID, pr.MergeBase)
	if err != nil {
		log.Error("CommitsBetweenIDs: %v", err)
		return nil
	}

	// It is possible that head branch is not fully sync with base branch for merge commits,
	// so we need to get latest head commit and append merge commit manually
	// to avoid strange diff commits produced.
	mergeCommit, err := baseGitRepo.GetBranchCommit(pr.BaseBranch)
	if err != nil {
		log.Error("GetBranchCommit: %v", err)
		return nil
	}
	if mergeStyle == models.MergeStyleMerge {
		l.PushFront(mergeCommit)
	}

	p := &api.PushPayload{
		Ref:        git.BranchPrefix + pr.BaseBranch,
		Before:     pr.MergeBase,
		After:      mergeCommit.ID.String(),
		CompareURL: setting.AppURL + pr.BaseRepo.ComposeCompareURL(pr.MergeBase, pr.MergedCommitID),
		Commits:    models.ListToPushCommits(l).ToAPIPayloadCommits(pr.BaseRepo.HTMLURL()),
		Repo:       pr.BaseRepo.APIFormat(mode),
		Pusher:     pr.HeadRepo.MustOwner().APIFormat(),
		Sender:     doer.APIFormat(),
	}
	if err = models.PrepareWebhooks(pr.BaseRepo, models.HookEventPush, p); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.BaseRepo.ID)
	}
	return nil
}

func getDiffTree(repoPath, baseBranch, headBranch string) (string, error) {
	getDiffTreeFromBranch := func(repoPath, baseBranch, headBranch string) (string, error) {
		var outbuf, errbuf strings.Builder
		// Compute the diff-tree for sparse-checkout
		// The branch argument must be enclosed with double-quotes ("") in case it contains slashes (e.g "feature/test")
		if err := git.NewCommand("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", baseBranch, headBranch).RunInDirPipeline(repoPath, &outbuf, &errbuf); err != nil {
			return "", fmt.Errorf("git diff-tree [%s base:%s head:%s]: %s", repoPath, baseBranch, headBranch, errbuf.String())
		}
		return outbuf.String(), nil
	}

	list, err := getDiffTreeFromBranch(repoPath, baseBranch, headBranch)
	if err != nil {
		return "", err
	}

	// Prefixing '/' for each entry, otherwise all files with the same name in subdirectories would be matched.
	out := bytes.Buffer{}
	scanner := bufio.NewScanner(strings.NewReader(list))
	for scanner.Scan() {
		fmt.Fprintf(&out, "/%s\n", scanner.Text())
	}
	return out.String(), nil
}
