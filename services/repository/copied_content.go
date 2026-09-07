// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"

	"github.com/google/uuid"
)

const (
	// CopiedContentMaxJSONBytes is the maximum accepted JSON body size.
	CopiedContentMaxJSONBytes = 1 << 20
	// CopiedContentMaxFiles is the maximum number of mapping entries.
	CopiedContentMaxFiles = 256
	// CopiedContentMaxRegularFileBytes is the maximum size of one ordinary source blob.
	CopiedContentMaxRegularFileBytes = 256 << 20
	// CopiedContentMaxRegularTotalBytes is the maximum ordinary source work per request.
	CopiedContentMaxRegularTotalBytes = 1 << 30
	// CopiedContentRequestTimeout is the maximum handler execution time.
	CopiedContentRequestTimeout = 5 * time.Minute
	// CopiedContentRepresentationGit is ordinary Git blob content.
	CopiedContentRepresentationGit = "git"
	// CopiedContentRepresentationLFS is an LFS pointer blob.
	CopiedContentRepresentationLFS = "lfs"
)

// CopiedContentError is a scrubbed finalizer failure.
type CopiedContentError struct {
	Status  int
	Code    string
	Message string
	Index   *int
}

func (e *CopiedContentError) Error() string {
	if e.Index != nil {
		return fmt.Sprintf("%s: %s (index %d)", e.Code, e.Message, *e.Index)
	}
	return e.Code + ": " + e.Message
}

func copiedErr(status int, code, message string) *CopiedContentError {
	return &CopiedContentError{Status: status, Code: code, Message: message}
}

func copiedErrAt(status int, code, message string, index int) *CopiedContentError {
	return &CopiedContentError{Status: status, Code: code, Message: message, Index: &index}
}

// FinalizeCopiedContentRequest is the trusted-backend finalizer payload.
type FinalizeCopiedContentRequest struct {
	RequestID   string                     `json:"request_id"`
	UserID      int64                      `json:"user_id"`
	Destination CopiedContentDestination   `json:"destination"`
	Files       []CopiedContentFileMapping `json:"files"`
}

// CopiedContentDestination identifies the published destination commit.
type CopiedContentDestination struct {
	RepositoryID int64   `json:"repository_id"`
	Branch       string  `json:"branch"`
	Commit       string  `json:"commit"`
	Parent       *string `json:"parent"`
}

// CopiedContentFileMapping is one source-to-destination file claim.
type CopiedContentFileMapping struct {
	SourceRepositoryID int64  `json:"source_repository_id"`
	SourceCommit       string `json:"source_commit"`
	SourcePath         string `json:"source_path"`
	DestinationPath    string `json:"destination_path"`
	Representation     string `json:"representation"`
}

// FinalizeCopiedContentResponse is the stable success payload.
type FinalizeCopiedContentResponse struct {
	RequestID               string                    `json:"request_id"`
	Status                  string                    `json:"status"`
	DestinationRepositoryID int64                     `json:"destination_repository_id"`
	Commit                  string                    `json:"commit"`
	Files                   []CopiedContentFileResult `json:"files"`
}

// CopiedContentFileResult reports one mapping's finalizer readiness.
type CopiedContentFileResult struct {
	Index           int    `json:"index"`
	DestinationPath string `json:"destination_path"`
	Representation  string `json:"representation"`
	Status          string `json:"status"`
}

type preparedCopiedFile struct {
	index          int
	destPath       string
	representation string
	pointer        *lfs.Pointer
	sourceEntry    *git.TreeEntry
	needSave       bool
}

type openedGitRepo struct {
	model *repo_model.Repository
	git   *git.Repository
}

// FinalizeCopiedContent verifies a published copy commit and associates missing LFS objects.
func FinalizeCopiedContent(ctx context.Context, req *FinalizeCopiedContentRequest) (*FinalizeCopiedContentResponse, error) {
	if req == nil {
		return nil, copiedErr(http.StatusBadRequest, "invalid_request", "request is required")
	}
	if err := validateCopiedContentRequest(req); err != nil {
		return nil, err
	}

	actor, err := user_model.GetUserByID(ctx, req.UserID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return nil, copiedErr(http.StatusBadRequest, "invalid_request", "user_id is invalid")
		}
		return nil, unavailableErr(err)
	}
	if !actor.IsActive || actor.ProhibitLogin || actor.IsGhost() || actor.IsGiteaActions() {
		return nil, copiedErr(http.StatusForbidden, "forbidden", "actor is not permitted")
	}

	destRepo, err := repo_model.GetRepositoryByID(ctx, req.Destination.RepositoryID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return nil, copiedErr(http.StatusNotFound, "not_found", "repository not found")
		}
		return nil, unavailableErr(err)
	}
	if destRepo.IsArchived {
		return nil, copiedErr(http.StatusForbidden, "forbidden", "destination repository is archived")
	}
	if err := requireRepoAccess(ctx, actor, destRepo, true); err != nil {
		return nil, err
	}

	sourceRepos := make(map[int64]*repo_model.Repository, len(req.Files))
	for _, file := range req.Files {
		if file.SourceRepositoryID == destRepo.ID {
			return nil, copiedErr(http.StatusBadRequest, "invalid_request", "source repository must differ from destination")
		}
		if _, ok := sourceRepos[file.SourceRepositoryID]; ok {
			continue
		}
		sourceRepo, err := repo_model.GetRepositoryByID(ctx, file.SourceRepositoryID)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				return nil, copiedErr(http.StatusNotFound, "not_found", "repository not found")
			}
			return nil, unavailableErr(err)
		}
		if err := requireRepoAccess(ctx, actor, sourceRepo, false); err != nil {
			return nil, concealSourceErr(err)
		}
		sourceRepos[file.SourceRepositoryID] = sourceRepo
	}

	opened := make(map[int64]*openedGitRepo, len(sourceRepos)+1)
	defer closeOpenedGitRepos(opened)
	destOpened, err := openCopiedRepo(ctx, opened, destRepo)
	if err != nil {
		return nil, err
	}

	destCommit, err := loadExactCommit(destOpened.git, destRepo, req.Destination.Commit)
	if err != nil {
		return nil, err
	}
	if err := requireCommitOnBranch(destOpened.git, req.Destination.Branch, destCommit); err != nil {
		return nil, err
	}
	if err := requireCommitParent(destCommit, req.Destination.Parent, destRepo); err != nil {
		return nil, err
	}
	if err := requireExactCreates(destCommit, req.Files); err != nil {
		return nil, err
	}

	prepared := make([]*preparedCopiedFile, 0, len(req.Files))
	var regularTotal int64
	for i, file := range req.Files {
		sourceOpened, err := openCopiedRepo(ctx, opened, sourceRepos[file.SourceRepositoryID])
		if err != nil {
			return nil, concealSourceErr(err)
		}
		sourceCommit, err := loadExactCommit(sourceOpened.git, sourceRepos[file.SourceRepositoryID], file.SourceCommit)
		if err != nil {
			return nil, concealSourceErr(err)
		}
		preparedFile, added, err := prepareCopiedFile(ctx, destRepo, sourceRepos[file.SourceRepositoryID], destCommit, sourceCommit, file, i)
		if err != nil {
			return nil, err
		}
		regularTotal, err = addCopiedBytes(regularTotal, added)
		if err != nil {
			return nil, err
		}
		if regularTotal > CopiedContentMaxRegularTotalBytes {
			return nil, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source work exceeds the allowed total")
		}
		prepared = append(prepared, preparedFile)
	}

	store := lfs.NewContentStore()
	for _, file := range prepared {
		if file.pointer == nil {
			continue
		}
		ok, err := store.Verify(*file.pointer)
		if err != nil {
			return nil, unavailableErr(err)
		}
		if ok {
			file.needSave = false
			continue
		}
		exists, err := store.Exists(*file.pointer)
		if err != nil {
			return nil, unavailableErr(err)
		}
		if exists {
			return nil, copiedErr(http.StatusServiceUnavailable, "unavailable", "stored object failed verification")
		}
		if !file.needSave {
			return nil, copiedErr(http.StatusServiceUnavailable, "unavailable", "expected LFS payload is missing")
		}
		if err := materializeCopiedPointer(ctx, store, file); err != nil {
			return nil, err
		}
	}

	if err := reverifyCopiedAccess(ctx, actor, destRepo, sourceRepos); err != nil {
		return nil, err
	}

	pointers := make([]git_model.CopiedContentLFSPointer, 0, len(prepared))
	results := make([]CopiedContentFileResult, 0, len(prepared))
	for _, file := range prepared {
		results = append(results, CopiedContentFileResult{
			Index:           file.index,
			DestinationPath: file.destPath,
			Representation:  file.representation,
			Status:          "ready",
		})
		if file.pointer == nil {
			continue
		}
		ok, err := store.Verify(*file.pointer)
		if err != nil {
			return nil, unavailableErr(err)
		}
		if !ok {
			return nil, copiedErr(http.StatusServiceUnavailable, "unavailable", "expected LFS payload is missing")
		}
		pointers = append(pointers, git_model.CopiedContentLFSPointer{Oid: file.pointer.Oid, Size: file.pointer.Size})
	}

	if err := git_model.AssociateCopiedContentLFS(ctx, destRepo.ID, pointers); err != nil {
		return nil, mapAssociationError(err)
	}

	return &FinalizeCopiedContentResponse{
		RequestID:               req.RequestID,
		Status:                  "finalized",
		DestinationRepositoryID: destRepo.ID,
		Commit:                  destCommit.ID.String(),
		Files:                   results,
	}, nil
}

func validateCopiedContentRequest(req *FinalizeCopiedContentRequest) error {
	if _, err := uuid.Parse(req.RequestID); err != nil {
		return copiedErr(http.StatusBadRequest, "invalid_request", "request_id must be a UUID")
	}
	if req.UserID <= 0 {
		return copiedErr(http.StatusBadRequest, "invalid_request", "user_id must be a positive integer")
	}
	if req.Destination.RepositoryID <= 0 {
		return copiedErr(http.StatusBadRequest, "invalid_request", "destination.repository_id must be a positive integer")
	}
	if !isCopiedBranchName(req.Destination.Branch) {
		return copiedErr(http.StatusBadRequest, "invalid_request", "destination.branch is invalid")
	}
	if req.Destination.Commit == "" {
		return copiedErr(http.StatusBadRequest, "invalid_request", "destination.commit is required")
	}
	if len(req.Files) == 0 {
		return copiedErr(http.StatusBadRequest, "invalid_request", "files must be nonempty")
	}
	if len(req.Files) > CopiedContentMaxFiles {
		return copiedErr(http.StatusRequestEntityTooLarge, "too_large", "files exceeds the allowed entry count")
	}

	destPaths := make([]string, 0, len(req.Files))
	for _, file := range req.Files {
		if file.SourceRepositoryID <= 0 {
			return copiedErr(http.StatusBadRequest, "invalid_request", "source_repository_id must be a positive integer")
		}
		if file.SourceCommit == "" {
			return copiedErr(http.StatusBadRequest, "invalid_request", "source_commit is required")
		}
		if !isCopiedRepoPath(file.SourcePath) || !isCopiedRepoPath(file.DestinationPath) {
			return copiedErr(http.StatusBadRequest, "invalid_request", "file path is invalid")
		}
		switch file.Representation {
		case CopiedContentRepresentationGit, CopiedContentRepresentationLFS:
		default:
			return copiedErr(http.StatusBadRequest, "invalid_request", "representation must be git or lfs")
		}
		destPaths = append(destPaths, file.DestinationPath)
	}
	if hasDuplicateOrAncestorPath(destPaths) {
		return copiedErr(http.StatusBadRequest, "invalid_request", "destination paths conflict")
	}
	return nil
}

func requireRepoAccess(ctx context.Context, actor *user_model.User, repo *repo_model.Repository, write bool) error {
	perm, err := access.GetDoerRepoPermission(ctx, repo, actor)
	if err != nil {
		return unavailableErr(err)
	}
	if write {
		if !perm.CanWrite(unit.TypeCode) {
			return copiedErr(http.StatusForbidden, "forbidden", "destination write denied")
		}
		return nil
	}
	if !perm.CanRead(unit.TypeCode) {
		return copiedErr(http.StatusNotFound, "not_found", "repository not found")
	}
	return nil
}

func concealSourceErr(err error) error {
	var copied *CopiedContentError
	if errors.As(err, &copied) {
		if copied.Status == http.StatusForbidden {
			return copiedErr(http.StatusNotFound, "not_found", "repository not found")
		}
		return copied
	}
	if repo_model.IsErrRepoNotExist(err) || git.IsErrNotExist(err) {
		return copiedErr(http.StatusNotFound, "not_found", "repository not found")
	}
	return unavailableErr(err)
}

func openCopiedRepo(ctx context.Context, opened map[int64]*openedGitRepo, repo *repo_model.Repository) (*openedGitRepo, error) {
	if existing, ok := opened[repo.ID]; ok {
		return existing, nil
	}
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return nil, unavailableErr(err)
	}
	entry := &openedGitRepo{model: repo, git: gitRepo}
	opened[repo.ID] = entry
	return entry, nil
}

func closeOpenedGitRepos(opened map[int64]*openedGitRepo) {
	for _, repo := range opened {
		if repo != nil && repo.git != nil {
			repo.git.Close()
		}
	}
}

func loadExactCommit(gitRepo *git.Repository, repo *repo_model.Repository, commitID string) (*git.Commit, error) {
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	if objectFormat == nil {
		return nil, copiedErr(http.StatusServiceUnavailable, "unavailable", "repository object format is unavailable")
	}
	if len(commitID) != objectFormat.FullLength() || !objectFormat.IsValid(commitID) {
		return nil, copiedErr(http.StatusBadRequest, "invalid_request", "commit id is invalid")
	}
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, copiedErr(http.StatusNotFound, "not_found", "commit not found")
		}
		return nil, unavailableErr(err)
	}
	if commit.ID.String() != commitID {
		return nil, copiedErr(http.StatusConflict, "conflict", "commit does not match the copy plan")
	}
	return commit, nil
}

func requireCommitOnBranch(gitRepo *git.Repository, branch string, destCommit *git.Commit) error {
	if !gitRepo.IsBranchExist(branch) {
		return copiedErr(http.StatusNotFound, "not_found", "branch not found")
	}
	branchCommit, err := gitRepo.GetBranchCommit(branch)
	if err != nil {
		if git.IsErrNotExist(err) || git.IsErrBranchNotExist(err) {
			return copiedErr(http.StatusNotFound, "not_found", "branch not found")
		}
		return unavailableErr(err)
	}
	if branchCommit.ID.String() == destCommit.ID.String() {
		return nil
	}
	reachable, err := branchCommit.HasPreviousCommit(destCommit.ID)
	if err != nil {
		return unavailableErr(err)
	}
	if !reachable {
		return copiedErr(http.StatusConflict, "conflict", "destination commit is not reachable from the branch")
	}
	return nil
}

func requireCommitParent(commit *git.Commit, parent *string, repo *repo_model.Repository) error {
	if parent == nil {
		if commit.ParentCount() != 0 {
			return copiedErr(http.StatusConflict, "conflict", "destination parent does not match the copy plan")
		}
		return nil
	}
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	if objectFormat == nil {
		return copiedErr(http.StatusServiceUnavailable, "unavailable", "repository object format is unavailable")
	}
	if len(*parent) != objectFormat.FullLength() || !objectFormat.IsValid(*parent) {
		return copiedErr(http.StatusBadRequest, "invalid_request", "destination.parent is invalid")
	}
	if commit.ParentCount() != 1 {
		return copiedErr(http.StatusConflict, "conflict", "destination parent does not match the copy plan")
	}
	parentID, err := commit.ParentID(0)
	if err != nil {
		return unavailableErr(err)
	}
	if parentID.String() != *parent {
		return copiedErr(http.StatusConflict, "conflict", "destination parent does not match the copy plan")
	}
	return nil
}

func requireExactCreates(destCommit *git.Commit, files []CopiedContentFileMapping) error {
	destFiles, err := blobPathMap(destCommit)
	if err != nil {
		return err
	}
	parentFiles := map[string]*git.TreeEntry{}
	if destCommit.ParentCount() > 0 {
		parent, err := destCommit.Parent(0)
		if err != nil {
			return unavailableErr(err)
		}
		parentFiles, err = blobPathMap(parent)
		if err != nil {
			return err
		}
	}

	planned := make(map[string]struct{}, len(files))
	for _, file := range files {
		planned[file.DestinationPath] = struct{}{}
	}

	for p := range parentFiles {
		if _, ok := destFiles[p]; !ok {
			return copiedErr(http.StatusConflict, "conflict", "destination commit contains non-create changes")
		}
	}
	creates := 0
	for p, destEntry := range destFiles {
		parentEntry, existed := parentFiles[p]
		if existed {
			if parentEntry.ID.String() != destEntry.ID.String() || parentEntry.Mode() != destEntry.Mode() {
				return copiedErr(http.StatusConflict, "conflict", "destination commit contains non-create changes")
			}
			if _, ok := planned[p]; ok {
				return copiedErr(http.StatusConflict, "conflict", "destination path already exists in the parent")
			}
			continue
		}
		if _, ok := planned[p]; !ok {
			return copiedErr(http.StatusConflict, "conflict", "destination commit contains extra paths")
		}
		if destEntry.IsLink() || destEntry.IsSubModule() || destEntry.IsDir() {
			return copiedErr(http.StatusUnprocessableEntity, "unprocessable", "unsupported git entry kind")
		}
		if !destEntry.IsRegular() && !destEntry.IsExecutable() {
			return copiedErr(http.StatusUnprocessableEntity, "unprocessable", "unsupported git entry kind")
		}
		creates++
	}
	if creates != len(files) {
		return copiedErr(http.StatusConflict, "conflict", "destination commit does not match the copy plan")
	}
	for _, file := range files {
		if _, ok := destFiles[file.DestinationPath]; !ok {
			return copiedErr(http.StatusConflict, "conflict", "destination commit omits a planned path")
		}
	}
	return nil
}

func blobPathMap(commit *git.Commit) (map[string]*git.TreeEntry, error) {
	entries, err := commit.Tree.ListEntriesRecursiveFast()
	if err != nil {
		return nil, unavailableErr(err)
	}
	files := make(map[string]*git.TreeEntry, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files[entry.Name()] = entry
	}
	return files, nil
}

func prepareCopiedFile(ctx context.Context, destRepo, sourceRepo *repo_model.Repository, destCommit, sourceCommit *git.Commit, file CopiedContentFileMapping, index int) (*preparedCopiedFile, int64, error) {
	sourceEntry, err := sourceCommit.GetTreeEntryByPath(file.SourcePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, 0, copiedErr(http.StatusNotFound, "not_found", "repository not found")
		}
		return nil, 0, unavailableErr(err)
	}
	destEntry, err := destCommit.GetTreeEntryByPath(file.DestinationPath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, 0, copiedErr(http.StatusConflict, "conflict", "destination commit omits a planned path")
		}
		return nil, 0, unavailableErr(err)
	}
	if sourceEntry.IsDir() || sourceEntry.IsLink() || sourceEntry.IsSubModule() ||
		destEntry.IsDir() || destEntry.IsLink() || destEntry.IsSubModule() {
		return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "unsupported git entry kind", index)
	}

	sourcePointer, sourceIsPointer, err := readCopiedPointer(sourceEntry)
	if err != nil {
		return nil, 0, err
	}
	if sourceIsPointer && file.Representation != CopiedContentRepresentationLFS {
		return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "source LFS pointer requires lfs representation", index)
	}

	prepared := &preparedCopiedFile{
		index:          index,
		destPath:       file.DestinationPath,
		representation: file.Representation,
		sourceEntry:    sourceEntry,
	}

	switch file.Representation {
	case CopiedContentRepresentationGit:
		if err := boundRegularSize(sourceEntry.Blob().Size()); err != nil {
			return nil, 0, err
		}
		if sourceEntry.Mode() != destEntry.Mode() {
			return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "git content does not match", index)
		}
		equal, err := gitBlobsEqual(sourceRepo, destRepo, sourceEntry, destEntry)
		if err != nil {
			return nil, 0, err
		}
		if !equal {
			return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "git content does not match", index)
		}
		return prepared, sourceEntry.Blob().Size(), nil
	case CopiedContentRepresentationLFS:
		destPointer, destIsPointer, err := readCopiedPointer(destEntry)
		if err != nil {
			return nil, 0, err
		}
		if !destIsPointer || !destEntry.IsRegular() {
			return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "target LFS pointer is invalid", index)
		}
		if destContent, err := destEntry.Blob().GetBlobBytes(lfs.MetaFileMaxSize); err != nil {
			return nil, 0, unavailableErr(err)
		} else if string(destContent) != destPointer.StringContent() {
			return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "target LFS pointer is not canonical", index)
		}
		prepared.pointer = &destPointer
		if sourceIsPointer {
			if err := verifySourceLFSAssociation(ctx, sourceRepo, sourcePointer); err != nil {
				return nil, 0, err
			}
			if sourcePointer.Oid != destPointer.Oid || sourcePointer.Size != destPointer.Size {
				return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "LFS pointer does not match", index)
			}
			return prepared, 0, nil
		}
		if sourceEntry.Blob().Size() > CopiedContentMaxRegularFileBytes {
			return nil, 0, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source exceeds the allowed size")
		}
		computed, err := hashBlobSHA256(ctx, sourceEntry.Blob(), CopiedContentMaxRegularFileBytes)
		if err != nil {
			return nil, 0, err
		}
		if computed.Oid != destPointer.Oid || computed.Size != destPointer.Size {
			return nil, 0, copiedErrAt(http.StatusUnprocessableEntity, "unprocessable", "regular source does not match the target pointer", index)
		}
		prepared.needSave = true
		return prepared, computed.Size, nil
	default:
		return nil, 0, copiedErr(http.StatusBadRequest, "invalid_request", "representation must be git or lfs")
	}
}

func verifySourceLFSAssociation(ctx context.Context, sourceRepo *repo_model.Repository, pointer lfs.Pointer) error {
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, sourceRepo.ID, pointer.Oid)
	if err != nil {
		if errors.Is(err, git_model.ErrLFSObjectNotExist) {
			return copiedErr(http.StatusNotFound, "not_found", "repository not found")
		}
		return unavailableErr(err)
	}
	if meta.Size != pointer.Size || meta.Oid != pointer.Oid {
		return copiedErr(http.StatusUnprocessableEntity, "unprocessable", "LFS pointer does not match")
	}
	return nil
}

func materializeCopiedPointer(ctx context.Context, store *lfs.ContentStore, file *preparedCopiedFile) error {
	reader, err := file.sourceEntry.Blob().DataAsync()
	if err != nil {
		return unavailableErr(err)
	}
	defer reader.Close()
	if err := store.Put(*file.pointer, &contextReader{ctx: ctx, r: reader}); err != nil {
		return unavailableErr(err)
	}
	ok, err := store.Verify(*file.pointer)
	if err != nil || !ok {
		if err == nil {
			err = errors.New("materialized object failed verification")
		}
		return unavailableErr(err)
	}
	return nil
}

func gitBlobsEqual(sourceRepo, destRepo *repo_model.Repository, sourceEntry, destEntry *git.TreeEntry) (bool, error) {
	if sourceEntry.Blob().Size() != destEntry.Blob().Size() {
		return false, nil
	}
	if sourceRepo.ObjectFormatName == destRepo.ObjectFormatName && sourceEntry.ID.String() == destEntry.ID.String() {
		return true, nil
	}
	sourceReader, err := sourceEntry.Blob().DataAsync()
	if err != nil {
		return false, unavailableErr(err)
	}
	defer sourceReader.Close()
	destReader, err := destEntry.Blob().DataAsync()
	if err != nil {
		return false, unavailableErr(err)
	}
	defer destReader.Close()
	bufA := make([]byte, 32*1024)
	bufB := make([]byte, 32*1024)
	for {
		nA, errA := io.ReadFull(sourceReader, bufA)
		nB, errB := io.ReadFull(destReader, bufB)
		if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false, nil
		}
		if errors.Is(errA, io.EOF) || errors.Is(errA, io.ErrUnexpectedEOF) {
			return errors.Is(errB, io.EOF) || errors.Is(errB, io.ErrUnexpectedEOF), nil
		}
		if errA != nil {
			return false, unavailableErr(errA)
		}
		if errB != nil {
			return false, unavailableErr(errB)
		}
	}
}

func readCopiedPointer(entry *git.TreeEntry) (lfs.Pointer, bool, error) {
	blob := entry.Blob()
	if blob.Size() == 0 || blob.Size() > lfs.MetaFileMaxSize {
		return lfs.Pointer{}, false, nil
	}
	data, err := blob.GetBlobBytes(lfs.MetaFileMaxSize)
	if err != nil {
		return lfs.Pointer{}, false, unavailableErr(err)
	}
	pointer, err := lfs.ReadPointerFromBuffer(data)
	if err != nil {
		return lfs.Pointer{}, false, nil
	}
	return pointer, true, nil
}

func hashBlobSHA256(ctx context.Context, blob *git.Blob, maxSize int64) (lfs.Pointer, error) {
	if blob.Size() > maxSize {
		return lfs.Pointer{}, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source exceeds the allowed size")
	}
	reader, err := blob.DataAsync()
	if err != nil {
		return lfs.Pointer{}, unavailableErr(err)
	}
	defer reader.Close()
	hasher := sha256.New()
	written, err := io.Copy(hasher, &contextReader{ctx: ctx, r: io.LimitReader(reader, maxSize+1)})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return lfs.Pointer{}, copiedErr(http.StatusServiceUnavailable, "unavailable", "request exceeded the allowed execution time")
		}
		return lfs.Pointer{}, unavailableErr(err)
	}
	if written > maxSize {
		return lfs.Pointer{}, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source exceeds the allowed size")
	}
	return lfs.Pointer{Oid: hex.EncodeToString(hasher.Sum(nil)), Size: written}, nil
}

func boundRegularSize(size int64) error {
	if size < 0 {
		return copiedErr(http.StatusUnprocessableEntity, "unprocessable", "git content does not match")
	}
	if size > CopiedContentMaxRegularFileBytes {
		return copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source exceeds the allowed size")
	}
	return nil
}

func addCopiedBytes(total, n int64) (int64, error) {
	if n < 0 {
		return 0, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source work exceeds the allowed total")
	}
	if total > math.MaxInt64-n {
		return 0, copiedErr(http.StatusRequestEntityTooLarge, "too_large", "regular source work exceeds the allowed total")
	}
	return total + n, nil
}

func reverifyCopiedAccess(ctx context.Context, actor *user_model.User, destRepo *repo_model.Repository, sourceRepos map[int64]*repo_model.Repository) error {
	freshDest, err := repo_model.GetRepositoryByID(ctx, destRepo.ID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return copiedErr(http.StatusNotFound, "not_found", "repository not found")
		}
		return unavailableErr(err)
	}
	if freshDest.IsArchived {
		return copiedErr(http.StatusForbidden, "forbidden", "destination repository is archived")
	}
	if err := requireRepoAccess(ctx, actor, freshDest, true); err != nil {
		return err
	}
	for _, sourceRepo := range sourceRepos {
		freshSource, err := repo_model.GetRepositoryByID(ctx, sourceRepo.ID)
		if err != nil {
			return concealSourceErr(err)
		}
		if err := requireRepoAccess(ctx, actor, freshSource, false); err != nil {
			return concealSourceErr(err)
		}
	}
	return nil
}

func mapAssociationError(err error) error {
	switch {
	case errors.Is(err, git_model.ErrCopiedContentSizeConflict):
		return copiedErr(http.StatusConflict, "conflict", "existing association metadata conflicts")
	case errors.Is(err, git_model.ErrCopiedContentRepoNotExist):
		return copiedErr(http.StatusNotFound, "not_found", "repository not found")
	case errors.Is(err, git_model.ErrCopiedContentRetryExhausted):
		return copiedErr(http.StatusServiceUnavailable, "unavailable", "could not persist LFS associations")
	default:
		return unavailableErr(err)
	}
}

func unavailableErr(err error) error {
	if err == nil {
		return copiedErr(http.StatusServiceUnavailable, "unavailable", "service unavailable")
	}
	var copied *CopiedContentError
	if errors.As(err, &copied) {
		return copied
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return copiedErr(http.StatusServiceUnavailable, "unavailable", "request exceeded the allowed execution time")
	}
	log.Error("copied content finalizer unavailable: %v", err)
	return copiedErr(http.StatusServiceUnavailable, "unavailable", "service unavailable")
}

func isCopiedBranchName(name string) bool {
	if name == "" || strings.HasPrefix(name, "refs/") || strings.Contains(name, "..") {
		return false
	}
	return git.IsValidRefPattern(name)
}

func isCopiedRepoPath(p string) bool {
	if p == "" || strings.ContainsRune(p, 0) || strings.Contains(p, "\\") || strings.HasPrefix(p, "/") {
		return false
	}
	if path.Clean(p) != p {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return false
			}
		}
	}
	return true
}

func hasDuplicateOrAncestorPath(paths []string) bool {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	seen := make(map[string]struct{}, len(sorted))
	for _, p := range sorted {
		if _, ok := seen[p]; ok {
			return true
		}
		seen[p] = struct{}{}
	}
	for i := 0; i < len(sorted)-1; i++ {
		if strings.HasPrefix(sorted[i+1], sorted[i]+"/") {
			return true
		}
	}
	return false
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
