// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/agit"
	gitea_context "code.gitea.io/gitea/services/context"
	pull_service "code.gitea.io/gitea/services/pull"
)

type preReceiveContext struct {
	*gitea_context.PrivateContext

	// loadedPusher indicates that where the following information are loaded
	loadedPusher        bool
	user                *user_model.User // it's the org user if a DeployKey is used
	userPerm            access_model.Permission
	deployKeyAccessMode perm_model.AccessMode

	canCreatePullRequest        bool
	checkedCanCreatePullRequest bool

	canWriteCode        bool
	checkedCanWriteCode bool

	protectedTags    []*git_model.ProtectedTag
	gotProtectedTags bool

	env []string

	opts *private.HookOptions

	branchName string
}

// CanWriteCode returns true if pusher can write code
func (ctx *preReceiveContext) CanWriteCode() bool {
	if !ctx.checkedCanWriteCode {
		if !ctx.loadPusherAndPermission() {
			return false
		}
		ctx.canWriteCode = issues_model.CanMaintainerWriteToBranch(ctx, ctx.userPerm, ctx.branchName, ctx.user) || ctx.deployKeyAccessMode >= perm_model.AccessModeWrite
		ctx.checkedCanWriteCode = true
	}
	return ctx.canWriteCode
}

// AssertCanWriteCode returns true if pusher can write code
func (ctx *preReceiveContext) AssertCanWriteCode() bool {
	if !ctx.CanWriteCode() {
		if ctx.Written() {
			return false
		}
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: "User permission denied for writing.",
		})
		return false
	}
	return true
}

// CanCreatePullRequest returns true if pusher can create pull requests
func (ctx *preReceiveContext) CanCreatePullRequest() bool {
	if !ctx.checkedCanCreatePullRequest {
		if !ctx.loadPusherAndPermission() {
			return false
		}
		ctx.canCreatePullRequest = ctx.userPerm.CanRead(unit.TypePullRequests)
		ctx.checkedCanCreatePullRequest = true
	}
	return ctx.canCreatePullRequest
}

// AssertCreatePullRequest returns true if can create pull requests
func (ctx *preReceiveContext) AssertCreatePullRequest() bool {
	if !ctx.CanCreatePullRequest() {
		if ctx.Written() {
			return false
		}
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: "User permission denied for creating pull-request.",
		})
		return false
	}
	return true
}

// calculateSizeOfObject calculates the size of one git object via git cat-file -s command
func calculateSizeOfObject(ctx *gitea_context.PrivateContext, dir string, env []string, objectID string) (int64, error) {
	objectSizeStr, _, err := gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(objectID).WithDir(dir).WithEnv(env).RunStdString(ctx)
	if err != nil {
		log.Trace("CalculateSizeOfRemovedObjects: Error during git cat-file -s on object: %s", objectID)
		return 0, err
	}

	objectSize, errParse := strconv.ParseInt(strings.TrimSpace(objectSizeStr), 10, 64)
	if errParse != nil {
		log.Trace("CalculateSizeOfRemovedObjects: Error during ParseInt on string '%s'", objectID)
		return 0, errParse
	}
	return objectSize, nil
}

// calculateSizeOfObjectsFromCache calculates the size of objects added and removed from the repository by new push
// it uses data that was cached about the repository for this run
func calculateSizeOfObjectsFromCache(newCommitObjects, oldCommitObjects, otherCommitObjects map[string]bool, commitObjectsSizes map[string]int64) (addedSize, removedSize int64) {
	// Calculate size of objects that were added
	for objectID := range newCommitObjects {
		if _, exists := oldCommitObjects[objectID]; !exists {
			addedSize += commitObjectsSizes[objectID]
		}
	}

	// Calculate size of objects that were removed
	for objectID := range oldCommitObjects {
		if _, exists := newCommitObjects[objectID]; !exists {
			if _, exists := otherCommitObjects[objectID]; !exists {
				removedSize += commitObjectsSizes[objectID]
			}
		}
	}
	return addedSize, removedSize
}

// convertObjectsToMap takes a newline-separated string of git objects and
// converts it into a map for efficient lookup.
func convertObjectsToMap(objects string) map[string]bool {
	objectsMap := make(map[string]bool)
	for object := range strings.SplitSeq(objects, "\n") {
		if len(object) == 0 {
			continue
		}
		objectID := strings.Split(object, " ")[0]
		objectsMap[objectID] = true
	}
	return objectsMap
}

// convertObjectsToSlice converts a list of hashes in a string from the git rev-list --objects command to a slice of string objects
func convertObjectsToSlice(objects string) (objectIDs []string) {
	for object := range strings.SplitSeq(objects, "\n") {
		if len(object) == 0 {
			continue
		}
		objectID := strings.Split(object, " ")[0]
		objectIDs = append(objectIDs, objectID)
	}
	return objectIDs
}

// loadObjectSizesFromPack access all packs that this push or repo has
// and load compressed object size in bytes into objectSizes map
// using `git verify-pack -v` output
func loadObjectSizesFromPack(ctx *gitea_context.PrivateContext, dir string, env, _ []string, objectsSizes map[string]int64) error {
	// Find the path from GIT_QUARANTINE_PATH environment variable (path to the pack file)
	var packPath string
	var errExec error
	for _, envVar := range env {
		split := strings.SplitN(envVar, "=", 2)
		if split[0] == "GIT_QUARANTINE_PATH" {
			packPath = split[1]
			break
		}
	}

	// if no quarantinPath determined we silently ignore
	if packPath == "" {
		log.Trace("GIT_QUARANTINE_PATH not found in the environment variables. Will read the pack files from main repo instead")
		packPath = filepath.Join(ctx.Repo.Repository.RepoPath(), "./objects/")
	}
	log.Trace("packPath: %s", packPath)

	// Find all pack files *.idx in the quarantine directory
	packFiles, err := filepath.Glob(filepath.Join(packPath, "./pack/*.idx"))
	// if pack file not found we silently ignore
	if err != nil {
		log.Trace("Error during finding pack files %s: %v", filepath.Join(packPath, "./pack/*.idx"), err)
	}

	// Loop over each pack file
	i := 0
	for _, packFile := range packFiles {
		log.Trace("Processing packfile %s", packFile)
		// Extract and store in cache objectsSizes the sizes of the object parsing output of the `git verify-pack` command
		output, _, err := gitcmd.NewCommand("verify-pack", "-v").AddDynamicArguments(packFile).WithDir(dir).WithEnv(env).RunStdString(ctx)
		if err != nil {
			log.Trace("Error during git verify-pack on pack file: %s", packFile)
			if errExec == nil {
				errExec = err
			} else {
				errExec = fmt.Errorf("%w; %v", errExec, err)
			}
			continue
		}

		// Parsing the output of the git verify-pack command
		lines := strings.SplitSeq(output, "\n")
		for line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			// Second field has object type
			objectType := fields[1]
			if objectType != "commit" && objectType != "tree" && objectType != "blob" && objectType != "tag" {
				continue
			}

			// First field would have an object hash
			objectID := fields[0]

			// Forth field would have an object compressed size
			size, err := strconv.ParseInt(fields[3], 10, 64)
			if err != nil {
				log.Trace("Failed to parse size for object %s: %v", objectID, err)
				continue
			}
			i++
			objectsSizes[objectID] = size
		}
	}

	log.Trace("Loaded %d items from packfiles", i)
	return errExec
}

// loadObjectsSizesViaCatFile uses hashes from objectIDs and runs `git cat-file -s` in 10 workers to return each object sizes
// Objects for which size is already loaded are skipped
// can't use `git cat-file --batch-check` here as it only provides data from git DB before the commit applied and has no knowledge on new commit objects
func loadObjectsSizesViaCatFile(ctx *gitea_context.PrivateContext, dir string, env, objectIDs []string, objectsSizes map[string]int64) error {
	// This is the number of workers that will simultaneously process CalculateSizeOfObject.
	const numWorkers = 10

	var wg sync.WaitGroup
	var mu sync.Mutex

	// errExec will hold the first error.
	var errOnce sync.Once
	var errExec error

	// errCount will count how many *additional* errors occurred after the first one.
	var errCount int64

	// Prepare numWorkers slices to store the work
	reducedObjectIDs := make([][]string, numWorkers)
	for i := range reducedObjectIDs {
		reducedObjectIDs[i] = make([]string, 0, len(objectIDs)/numWorkers+1)
	}

	// Loop over all objectIDs and find which ones are missing size information
	i := 0
	for _, objectID := range objectIDs {
		_, exists := objectsSizes[objectID]
		if !exists {
			reducedObjectIDs[i%numWorkers] = append(reducedObjectIDs[i%numWorkers], objectID)
			i++
		}
	}

	// Start workers and determine size using `git cat-file -s`, store in objectsSizes cache
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(reducedObjectIDs *[]string) {
			defer wg.Done()
			for _, objectID := range *reducedObjectIDs {
				ctx := ctx
				// Ensure that each worker has its own copy of the env environment to prevent races
				env := append([]string(nil), env...)

				objectSize, err := calculateSizeOfObject(ctx, dir, env, objectID)
				// Upon error we store the first error and continue processing, as we can't stop the push
				// if we were not able to calculate the size of the object, but we keep one error to
				// return at the end, along with a count of subsequent similar errors.
				if err != nil {
					ran := false
					errOnce.Do(func() {
						errExec = err
						ran = true
					})
					if !ran {
						atomic.AddInt64(&errCount, 1)
					}
				}

				mu.Lock() // Protecting shared resource
				objectsSizes[objectID] = objectSize
				mu.Unlock() // Releasing shared resource for other goroutines
			}
		}(&reducedObjectIDs[(w-1)%numWorkers])
	}

	wg.Wait()

	if errExec == nil {
		return nil
	}
	if n := atomic.LoadInt64(&errCount); n > 0 {
		return fmt.Errorf("%w (and %d subsequent similar errors)", errExec, n)
	}
	return errExec
}

// loadObjectsSizesViaBatch uses hashes from objectIDs and uses `git cat-file --batch` command to retrieve object sizes
// This function can't be used for new commit objects.
func loadObjectsSizesViaBatch(ctx *gitea_context.PrivateContext, repoPath string, objectIDs []string, objectsSizes map[string]int64) error {
	reducedObjectIDs := make([]string, 0, len(objectIDs))
	for _, objectID := range objectIDs {
		_, exists := objectsSizes[objectID]
		if !exists {
			reducedObjectIDs = append(reducedObjectIDs, objectID)
		}
	}

	batch, err := git.NewBatch(ctx, repoPath)
	if err != nil {
		log.Error("Unable to create CatFileBatch in %s Error: %v", repoPath, err)
		return fmt.Errorf("Fail to create CatFileBatch: %v", err)
	}
	defer batch.Close()

	for _, objectID := range reducedObjectIDs {
		info, err := batch.QueryInfo(objectID)
		if err != nil {
			log.Trace("Failed to query info for object %s: %v", objectID, err)
			continue
		}
		objectsSizes[objectID] = info.Size
	}

	return nil
}

// parseSize parses the object size from a string
func parseSize(sizeStr string) (int64, error) {
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse object size: %w", err)
	}
	return size, nil
}

/*
LFS pointer scanning (fast-ish, bounded)

We look for pointer blobs (small, <= 4KiB) and parse:

	oid sha256:<64hex>
	size <bytes>

This lets us compute:
- incomingNewToRepoLFS: pointers that are new vs old AND not referenced in "other" parts of repo
- removedLFSSize: pointers removed vs new AND not referenced in "other"
*/
var (
	lfsPointerMarker = []byte("version https://git-lfs.github.com/spec/v1")
	lfsOIDRe         = regexp.MustCompile(`(?m)^oid sha256:([0-9a-f]{64})$`)
	lfsSizeRe        = regexp.MustCompile(`(?m)^size ([0-9]+)$`)
)

func sumLFSSizes(m map[string]int64) int64 {
	var s int64
	for _, v := range m {
		s += v
	}
	return s
}

// scanLFSPointersFromObjectIDs finds LFS pointer blobs among objectIDs and returns map[oid]size.
// It only reads small blobs via cat-file, so it stays bounded.
// scanLFSPointersFromObjectIDs finds LFS pointer blobs among objectIDs and returns map[oid]size.
// It only reads small blobs via cat-file, so it stays bounded.
func scanLFSPointersFromObjectIDs(ctx *gitea_context.PrivateContext, repoPath string, env, objectIDs []string, maxBlobSize int64) (map[string]int64, error) {
	out := make(map[string]int64)
	if len(objectIDs) == 0 {
		return out, nil
	}

	// 1) batch-check: filter small blobs only
	var input bytes.Buffer
	for _, oid := range objectIDs {
		if oid == "" {
			continue
		}
		input.WriteString(oid)
		input.WriteByte('\n')
	}

	// Feed stdin via WithStdin, RunStdBytes takes only context.Context in your version
	checkCmd := gitcmd.NewCommand("cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)").
		WithDir(repoPath).
		WithEnv(env).
		WithStdin(bytes.NewReader(input.Bytes()))

	checkBytes, _, err := checkCmd.RunStdBytes(ctx)
	if err != nil {
		return out, err
	}

	smallBlobs := make([]string, 0, 1024)
	for line := range bytes.SplitSeq(checkBytes, []byte{'\n'}) {
		// "<sha> blob <size>"
		fields := bytes.Fields(line)
		if len(fields) != 3 {
			continue
		}
		if !bytes.Equal(fields[1], []byte("blob")) {
			continue
		}
		size, perr := strconv.ParseInt(string(fields[2]), 10, 64)
		if perr != nil {
			continue
		}
		if size <= maxBlobSize {
			smallBlobs = append(smallBlobs, string(fields[0]))
		}
	}

	if len(smallBlobs) == 0 {
		return out, nil
	}

	// 2) batch: read contents of small blobs, parse LFS pointers
	var input2 bytes.Buffer
	for _, oid := range smallBlobs {
		input2.WriteString(oid)
		input2.WriteByte('\n')
	}

	catCmd := gitcmd.NewCommand("cat-file", "--batch").
		WithDir(repoPath).
		WithEnv(env).
		WithStdin(bytes.NewReader(input2.Bytes()))

	catBytes, _, err := catCmd.RunStdBytes(ctx)
	if err != nil {
		return out, err
	}

	data := catBytes
	i := 0
	for i < len(data) {
		j := bytes.IndexByte(data[i:], '\n')
		if j < 0 {
			break
		}
		j += i
		header := data[i:j]
		i = j + 1

		hf := bytes.Fields(header)
		if len(hf) < 3 {
			break
		}
		blobSize, perr := strconv.ParseInt(string(hf[2]), 10, 64)
		if perr != nil || blobSize < 0 {
			break
		}
		if i+int(blobSize) > len(data) {
			break
		}

		content := data[i : i+int(blobSize)]
		i += int(blobSize)

		if i < len(data) && data[i] == '\n' {
			i++
		}

		if !bytes.Contains(content, lfsPointerMarker) {
			continue
		}

		oidm := lfsOIDRe.FindSubmatch(content)
		if len(oidm) != 2 {
			continue
		}
		sizem := lfsSizeRe.FindSubmatch(content)
		if len(sizem) != 2 {
			continue
		}

		oid := string(oidm[1])
		sz, perr := strconv.ParseInt(string(sizem[1]), 10, 64)
		if perr != nil || sz < 0 {
			continue
		}

		if prev, ok := out[oid]; !ok || sz > prev {
			out[oid] = sz
		}
	}

	return out, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *gitea_context.PrivateContext) {
	startTime := time.Now()
	const maxLFSPointerBlobSize = int64(4096)

	opts := web.GetForm(ctx).(*private.HookOptions)

	ourCtx := &preReceiveContext{
		PrivateContext: ctx,
		env:            generateGitEnv(opts),
		opts:           opts,
	}

	repo := ourCtx.Repo.Repository

	var addedSize int64
	var removedSize int64

	// LFS sizes derived from pointers
	var incomingNewToRepoLFS int64 // best proxy for “incoming LFS objects”
	var removedLFSSize int64
	var addedLFSSize int64 // new-vs-old pointers (can include already-known-to-repo)

	var isRepoOversized bool
	var pushSize *git.CountObject
	var repoSize *git.CountObject
	var err error
	var duration time.Duration

	needGitDelta := repo.ShouldCheckRepoSize()
	needLFSDelta := repo.ShouldCheckLFSSize() || setting.LFSSizeInRepoSize

	// Only do CountObjects (push/repo) when we're doing the repo-size limit at all
	if needGitDelta {
		repoSize, err = git.CountObjects(ctx, repo.RepoPath())
		if err != nil {
			log.Error("Unable to get repository size with env %v: %s Error: %v", repo.RepoPath(), ourCtx.env, err)
			ctx.JSON(http.StatusInternalServerError, map[string]any{
				"err": err.Error(),
			})
			return
		}

		pushSize, err = git.CountObjectsWithEnv(ctx, repo.RepoPath(), ourCtx.env)
		if err != nil {
			log.Error("Unable to get push size with env %v: %s Error: %v", repo.RepoPath(), ourCtx.env, err)
			ctx.JSON(http.StatusInternalServerError, map[string]any{
				"err": err.Error(),
			})
			return
		}

		isRepoOversized = repo.IsRepoSizeOversized(pushSize.Size + pushSize.SizePack)
		log.Trace("Push counts %+v", pushSize)
		log.Trace("Repo counts %+v", repoSize)
	}

	for i := range opts.OldCommitIDs {
		oldCommitID := opts.OldCommitIDs[i]
		newCommitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		log.Trace("Processing old commit: %s, new commit: %s, ref: %s", oldCommitID, newCommitID, refFullName)

		// Deep work is only needed if:
		// - repo is oversized (git deep path), OR
		// - we need LFS delta (LFS limit enabled OR combined mode enabled)
		if isRepoOversized || needLFSDelta {
			var gitObjects string
			var errLoop error
			var errLFS error

			// Keep pointer maps so we can compute delta at the end
			var oldLFSPtrs, otherLFSPtrs, newLFSPtrs map[string]int64

			// Only allocate object-size cache if we'll actually do git delta calc
			var commitObjectsSizes map[string]int64
			if isRepoOversized {
				commitObjectsSizes = make(map[string]int64)
			}

			// OLD commit objects
			if oldCommitID != "0000000000000000000000000000000000000000" {
				gitObjects, _, err = gitcmd.NewCommand("rev-list", "--objects").
					AddDynamicArguments(oldCommitID).
					WithDir(repo.RepoPath()).WithEnv(ourCtx.env).RunStdString(ctx)
				if err != nil {
					log.Error("Unable to list objects in old commit: %s in %-v Error: %v", oldCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to list objects in old commit: %v", err),
					})
					return
				}
			}

			oldCommitObjects := convertObjectsToMap(gitObjects)
			objectIDs := convertObjectsToSlice(gitObjects)

			// LFS pointers for OLD (only if needed)
			oldLFSPtrs = map[string]int64{}
			if needLFSDelta {
				oldLFSPtrs, errLFS = scanLFSPointersFromObjectIDs(ctx, repo.RepoPath(), ourCtx.env, objectIDs, maxLFSPointerBlobSize)
				if errLFS != nil {
					log.Error("Unable to scan old commit LFS pointers for %s in %-v: %v", oldCommitID, repo, errLFS)
					oldLFSPtrs = map[string]int64{}
				} else {
					log.Trace("LFS(old): pointers=%d total=%s", len(oldLFSPtrs), base.FileSize(sumLFSSizes(oldLFSPtrs)))
				}
			}

			// OTHER objects (repo excluding old+new)
			if oldCommitID == "0000000000000000000000000000000000000000" {
				gitObjects, _, err = gitcmd.NewCommand("rev-list", "--objects", "--all").
					AddDynamicArguments("^" + newCommitID).
					WithDir(repo.RepoPath()).WithEnv(ourCtx.env).RunStdString(ctx)
				if err != nil {
					log.Error("Unable to list objects in the repo that are missing from both old %s and new %s commits in %-v Error: %v", oldCommitID, newCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to list objects missing from both old and new commits: %v", err),
					})
					return
				}
			} else {
				gitObjects, _, err = gitcmd.NewCommand("rev-list", "--objects", "--all").
					AddDynamicArguments("^"+oldCommitID, "^"+newCommitID).
					WithDir(repo.RepoPath()).WithEnv(ourCtx.env).RunStdString(ctx)
				if err != nil {
					log.Error("Unable to list objects in the repo that are missing from both old %s and new %s commits in %-v Error: %v", oldCommitID, newCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to list objects missing from both old and new commits: %v", err),
					})
					return
				}
			}

			otherCommitObjects := convertObjectsToMap(gitObjects)
			objectIDs = append(objectIDs, convertObjectsToSlice(gitObjects)...)

			// LFS pointers for OTHER (only if needed)
			otherLFSPtrs = map[string]int64{}
			if needLFSDelta {
				otherLFSPtrs, errLFS = scanLFSPointersFromObjectIDs(ctx, repo.RepoPath(), ourCtx.env, objectIDs, maxLFSPointerBlobSize)
				if errLFS != nil {
					log.Error("Unable to scan other-objects LFS pointers for repo %-v: %v", repo, errLFS)
					otherLFSPtrs = map[string]int64{}
				} else {
					log.Trace("LFS(other): pointers=%d total=%s", len(otherLFSPtrs), base.FileSize(sumLFSSizes(otherLFSPtrs)))
				}
			}

			// Load sizes of OLD+OTHER objects (existing in DB): pack + batch (git deep only)
			if isRepoOversized {
				if repoSize != nil && repoSize.InPack > 0 {
					errLoop = loadObjectSizesFromPack(ctx, repo.RepoPath(), nil, objectIDs, commitObjectsSizes)
					if errLoop != nil {
						log.Error("Unable to get sizes of objects from the pack in %-v Error: %v", repo, errLoop)
					}
				}

				errLoop = loadObjectsSizesViaBatch(ctx, repo.RepoPath(), objectIDs, commitObjectsSizes)
				if errLoop != nil {
					log.Error("Unable to get sizes of objects that are missing in both old %s and new commits %s in %-v Error: %v", oldCommitID, newCommitID, repo, errLoop)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to get sizes of objects missing in both old and new commit and those in old commit: %v", errLoop),
					})
					return
				}
			}

			// NEW commit objects
			gitObjects, _, err = gitcmd.NewCommand("rev-list", "--objects").
				AddDynamicArguments(newCommitID).
				WithDir(repo.RepoPath()).WithEnv(ourCtx.env).RunStdString(ctx)
			if err != nil {
				log.Error("Unable to list objects in new commit %s in %-v Error: %v", newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Fail to list objects in new commit: %v", err),
				})
				return
			}

			newCommitObjects := convertObjectsToMap(gitObjects)
			objectIDs = convertObjectsToSlice(gitObjects)

			// LFS pointers for NEW (only if needed)
			newLFSPtrs = map[string]int64{}
			if needLFSDelta {
				newLFSPtrs, errLFS = scanLFSPointersFromObjectIDs(ctx, repo.RepoPath(), ourCtx.env, objectIDs, maxLFSPointerBlobSize)
				if errLFS != nil {
					log.Error("Unable to scan new commit LFS pointers for %s in %-v: %v", newCommitID, repo, errLFS)
					newLFSPtrs = map[string]int64{}
				} else {
					log.Trace("LFS(new): pointers=%d total=%s", len(newLFSPtrs), base.FileSize(sumLFSSizes(newLFSPtrs)))
				}
			}

			// Load sizes of NEW objects (may be in quarantine packs, etc.) (git deep only)
			if isRepoOversized {
				if pushSize != nil && pushSize.InPack > 0 {
					errLoop = loadObjectSizesFromPack(ctx, repo.RepoPath(), ourCtx.env, objectIDs, commitObjectsSizes)
					if errLoop != nil {
						log.Error("Unable to get sizes of objects from the pack in new commit %s in %-v Error: %v", newCommitID, repo, errLoop)
					}
				}

				errLoop = loadObjectsSizesViaCatFile(ctx, repo.RepoPath(), ourCtx.env, objectIDs, commitObjectsSizes)
				if errLoop != nil {
					log.Error("Unable to get sizes of objects in new commit %s in %-v Error: %v", newCommitID, repo, errLoop)
				}

				// Git object delta (git deep only)
				addedSize, removedSize = calculateSizeOfObjectsFromCache(newCommitObjects, oldCommitObjects, otherCommitObjects, commitObjectsSizes)
			}

			// LFS delta based on pointer presence (LFS deep only)
			if needLFSDelta {
				for oid, sz := range newLFSPtrs {
					if _, inOld := oldLFSPtrs[oid]; !inOld {
						addedLFSSize += sz
						if _, inOther := otherLFSPtrs[oid]; !inOther {
							// Check if the object is already in the database for this repository (e.g. orphan or referenced by hidden ref)
							if _, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, oid); err == nil {
								continue
							}
							incomingNewToRepoLFS += sz
						}
					}
				}

				for oid, sz := range oldLFSPtrs {
					if _, inNew := newLFSPtrs[oid]; inNew {
						continue
					}
					if _, inOther := otherLFSPtrs[oid]; inOther {
						continue
					}
					removedLFSSize += sz
				}

				log.Trace(
					"LFS(delta): incoming-new-to-repo=%s added(vs old)=%s removed=%s current(repo.LFSSize)=%s predicted=%s",
					base.FileSize(incomingNewToRepoLFS),
					base.FileSize(addedLFSSize),
					base.FileSize(removedLFSSize),
					base.FileSize(repo.LFSSize),
					base.FileSize(repo.LFSSize+incomingNewToRepoLFS-removedLFSSize),
				)
			}
		}

		switch {
		case refFullName.IsBranch():
			preReceiveBranch(ourCtx, oldCommitID, newCommitID, refFullName)
		case refFullName.IsTag():
			preReceiveTag(ourCtx, refFullName)
		case git.DefaultFeatures().SupportProcReceive && refFullName.IsFor():
			preReceiveFor(ourCtx, refFullName)
		default:
			ourCtx.AssertCanWriteCode()
		}
		if ctx.Written() {
			return
		}
	}

	// --------- Final accounting + enforcement (one timing) ---------
	duration = time.Since(startTime)

	currentGit := repo.GitSize
	currentLFS := repo.LFSSize
	currentCombined := currentGit + currentLFS

	gitDelta := addedSize - removedSize
	predictedGitAfter := currentGit + gitDelta

	lfsDelta := incomingNewToRepoLFS - removedLFSSize
	predictedLFSAfter := currentLFS + lfsDelta

	predictedCombinedAfter := predictedGitAfter
	if setting.LFSSizeInRepoSize {
		predictedCombinedAfter = predictedGitAfter + predictedLFSAfter
	}

	// Avoid nil panic when repo-size check is disabled but LFS delta is enabled (combined mode / LFS limit)
	pushBytes := int64(0)
	if pushSize != nil {
		pushBytes = maxInt64(0, pushSize.Size+pushSize.SizePack)
	}

	// One summary line (time included here only)
	if repo.ShouldCheckRepoSize() || repo.ShouldCheckLFSSize() {
		log.Warn(
			"SizeCheck summary: took=%s repo=%s/%s git(pred=%s cur=%s delta=%s) lfs(pred=%s cur=%s delta=%s) combined(pred=%s) limits(repo=%s lfs=%s) LFSSizeInRepoSize=%v push=%s",
			duration,
			repo.OwnerName, repo.Name,
			base.FileSize(predictedGitAfter), base.FileSize(currentGit), base.FileSize(gitDelta),
			base.FileSize(predictedLFSAfter), base.FileSize(currentLFS), base.FileSize(lfsDelta),
			base.FileSize(predictedCombinedAfter),
			base.FileSize(repo.GetActualSizeLimit()),
			base.FileSize(repo.GetActualLFSSizeLimit()),
			setting.LFSSizeInRepoSize,
			base.FileSize(pushBytes),
		)
	}

	// 1) LFS-only limit: compare against predicted LFS after push
	if repo.ShouldCheckLFSSize() {
		lfsLimit := repo.GetActualLFSSizeLimit()
		if lfsLimit > 0 && predictedLFSAfter > lfsLimit && predictedLFSAfter > currentLFS {
			log.Warn("Forbidden: LFS limit exceeded: %s > %s for repo %-v",
				base.FileSize(predictedLFSAfter),
				base.FileSize(lfsLimit),
				repo,
			)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("LFS size limit exceeded: %s > then limit of %s",
					base.FileSize(predictedLFSAfter),
					base.FileSize(lfsLimit),
				),
			})
			return
		}
	}

	// 2) Repo (git) size limit when NOT counting LFS into repo size
	if repo.ShouldCheckRepoSize() && !setting.LFSSizeInRepoSize {
		limit := repo.GetActualSizeLimit()
		if limit > 0 && predictedGitAfter > limit && predictedGitAfter > currentGit {
			log.Warn("Forbidden: repository size limit exceeded: %s > %s for repo %-v",
				base.FileSize(predictedGitAfter),
				base.FileSize(limit),
				repo,
			)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("Repository size limit exceeded: %s > then limit of %s",
					base.FileSize(predictedGitAfter),
					base.FileSize(limit),
				),
			})
			return
		}
	}

	// 3) Combined limit when LFS is counted in repo size
	if setting.LFSSizeInRepoSize && repo.ShouldCheckRepoSize() {
		limit := repo.GetActualSizeLimit()
		if limit > 0 && predictedCombinedAfter > limit && predictedCombinedAfter > currentCombined {
			log.Warn("Forbidden: combined repo and LFS size limit exceeded: %s > %s for repo %-v",
				base.FileSize(predictedCombinedAfter),
				base.FileSize(limit),
				repo,
			)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("Combined repository+LFS size limit exceeded: %s > then limit of %s",
					base.FileSize(predictedCombinedAfter),
					base.FileSize(limit),
				),
			})
			return
		}
	}

	ctx.PlainText(http.StatusOK, "ok")
}

func preReceiveBranch(ctx *preReceiveContext, oldCommitID, newCommitID string, refFullName git.RefName) {
	branchName := refFullName.BranchName()
	ctx.branchName = branchName

	if !ctx.AssertCanWriteCode() {
		return
	}

	repo := ctx.Repo.Repository
	gitRepo := ctx.Repo.GitRepo
	objectFormat := ctx.Repo.GetObjectFormat()

	if branchName == repo.DefaultBranch && newCommitID == objectFormat.EmptyObjectID().String() {
		log.Warn("Forbidden: Branch: %s is the default branch in %-v and cannot be deleted", branchName, repo)
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("branch %s is the default branch and cannot be deleted", branchName),
		})
		return
	}

	protectBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, branchName)
	if err != nil {
		log.Error("Unable to get protected branch: %s in %-v Error: %v", branchName, repo, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	if protectBranch == nil {
		return
	}
	protectBranch.Repo = repo

	if newCommitID == objectFormat.EmptyObjectID().String() {
		log.Warn("Forbidden: Branch: %s in %-v is protected from deletion", branchName, repo)
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("branch %s is protected from deletion", branchName),
		})
		return
	}

	isForcePush := false

	if oldCommitID != objectFormat.EmptyObjectID().String() {
		output, _, err := gitrepo.RunCmdString(ctx,
			repo,
			gitcmd.NewCommand("rev-list", "--max-count=1").
				AddDynamicArguments(oldCommitID, "^"+newCommitID).
				WithEnv(ctx.env),
		)
		if err != nil {
			log.Error("Unable to detect force push between: %s and %s in %-v Error: %v", oldCommitID, newCommitID, repo, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Fail to detect force push: %v", err),
			})
			return
		} else if len(output) > 0 {
			if protectBranch.CanForcePush {
				isForcePush = true
			} else {
				log.Warn("Forbidden: Branch: %s in %-v is protected from force push", branchName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: fmt.Sprintf("branch %s is protected from force push", branchName),
				})
				return
			}
		}
	}

	if protectBranch.RequireSignedCommits {
		err := verifyCommits(oldCommitID, newCommitID, gitRepo, ctx.env)
		if err != nil {
			if !isErrUnverifiedCommit(err) {
				log.Error("Unable to check commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Unable to check commits from %s to %s: %v", oldCommitID, newCommitID, err),
				})
				return
			}
			unverifiedCommit := err.(*errUnverifiedCommit).sha
			log.Warn("Forbidden: Branch: %s in %-v is protected from unverified commit %s", branchName, repo, unverifiedCommit)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("branch %s is protected from unverified commit %s", branchName, unverifiedCommit),
			})
			return
		}
	}

	changedProtectedfiles := false
	protectedFilePath := ""

	globs := protectBranch.GetProtectedFilePatterns()
	if len(globs) > 0 {
		_, err := pull_service.CheckFileProtection(gitRepo, branchName, oldCommitID, newCommitID, globs, 1, ctx.env)
		if err != nil {
			if !pull_service.IsErrFilePathProtected(err) {
				log.Error("Unable to check file protection for commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err),
				})
				return
			}

			changedProtectedfiles = true
			protectedFilePath = err.(pull_service.ErrFilePathProtected).Path
		}
	}

	var canPush bool
	if ctx.opts.DeployKeyID != 0 {
		if isForcePush {
			canPush = !changedProtectedfiles && protectBranch.CanPush && (!protectBranch.EnableForcePushAllowlist || protectBranch.ForcePushAllowlistDeployKeys)
		} else {
			canPush = !changedProtectedfiles && protectBranch.CanPush && (!protectBranch.EnableWhitelist || protectBranch.WhitelistDeployKeys)
		}
	} else {
		user, err := user_model.GetUserByID(ctx, ctx.opts.UserID)
		if err != nil {
			log.Error("Unable to GetUserByID for commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to GetUserByID for commits from %s to %s: %v", oldCommitID, newCommitID, err),
			})
			return
		}
		if isForcePush {
			canPush = !changedProtectedfiles && protectBranch.CanUserForcePush(ctx, user)
		} else {
			canPush = !changedProtectedfiles && protectBranch.CanUserPush(ctx, user)
		}
	}

	if !canPush {
		if ctx.opts.PullRequestID == 0 {
			if changedProtectedfiles {
				log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
				})
				return
			}

			globs := protectBranch.GetUnprotectedFilePatterns()
			if len(globs) > 0 {
				unprotectedFilesOnly, err := pull_service.CheckUnprotectedFiles(gitRepo, branchName, oldCommitID, newCommitID, globs, ctx.env)
				if err != nil {
					log.Error("Unable to check file protection for commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err),
					})
					return
				}
				if unprotectedFilesOnly {
					return
				}
			}

			if isForcePush {
				log.Warn("Forbidden: User %d is not allowed to force-push to protected branch: %s in %-v", ctx.opts.UserID, branchName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: "Not allowed to force-push to protected branch " + branchName,
				})
				return
			}
			log.Warn("Forbidden: User %d is not allowed to push to protected branch: %s in %-v", ctx.opts.UserID, branchName, repo)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Not allowed to push to protected branch " + branchName,
			})
			return
		}

		pr, err := issues_model.GetPullRequestByID(ctx, ctx.opts.PullRequestID)
		if err != nil {
			log.Error("Unable to get PullRequest %d Error: %v", ctx.opts.PullRequestID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get PullRequest %d Error: %v", ctx.opts.PullRequestID, err),
			})
			return
		}

		if !ctx.loadPusherAndPermission() {
			return
		}

		allowedMerge, err := pull_service.IsUserAllowedToMerge(ctx, pr, ctx.userPerm, ctx.user)
		if err != nil {
			log.Error("Error calculating if allowed to merge: %v", err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Error calculating if allowed to merge: %v", err),
			})
			return
		}

		if !allowedMerge {
			log.Warn("Forbidden: User %d is not allowed to push to protected branch: %s in %-v and is not allowed to merge pr #%d", ctx.opts.UserID, branchName, repo, pr.Index)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Not allowed to push to protected branch " + branchName,
			})
			return
		}

		if ctx.userPerm.IsAdmin() {
			return
		}

		if changedProtectedfiles {
			log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
			})
			return
		}

		if err := pull_service.CheckPullBranchProtections(ctx, pr, true); err != nil {
			if errors.Is(err, pull_service.ErrNotReadyToMerge) {
				log.Warn("Forbidden: User %d is not allowed push to protected branch %s in %-v and pr #%d is not ready to be merged: %s", ctx.opts.UserID, branchName, repo, pr.Index, err.Error())
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: fmt.Sprintf("Not allowed to push to protected branch %s and pr #%d is not ready to be merged: %s", branchName, ctx.opts.PullRequestID, err.Error()),
				})
				return
			}
			log.Error("Unable to check if mergeable: protected branch %s in %-v and pr #%d. Error: %v", ctx.opts.UserID, branchName, repo, pr.Index, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get status of pull request %d. Error: %v", ctx.opts.PullRequestID, err),
			})
			return
		}
	}
}

func preReceiveTag(ctx *preReceiveContext, refFullName git.RefName) {
	if !ctx.AssertCanWriteCode() {
		return
	}

	tagName := refFullName.TagName()

	if !ctx.gotProtectedTags {
		var err error
		ctx.protectedTags, err = git_model.GetProtectedTags(ctx, ctx.Repo.Repository.ID)
		if err != nil {
			log.Error("Unable to get protected tags for %-v Error: %v", ctx.Repo.Repository, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
			return
		}
		ctx.gotProtectedTags = true
	}

	isAllowed, err := git_model.IsUserAllowedToControlTag(ctx, ctx.protectedTags, tagName, ctx.opts.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}
	if !isAllowed {
		log.Warn("Forbidden: Tag %s in %-v is protected", tagName, ctx.Repo.Repository)
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("Tag %s is protected", tagName),
		})
		return
	}
}

func preReceiveFor(ctx *preReceiveContext, refFullName git.RefName) {
	if !ctx.AssertCreatePullRequest() {
		return
	}

	if ctx.Repo.Repository.IsEmpty {
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: "Can't create pull request for an empty repository.",
		})
		return
	}

	if ctx.opts.IsWiki {
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: "Pull requests are not supported on the wiki.",
		})
		return
	}

	_, _, err := agit.GetAgitBranchInfo(ctx, ctx.Repo.Repository.ID, refFullName.ForBranchName())
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("Unexpected ref: %s", refFullName),
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
		}
	}
}

func generateGitEnv(opts *private.HookOptions) (env []string) {
	env = os.Environ()
	if opts.GitAlternativeObjectDirectories != "" {
		env = append(env, private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
	}
	if opts.GitObjectDirectory != "" {
		env = append(env, private.GitObjectDirectory+"="+opts.GitObjectDirectory)
	}
	if opts.GitQuarantinePath != "" {
		env = append(env, private.GitQuarantinePath+"="+opts.GitQuarantinePath)
	}
	return env
}

// loadPusherAndPermission returns false if an error occurs, and it writes the error response
func (ctx *preReceiveContext) loadPusherAndPermission() bool {
	if ctx.loadedPusher {
		return true
	}

	if ctx.opts.UserID == user_model.ActionsUserID {
		ctx.user = user_model.NewActionsUser()
		ctx.userPerm.AccessMode = perm_model.AccessMode(ctx.opts.ActionPerm)
		if err := ctx.Repo.Repository.LoadUnits(ctx); err != nil {
			log.Error("Unable to get User id %d Error: %v", ctx.opts.UserID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get User id %d Error: %v", ctx.opts.UserID, err),
			})
			return false
		}
		ctx.userPerm.SetUnitsWithDefaultAccessMode(ctx.Repo.Repository.Units, ctx.userPerm.AccessMode)
	} else {
		user, err := user_model.GetUserByID(ctx, ctx.opts.UserID)
		if err != nil {
			log.Error("Unable to get User id %d Error: %v", ctx.opts.UserID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get User id %d Error: %v", ctx.opts.UserID, err),
			})
			return false
		}
		ctx.user = user
		userPerm, err := access_model.GetUserRepoPermission(ctx, ctx.Repo.Repository, user)
		if err != nil {
			log.Error("Unable to get Repo permission of repo %s/%s of User %s: %v", ctx.Repo.Repository.OwnerName, ctx.Repo.Repository.Name, user.Name, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get Repo permission of repo %s/%s of User %s: %v", ctx.Repo.Repository.OwnerName, ctx.Repo.Repository.Name, user.Name, err),
			})
			return false
		}
		ctx.userPerm = userPerm
	}

	if ctx.opts.DeployKeyID != 0 {
		deployKey, err := asymkey_model.GetDeployKeyByID(ctx, ctx.opts.DeployKeyID)
		if err != nil {
			log.Error("Unable to get DeployKey id %d Error: %v", ctx.opts.DeployKeyID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get DeployKey id %d Error: %v", ctx.opts.DeployKeyID, err),
			})
			return false
		}
		ctx.deployKeyAccessMode = deployKey.Mode
	}

	ctx.loadedPusher = true
	return true
}
