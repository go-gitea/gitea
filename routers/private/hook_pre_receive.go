// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/web"
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
func calculateSizeOfObject(ctx *gitea_context.PrivateContext, opts *git.RunOpts, objectID string) (objectSize int64) {
	objectSizeStr, _, err := git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(objectID).RunStdString(opts)
	if err != nil {
		log.Trace("CalculateSizeOfRemovedObjects: Error during git cat-file -s on object: %s", objectID)
		return objectSize
	}

	objectSize, _ = strconv.ParseInt(strings.TrimSpace(objectSizeStr), 10, 64)
	if err != nil {
		log.Trace("CalculateSizeOfRemovedObjects: Error during ParseInt on string '%s'", objectID)
		return objectSize
	}
	return objectSize
}

// calculateSizeOfObjectsFromCache calculates the size of objects added and removed from the repository by new push
// it uses data that was cached about the repository for this run
func calculateSizeOfObjectsFromCache(newCommitObjects, oldCommitObjects, otherCommitObjects map[string]bool, commitObjectsSizes map[string]int64) (addedSize, removedSize int64) {
	// Calculate size of objects that were added
	for objectID := range newCommitObjects {
		if _, exists := oldCommitObjects[objectID]; !exists {
			// objectID is not referenced in the list of objects of old commit so it is a new object
			// Calculate its size and add it to the addedSize
			addedSize += commitObjectsSizes[objectID]
		}
		// We might check here if new object is not already in the rest of repo to be precise
		// However our goal is to prevent growth of repository so on determination of addedSize
		// We can skip this preciseness, addedSize will be more then real addedSize
		// TODO - do not count size of object that is referenced in other part of repo but not referenced neither in old nor new commit
		//        git will not add the object twice
	}

	// Calculate size of objects that were removed
	for objectID := range oldCommitObjects {
		if _, exists := newCommitObjects[objectID]; !exists {
			// objectID is not referenced in the list of new commit objects so it was possibly removed
			if _, exists := otherCommitObjects[objectID]; !exists {
				// objectID is not referenced in rest of the objects of the repository so it was removed
				// Calculate its size and add it to the removedSize
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
	for _, object := range strings.Split(objects, "\n") {
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
	for _, object := range strings.Split(objects, "\n") {
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
func loadObjectSizesFromPack(ctx *gitea_context.PrivateContext, opts *git.RunOpts, objectIDs []string, objectsSizes map[string]int64) error {
	// Find the path from GIT_QUARANTINE_PATH environment variable (path to the pack file)
	var packPath string
	for _, envVar := range opts.Env {
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
	log.Warn("packPath: %s", packPath)

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
		output, _, err := git.NewCommand(ctx, "verify-pack", "-v").AddDynamicArguments(packFile).RunStdString(opts)
		if err != nil {
			log.Trace("Error during git verify-pack on pack file: %s", packFile)
			continue
		}

		// Parsing the output of the git verify-pack command
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			// Second field has object type
			// If object type is not known filter it out and do not process
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
	return nil
}

// loadObjectsSizesViaCatFile uses hashes from objectIDs and runs `git cat-file -s` in 10 workers to return each object sizes
// Objects for which size is already loaded are skipped
// can't use `git cat-file --batch-check` here as it only provides data from git DB before the commit applied and has no knowledge on new commit objects
func loadObjectsSizesViaCatFile(ctx *gitea_context.PrivateContext, opts *git.RunOpts, objectIDs []string, objectsSizes map[string]int64) error {
	// This is the number of workers that will simultaneously process CalculateSizeOfObject.
	const numWorkers = 10

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Prepare numWorker slices to store the work
	reducedObjectIDs := make([][]string, numWorkers)
	for i := 0; i < numWorkers; i++ {
		reducedObjectIDs[i] = make([]string, 0, len(objectIDs)/numWorkers+1)
	}

	// Loop over all objectIDs and find which ones are missing size information
	i := 0
	for _, objectID := range objectIDs {
		_, exists := objectsSizes[objectID]

		// If object doesn't yet have size in objectsSizes add it for further processing
		if !exists {
			reducedObjectIDs[i%numWorkers] = append(reducedObjectIDs[i%numWorkers], objectID)
			i++
		}
	}

	// Start workers and determine size using `git cat-file -s` store in objectsSizes cache
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(reducedObjectIDs *[]string) {
			defer wg.Done()
			for _, objectID := range *reducedObjectIDs {
				ctx := ctx
				// Create a copy of opts to allow change of the Env property
				tsopts := *opts
				// Ensure that each worker has its own copy of the Env environment to prevent races
				tsopts.Env = append([]string(nil), opts.Env...)
				objectSize := calculateSizeOfObject(ctx, &tsopts, objectID)
				mu.Lock() // Protecting shared resource
				objectsSizes[objectID] = objectSize
				mu.Unlock() // Releasing shared resource for other goroutines
			}
		}(&reducedObjectIDs[(w-1)%numWorkers])
	}

	// Wait for all workers to finish processing.
	wg.Wait()

	return nil
}

// loadObjectsSizesViaBatch uses hashes from objectIDs and uses pre-opened `git cat-file --batch-check` command to slice and return each object sizes
// This function can't be used for new commit objects.
// It speeds up loading object sizes from existing git database of the repository avoiding
// multiple `git cat-files -s`
func loadObjectsSizesViaBatch(ctx *gitea_context.PrivateContext, repoPath string, objectIDs []string, objectsSizes map[string]int64) error {
	var i int32

	reducedObjectIDs := make([]string, 0, len(objectIDs))

	// Loop over all objectIDs and find which ones are missing size information
	for _, objectID := range objectIDs {
		_, exists := objectsSizes[objectID]

		// If object doesn't yet have size in objectsSizes add it for further processing
		if !exists {
			reducedObjectIDs = append(reducedObjectIDs, objectID)
		}
	}

	wr, rd, cancel := git.CatFileBatchCheck(ctx, repoPath)
	defer cancel()

	for _, commitID := range reducedObjectIDs {
		_, err := wr.Write([]byte(commitID + "\n"))
		if err != nil {
			return err
		}
		i++
		line, err := rd.ReadString('\n')
		if err != nil {
			return err
		}
		if len(line) == 1 {
			line, err = rd.ReadString('\n')
			if err != nil {
				return err
			}
		}
		fields := strings.Fields(line)
		objectID := fields[0]
		if len(fields) < 3 || len(fields) > 3 {
			log.Trace("String '%s' does not contain size ignored %s: %v", line, objectID, err)
			continue
		}
		sizeStr := fields[2]
		size, err := parseSize(sizeStr)
		if err != nil {
			log.Trace("String '%s' Failed to parse size for object %s: %v", line, objectID, err)
			continue
		}
		objectsSizes[objectID] = size
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

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *gitea_context.PrivateContext) {
	startTime := time.Now()

	opts := web.GetForm(ctx).(*private.HookOptions)

	ourCtx := &preReceiveContext{
		PrivateContext: ctx,
		env:            generateGitEnv(opts), // Generate git environment for checking commits
		opts:           opts,
	}

	repo := ourCtx.Repo.Repository

	var addedSize int64
	var removedSize int64
	var isRepoOversized bool
	var pushSize *git.CountObject
	var repoSize *git.CountObject
	var err error
	var duration time.Duration

	if repo.IsRepoSizeLimitEnabled() {

		// Calculating total size of the repo using `git count-objects`
		repoSize, err = git.CountObjects(ctx, repo.RepoPath())
		if err != nil {
			log.Error("Unable to get repository size with env %v: %s Error: %v", repo.RepoPath(), ourCtx.env, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": err.Error(),
			})
			return
		}

		// Calculating total size of the push using `git count-objects`
		pushSize, err = git.CountObjectsWithEnv(ctx, repo.RepoPath(), ourCtx.env)
		if err != nil {
			log.Error("Unable to get push size with env %v: %s Error: %v", repo.RepoPath(), ourCtx.env, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": err.Error(),
			})
			return
		}

		// Cache whether the repository would breach the size limit after the operation
		isRepoOversized = repo.IsRepoSizeOversized(pushSize.Size + pushSize.SizePack)
		log.Warn("Push counts %+v", pushSize)
		log.Warn("Repo counts %+v", repoSize)
	}

	// Iterate across the provided old commit IDs
	for i := range opts.OldCommitIDs {
		oldCommitID := opts.OldCommitIDs[i]
		newCommitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		log.Trace("Processing old commit: %s, new commit: %s, ref: %s", oldCommitID, newCommitID, refFullName)

		// If operation is in potential breach of size limit prepare data for analysis
		if isRepoOversized {
			var gitObjects string
			var error error

			// Create cache of objects in old commit
			// if oldCommitID all 0 then it's a fresh repository on gitea server and all git operations on such oldCommitID would fail
			if oldCommitID != "0000000000000000000000000000000000000000" {
				gitObjects, _, err = git.NewCommand(ctx, "rev-list", "--objects").AddDynamicArguments(oldCommitID).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env})
				if err != nil {
					log.Error("Unable to list objects in old commit: %s in %-v Error: %v", oldCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to list objects in old commit: %v", err),
					})
					return
				}
			}

			commitObjectsSizes := make(map[string]int64)
			oldCommitObjects := convertObjectsToMap(gitObjects)
			objectIDs := convertObjectsToSlice(gitObjects)

			// Create cache of objects that are in the repository but not part of old or new commit
			// if oldCommitID all 0 then it's a fresh repository on gitea server and all git operations on such oldCommitID would fail
			if oldCommitID == "0000000000000000000000000000000000000000" {
				gitObjects, _, err = git.NewCommand(ctx, "rev-list", "--objects", "--all").AddDynamicArguments("^" + newCommitID).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env})
				if err != nil {
					log.Error("Unable to list objects in the repo that are missing from both old %s and new %s commits in %-v Error: %v", oldCommitID, newCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to list objects missing from both old and new commits: %v", err),
					})
					return
				}
			} else {
				gitObjects, _, err = git.NewCommand(ctx, "rev-list", "--objects", "--all").AddDynamicArguments("^"+oldCommitID, "^"+newCommitID).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env})
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
			// Unfortunately `git cat-file --check-batch` shows full object size
			// so we would load compressed sizes from pack file via `git verify-pack -v` if there are pack files in repo
			// The result would still miss items that are loose as individual objects (not part of pack files)
			if repoSize.InPack > 0 {
				error = loadObjectSizesFromPack(ctx, &git.RunOpts{Dir: repo.RepoPath(), Env: nil}, objectIDs, commitObjectsSizes)
				if error != nil {
					log.Error("Unable to get sizes of objects from the pack in %-v Error: %v", repo, error)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to get sizes of objects in repo: %v", err),
					})
					return
				}
			}

			// Load loose objects that are missing
			error = loadObjectsSizesViaBatch(ctx, repo.RepoPath(), objectIDs, commitObjectsSizes)
			if error != nil {
				log.Error("Unable to get sizes of objects that are missing in both old %s and new commits %s in %-v Error: %v", oldCommitID, newCommitID, repo, error)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Fail to get sizes of objects missing in both old and new commit and those in old commit: %v", err),
				})
				return
			}

			// Create cache of objects in new commit
			gitObjects, _, err = git.NewCommand(ctx, "rev-list", "--objects").AddDynamicArguments(newCommitID).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env})
			if err != nil {
				log.Error("Unable to list objects in new commit %s in %-v Error: %v", newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Fail to list objects in new commit: %v", err),
				})
				return
			}

			newCommitObjects := convertObjectsToMap(gitObjects)
			objectIDs = convertObjectsToSlice(gitObjects)
			// Unfortunately `git cat-file --check-batch` doesn't work on objects not yet accepted into git database
			// so the sizes will be calculated through pack file `git verify-pack -v` if there are pack files
			// The result would still miss items that were sent loose as individual objects (not part of pack files)
			if pushSize.InPack > 0 {
				error = loadObjectSizesFromPack(ctx, &git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env}, objectIDs, commitObjectsSizes)
				if error != nil {
					log.Error("Unable to get sizes of objects from the pack in new commit %s in %-v Error: %v", newCommitID, repo, error)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to get sizes of objects in new commit: %v", err),
					})
					return
				}
			}

			// After loading everything we could from pack file, objects could have been sent as loose bunch as well
			// We need to load them individually with `git cat-file -s` on any object that is missing from accumulated size cache commitObjectsSizes
			error = loadObjectsSizesViaCatFile(ctx, &git.RunOpts{Dir: repo.RepoPath(), Env: ourCtx.env}, objectIDs, commitObjectsSizes)
			if error != nil {
				log.Error("Unable to get sizes of objects in new commit %s in %-v Error: %v", newCommitID, repo, error)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Fail to get sizes of objects in new commit: %v", err),
				})
				return
			}

			// Calculate size that was added and removed by the new commit
			addedSize, removedSize = calculateSizeOfObjectsFromCache(newCommitObjects, oldCommitObjects, otherCommitObjects, commitObjectsSizes)
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

	if repo.IsRepoSizeLimitEnabled() {
		duration = time.Since(startTime)
		log.Warn("During size checking - Addition in size is: %d, removal in size is: %d, limit size: %d, push size: %d, repo size: %d. Took %s seconds.", addedSize, removedSize, repo.GetActualSizeLimit(), pushSize.Size+pushSize.SizePack, repo.GitSize, duration)
	}

	// If total of commits add more size then they remove and we are in a potential breach of size limit -- abort
	if (addedSize > removedSize) && isRepoOversized {
		log.Warn("Forbidden: new repo size %s would be over limitation of %s. Push size: %s. Took %s seconds. addedSize: %s. removedSize: %s", base.FileSize(repo.GitSize+addedSize-removedSize), base.FileSize(repo.GetActualSizeLimit()), base.FileSize(pushSize.Size+pushSize.SizePack), duration, base.FileSize(addedSize), base.FileSize(removedSize))
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("New repository size is over limitation of %s", base.FileSize(repo.GetActualSizeLimit())),
		})
		return
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

	// Allow pushes to non-protected branches
	if protectBranch == nil {
		return
	}
	protectBranch.Repo = repo

	// This ref is a protected branch.
	//
	// First of all we need to enforce absolutely:
	//
	// 1. Detect and prevent deletion of the branch
	if newCommitID == objectFormat.EmptyObjectID().String() {
		log.Warn("Forbidden: Branch: %s in %-v is protected from deletion", branchName, repo)
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("branch %s is protected from deletion", branchName),
		})
		return
	}

	isForcePush := false

	// 2. Disallow force pushes to protected branches
	if oldCommitID != objectFormat.EmptyObjectID().String() {
		output, _, err := git.NewCommand(ctx, "rev-list", "--max-count=1").AddDynamicArguments(oldCommitID, "^"+newCommitID).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: ctx.env})
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

	// 3. Enforce require signed commits
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

	// Now there are several tests which can be overridden:
	//
	// 4. Check protected file patterns - this is overridable from the UI
	changedProtectedfiles := false
	protectedFilePath := ""

	globs := protectBranch.GetProtectedFilePatterns()
	if len(globs) > 0 {
		_, err := pull_service.CheckFileProtection(gitRepo, branchName, oldCommitID, newCommitID, globs, 1, ctx.env)
		if err != nil {
			if !models.IsErrFilePathProtected(err) {
				log.Error("Unable to check file protection for commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err),
				})
				return
			}

			changedProtectedfiles = true
			protectedFilePath = err.(models.ErrFilePathProtected).Path
		}
	}

	// 5. Check if the doer is allowed to push (and force-push if the incoming push is a force-push)
	var canPush bool
	if ctx.opts.DeployKeyID != 0 {
		// This flag is only ever true if protectBranch.CanForcePush is true
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

	// 6. If we're not allowed to push directly
	if !canPush {
		// Is this is a merge from the UI/API?
		if ctx.opts.PullRequestID == 0 {
			// 6a. If we're not merging from the UI/API then there are two ways we got here:
			//
			// We are changing a protected file and we're not allowed to do that
			if changedProtectedfiles {
				log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
				})
				return
			}

			// Allow commits that only touch unprotected files
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
					// Commit only touches unprotected files, this is allowed
					return
				}
			}

			// Or we're simply not able to push to this protected branch
			if isForcePush {
				log.Warn("Forbidden: User %d is not allowed to force-push to protected branch: %s in %-v", ctx.opts.UserID, branchName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: fmt.Sprintf("Not allowed to force-push to protected branch %s", branchName),
				})
				return
			}
			log.Warn("Forbidden: User %d is not allowed to push to protected branch: %s in %-v", ctx.opts.UserID, branchName, repo)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("Not allowed to push to protected branch %s", branchName),
			})
			return
		}
		// 6b. Merge (from UI or API)

		// Get the PR, user and permissions for the user in the repository
		pr, err := issues_model.GetPullRequestByID(ctx, ctx.opts.PullRequestID)
		if err != nil {
			log.Error("Unable to get PullRequest %d Error: %v", ctx.opts.PullRequestID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get PullRequest %d Error: %v", ctx.opts.PullRequestID, err),
			})
			return
		}

		// although we should have called `loadPusherAndPermission` before, here we call it explicitly again because we need to access ctx.user below
		if !ctx.loadPusherAndPermission() {
			// if error occurs, loadPusherAndPermission had written the error response
			return
		}

		// Now check if the user is allowed to merge PRs for this repository
		// Note: we can use ctx.perm and ctx.user directly as they will have been loaded above
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
				UserMsg: fmt.Sprintf("Not allowed to push to protected branch %s", branchName),
			})
			return
		}

		// If we're an admin for the repository we can ignore status checks, reviews and override protected files
		if ctx.userPerm.IsAdmin() {
			return
		}

		// Now if we're not an admin - we can't overwrite protected files so fail now
		if changedProtectedfiles {
			log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
			})
			return
		}

		// Check all status checks and reviews are ok
		if err := pull_service.CheckPullBranchProtections(ctx, pr, true); err != nil {
			if models.IsErrDisallowedToMerge(err) {
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

	baseBranchName := refFullName.ForBranchName()

	baseBranchExist := false
	if ctx.Repo.GitRepo.IsBranchExist(baseBranchName) {
		baseBranchExist = true
	}

	if !baseBranchExist {
		for p, v := range baseBranchName {
			if v == '/' && ctx.Repo.GitRepo.IsBranchExist(baseBranchName[:p]) && p != len(baseBranchName)-1 {
				baseBranchExist = true
				break
			}
		}
	}

	if !baseBranchExist {
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("Unexpected ref: %s", refFullName),
		})
		return
	}
}

func generateGitEnv(opts *private.HookOptions) (env []string) {
	env = os.Environ()
	if opts.GitAlternativeObjectDirectories != "" {
		env = append(env,
			private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
	}
	if opts.GitObjectDirectory != "" {
		env = append(env,
			private.GitObjectDirectory+"="+opts.GitObjectDirectory)
	}
	if opts.GitQuarantinePath != "" {
		env = append(env,
			private.GitQuarantinePath+"="+opts.GitQuarantinePath)
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
