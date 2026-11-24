// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"unicode/utf8"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/git"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
)

const CodeSuggestionsBatchCommitMessage = "Apply suggestions from code review"

const (
	codeSuggestionReasonPrefix = "repo.diff.suggestion.not_appliable."
	maxCodeSuggestions         = 50
)

var ErrCodeSuggestionHeadChanged = util.ErrorWrapTranslatable(util.ErrAlreadyExist, codeSuggestionReasonPrefix+"head_changed")

type codeSuggestionRejection string

func (reason codeSuggestionRejection) Error() string { return string(reason) }

func (reason codeSuggestionRejection) Unwrap() error { return util.ErrUnprocessableContent }

func rejectCodeSuggestion(reason string) error {
	rejection := codeSuggestionRejection(codeSuggestionReasonPrefix + reason)
	return util.ErrorWrapTranslatable(rejection, string(rejection))
}

type CodeSuggestionFileChange struct {
	Path    string
	BlobID  string
	Content []byte
}

type CodeSuggestionChanges struct {
	HeadCommitID  string
	Files         []CodeSuggestionFileChange
	Comments      issues_model.CommentList
	Conversations issues_model.CommentList
}

type codeSuggestionConversation struct {
	path string
	line int64
}

type codeSuggestionPath struct {
	repo     *git.Repository
	commitID string
	path     string
}

type codeSuggestionBlob struct {
	id        string
	content   []byte
	rejection error
}

type codeSuggestionLoader struct {
	pr            *issues_model.PullRequest
	headRepo      *git.Repository
	baseRepo      *git.Repository
	baseCloser    io.Closer
	headCloser    io.Closer
	headCommitID  string
	canPushToPath func(string) bool
	cache         *cache.EphemeralCache
}

func openCodeSuggestionLoader(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, expectedHead string) (_ *codeSuggestionLoader, errRet error) {
	if doer == nil || pr.Flow != issues_model.PullRequestFlowGithub || pr.HeadBranch == "" {
		return nil, util.ErrPermissionDenied
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, err
	}
	if pr.HeadRepo == nil || !pr.HeadRepo.CanContentChange() {
		return nil, util.ErrPermissionDenied
	}
	permission, err := access_model.GetDoerRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		return nil, err
	}
	if canWrite, err := issues_model.CheckMaintainerWriteToBranch(ctx, permission, pr.HeadBranch, doer); err != nil {
		return nil, err
	} else if !canWrite {
		return nil, util.ErrPermissionDenied
	}

	headRepo, headCloser, err := git.RepositoryFromContextOrOpen(ctx, pr.HeadRepo)
	if errors.Is(err, util.ErrNotExist) {
		return nil, util.ErrPermissionDenied
	} else if err != nil {
		return nil, err
	}
	defer func() {
		if errRet != nil {
			headCloser.Close()
		}
	}()
	headCommitID, err := headRepo.GetBranchCommitID(ctx, pr.HeadBranch)
	if errors.Is(err, util.ErrNotExist) {
		if expectedHead != "" {
			return nil, ErrCodeSuggestionHeadChanged
		}
		return nil, util.ErrPermissionDenied
	} else if err != nil {
		return nil, err
	}
	if expectedHead != "" && headCommitID != expectedHead {
		return nil, ErrCodeSuggestionHeadChanged
	}

	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	loader := &codeSuggestionLoader{
		pr: pr, headRepo: headRepo, headCloser: headCloser, headCommitID: headCommitID,
		cache: cache.NewEphemeralCache(),
	}
	protectedBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.HeadRepoID, pr.HeadBranch)
	if err != nil {
		return nil, err
	}
	if protectedBranch != nil {
		if protectedBranch.RequireSignedCommits {
			_, _, _, err = asymkey_service.SignCRUDAction(ctx, doer, headRepo, pr.HeadBranch)
			if asymkey_service.IsErrWontSign(err) {
				return nil, util.ErrPermissionDenied
			} else if err != nil {
				return nil, err
			}
		}
		protectedBranch.Repo = pr.HeadRepo
		canPush := protectedBranch.CanUserPush(ctx, doer, permission)
		protectedPaths, unprotectedPaths := protectedBranch.GetProtectedFilePatterns(), protectedBranch.GetUnprotectedFilePatterns()
		loader.canPushToPath = func(path string) bool {
			return (canPush || protectedBranch.IsUnprotectedFile(unprotectedPaths, path)) && !protectedBranch.IsProtectedFile(protectedPaths, path)
		}
	}
	return loader, nil
}

func (loader *codeSuggestionLoader) close() {
	if loader.baseCloser != nil {
		loader.baseCloser.Close()
	}
	loader.headCloser.Close()
}

func (loader *codeSuggestionLoader) getBaseRepo(ctx context.Context) (*git.Repository, error) {
	if loader.pr.BaseRepoID == loader.pr.HeadRepoID {
		return loader.headRepo, nil
	}
	if loader.baseRepo == nil {
		if err := loader.pr.LoadBaseRepo(ctx); err != nil {
			return nil, err
		}
		var err error
		loader.baseRepo, loader.baseCloser, err = git.RepositoryFromContextOrOpen(ctx, loader.pr.BaseRepo)
		if err != nil {
			return nil, err
		}
	}
	return loader.baseRepo, nil
}

func (loader *codeSuggestionLoader) getConversation(ctx context.Context, comment *issues_model.Comment) (*issues_model.Comment, error) {
	return cache.GetWithEphemeralCache(ctx, loader.cache, "conversation", codeSuggestionConversation{comment.TreePath, comment.Line},
		func(ctx context.Context, key codeSuggestionConversation) (*issues_model.Comment, error) {
			return issues_model.GetPublishedCodeConversationRoot(ctx, loader.pr.IssueID, key.path, key.line)
		})
}

func (loader *codeSuggestionLoader) getBlob(ctx context.Context, key codeSuggestionPath) (codeSuggestionBlob, error) {
	blob, err := cache.GetWithEphemeralCache(ctx, loader.cache, "path", key, loader.readBlob)
	return blob, cmp.Or(err, blob.rejection)
}

func (loader *codeSuggestionLoader) readBlob(ctx context.Context, key codeSuggestionPath) (codeSuggestionBlob, error) {
	commit, err := key.repo.GetCommit(ctx, key.commitID)
	if errors.Is(err, util.ErrNotExist) {
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("outdated")}, nil
	} else if err != nil {
		return codeSuggestionBlob{}, err
	}
	entry, err := commit.GetTreeEntryByPath(ctx, key.repo, key.path)
	if errors.Is(err, util.ErrNotExist) {
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("outdated")}, nil
	} else if err != nil {
		return codeSuggestionBlob{}, err
	}
	if !entry.IsRegular() && !entry.IsExecutable() {
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("unsupported_file")}, nil
	}

	return cache.GetWithEphemeralCache(ctx, loader.cache, "blob", entry.ID.String(), func(ctx context.Context, id string) (codeSuggestionBlob, error) {
		entryBlob := entry.Blob(key.repo)
		if entryBlob.Size(ctx) > setting.UI.MaxDisplayFileSize {
			return codeSuggestionBlob{rejection: rejectCodeSuggestion("unsupported_file")}, nil
		}
		content, err := entryBlob.GetBlobBytes(ctx, setting.UI.MaxDisplayFileSize+1)
		if err != nil {
			return codeSuggestionBlob{}, err
		}
		_, pointerErr := lfs.ReadPointerFromBuffer(content)
		if int64(len(content)) > setting.UI.MaxDisplayFileSize || !isCodeSuggestionText(content) || pointerErr == nil {
			return codeSuggestionBlob{rejection: rejectCodeSuggestion("unsupported_file")}, nil
		}
		return codeSuggestionBlob{id: id, content: content}, nil
	})
}

func isCodeSuggestionText(content []byte) bool {
	return utf8.Valid(content) && bytes.IndexByte(content, 0) < 0
}

type codeSuggestionEdit struct {
	start, end  int
	replacement []byte
}

type codeSuggestionSelection struct {
	edit         codeSuggestionEdit
	conversation *issues_model.Comment
}

func (loader *codeSuggestionLoader) getReview(ctx context.Context, comment *issues_model.Comment) (*issues_model.Review, error) {
	if comment.Review != nil {
		return comment.Review, nil
	}
	return cache.GetWithEphemeralCache(ctx, loader.cache, "review", comment.ReviewID, issues_model.GetReviewByID)
}

func (loader *codeSuggestionLoader) validate(ctx context.Context, comment *issues_model.Comment, suggestions []string) (codeSuggestionSelection, error) {
	if loader.pr.HasMerged || loader.pr.Issue.IsClosed {
		return codeSuggestionSelection{}, rejectCodeSuggestion("closed")
	}
	if comment.Line <= 0 || comment.CodeStartLine() <= 0 {
		return codeSuggestionSelection{}, rejectCodeSuggestion("deleted_line")
	}
	if len(suggestions) != 1 {
		return codeSuggestionSelection{}, rejectCodeSuggestion("multiple")
	}
	if comment.ReviewID == 0 || comment.Invalidated {
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	if util.PathJoinRel(comment.TreePath) != comment.TreePath {
		return codeSuggestionSelection{}, rejectCodeSuggestion("unsupported_file")
	}
	review, err := loader.getReview(ctx, comment)
	if errors.Is(err, util.ErrNotExist) {
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	} else if err != nil {
		return codeSuggestionSelection{}, err
	}
	if review.IssueID != loader.pr.IssueID {
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	switch review.Type {
	case issues_model.ReviewTypePending:
		return codeSuggestionSelection{}, rejectCodeSuggestion("pending")
	case issues_model.ReviewTypeApprove, issues_model.ReviewTypeComment, issues_model.ReviewTypeReject:
	default:
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	first, err := loader.getConversation(ctx, comment)
	if errors.Is(err, util.ErrNotExist) {
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	if err != nil {
		return codeSuggestionSelection{}, err
	}
	if first.IsResolved() {
		return codeSuggestionSelection{}, rejectCodeSuggestion("resolved")
	}

	edit, err := loader.validateEdit(ctx, comment, suggestions[0])
	return codeSuggestionSelection{edit: edit, conversation: first}, err
}

func (loader *codeSuggestionLoader) validateEdit(ctx context.Context, comment *issues_model.Comment, suggestion string) (codeSuggestionEdit, error) {
	baseRepo, err := loader.getBaseRepo(ctx)
	if err != nil {
		return codeSuggestionEdit{}, err
	}
	original, err := loader.getBlob(ctx, codeSuggestionPath{baseRepo, comment.CodeCommitID(), comment.TreePath})
	if err != nil {
		return codeSuggestionEdit{}, err
	}
	blob, err := loader.getBlob(ctx, codeSuggestionPath{loader.headRepo, loader.headCommitID, comment.TreePath})
	if err != nil {
		return codeSuggestionEdit{}, err
	}
	if original.id != blob.id {
		hunks, err := getCodeCommentDiff(ctx, loader.cache, loader.headRepo, codeCommentDiff{
			before: comment.CodeCommitID(), after: loader.headCommitID, path: comment.TreePath,
		})
		if errors.Is(err, util.ErrNotExist) {
			return codeSuggestionEdit{}, rejectCodeSuggestion("outdated")
		} else if err != nil {
			return codeSuggestionEdit{}, err
		}
		if codeSuggestionAnchorMoved(hunks, comment.CodeStartLine(), comment.Line) {
			return codeSuggestionEdit{}, rejectCodeSuggestion("outdated")
		}
	}
	return validateSuggestionLines(original.content, blob.content, comment.CodeStartLine(), comment.Line, suggestion)
}

func codeSuggestionAnchorMoved(hunks []git.FileDiffHunk, startLine, endLine int64) bool {
	var shift int64
	for _, hunk := range hunks {
		if hunk.OldStart+hunk.OldCount <= startLine {
			shift += hunk.NewCount - hunk.OldCount
		} else if hunk.OldStart <= endLine {
			return true
		}
	}
	return shift != 0
}

func suggestionLineRange(content []byte, startLine, endLine int64) (start, end int, ok bool) {
	if startLine < 1 || endLine < startLine {
		return 0, 0, false
	}
	var lineNumber int64
	for line := range bytes.Lines(content) {
		lineNumber++
		if lineNumber == startLine {
			start = end
		}
		end += len(line)
		if lineNumber == endLine {
			return start, end, true
		}
	}
	return 0, 0, false
}

func validateSuggestionLines(original, head []byte, startLine, endLine int64, suggestion string) (codeSuggestionEdit, error) {
	originalStart, originalEnd, originalValid := suggestionLineRange(original, startLine, endLine)
	start, end, headValid := suggestionLineRange(head, startLine, endLine)
	if !originalValid || !headValid || !bytes.Equal(original[originalStart:originalEnd], head[start:end]) {
		return codeSuggestionEdit{}, rejectCodeSuggestion("outdated")
	}

	lines := head[start:end]
	replacement := []byte(suggestion)
	if len(replacement) != 0 {
		if end == len(head) && head[end-1] != '\n' {
			replacement = bytes.TrimSuffix(replacement, []byte("\n"))
		} else if replacement[len(replacement)-1] != '\n' {
			replacement = append(replacement, '\n')
		}
		firstNewline := bytes.IndexByte(lines, '\n')
		if firstNewline > 0 && lines[firstNewline-1] == '\r' || firstNewline == -1 && bytes.HasSuffix(head[:start], []byte("\r\n")) {
			replacement = bytes.ReplaceAll(replacement, []byte("\n"), []byte("\r\n"))
		}
	}
	if !isCodeSuggestionText(replacement) {
		return codeSuggestionEdit{}, rejectCodeSuggestion("unsupported_file")
	}
	if bytes.Equal(lines, replacement) {
		return codeSuggestionEdit{}, rejectCodeSuggestion("no_changes")
	}
	return codeSuggestionEdit{start, end, replacement}, nil
}

func spliceCodeSuggestions(content []byte, edits []codeSuggestionEdit) ([]byte, error) {
	slices.SortFunc(edits, func(first, second codeSuggestionEdit) int {
		return cmp.Compare(first.start, second.start)
	})
	var result bytes.Buffer
	previousEnd := 0
	for _, edit := range edits {
		if edit.start < previousEnd {
			return nil, rejectCodeSuggestion("overlap")
		}
		result.Write(content[previousEnd:edit.start])
		result.Write(edit.replacement)
		previousEnd = edit.end
	}
	result.Write(content[previousEnd:])
	return result.Bytes(), nil
}

func LoadCodeSuggestionStates(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comments []*issues_model.Comment) error {
	type candidate struct {
		comment     *issues_model.Comment
		suggestions []string
	}
	var candidates []candidate
	for _, comment := range comments {
		comment.CodeSuggestion = nil
		if doer != nil && comment.Type == issues_model.CommentTypeCode && comment.IssueID == pr.IssueID && comment.CodeCommitID() != "" {
			if suggestions := markdown.ParseCodeSuggestions(comment.Content); len(suggestions) > 0 {
				candidates = append(candidates, candidate{comment, suggestions})
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	loader, err := openCodeSuggestionLoader(ctx, doer, pr, "")
	if errors.Is(err, util.ErrPermissionDenied) {
		return nil
	} else if err != nil {
		return err
	}
	defer loader.close()

	for _, candidate := range candidates {
		if loader.canPushToPath != nil && !loader.canPushToPath(candidate.comment.TreePath) {
			continue
		}
		_, err := loader.validate(ctx, candidate.comment, candidate.suggestions)
		var state issues_model.CodeSuggestionState
		if rejection, ok := errors.AsType[codeSuggestionRejection](err); ok {
			state.Reason = string(rejection)
		} else if err != nil {
			return err
		} else if err := candidate.comment.LoadPoster(ctx); err != nil {
			return err
		}
		candidate.comment.CodeSuggestion = &state
	}
	return nil
}

// PrepareCodeSuggestionChanges validates suggestions and splices them into the head files, contentVersions optionally pins the comment versions the doer saw
func PrepareCodeSuggestionChanges(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, commentIDs []int64, contentVersions map[int64]int, expectedHead string) (*CodeSuggestionChanges, error) {
	ids := make([]int64, 0, min(len(commentIDs), maxCodeSuggestions))
	seen := make(map[int64]bool)
	for _, id := range commentIDs {
		if seen[id] {
			continue
		}
		if len(ids) == maxCodeSuggestions {
			return nil, util.ErrorWrapTranslatable(util.ErrInvalidArgument, codeSuggestionReasonPrefix+"batch_limit", maxCodeSuggestions)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, util.ErrorWrapTranslatable(util.ErrInvalidArgument, codeSuggestionReasonPrefix+"empty_batch")
	}

	loader, err := openCodeSuggestionLoader(ctx, doer, pr, expectedHead)
	if err != nil {
		return nil, err
	}
	defer loader.close()

	commentsByID, err := issues_model.FindCodeCommentsByIDs(ctx, pr.IssueID, ids)
	if err != nil {
		return nil, err
	}
	changes := &CodeSuggestionChanges{
		HeadCommitID: loader.headCommitID,
		Comments:     make(issues_model.CommentList, 0, len(ids)),
	}
	files := make(map[string][]codeSuggestionEdit)
	for _, id := range ids {
		comment := commentsByID[id]
		if comment == nil {
			return nil, util.ErrNotExist
		}
		if review, err := loader.getReview(ctx, comment); err != nil && !errors.Is(err, util.ErrNotExist) {
			return nil, err
		} else if review != nil && review.Type == issues_model.ReviewTypePending && review.ReviewerID != doer.ID {
			return nil, util.ErrNotExist // other users' pending reviews are invisible
		}
		if loader.canPushToPath != nil && !loader.canPushToPath(comment.TreePath) {
			return nil, util.ErrPermissionDenied
		}
		if version, ok := contentVersions[id]; ok && version != comment.ContentVersion {
			return nil, rejectCodeSuggestion("edited")
		}
		suggestions := markdown.ParseCodeSuggestions(comment.Content)
		if len(suggestions) == 0 || comment.CodeCommitID() == "" {
			return nil, rejectCodeSuggestion("missing_suggestion")
		}
		selection, err := loader.validate(ctx, comment, suggestions)
		if err != nil {
			return nil, err
		}
		files[comment.TreePath] = append(files[comment.TreePath], selection.edit)
		changes.Comments = append(changes.Comments, comment)
		changes.Conversations = append(changes.Conversations, selection.conversation)
	}
	if err := changes.Comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	for _, path := range slices.Sorted(maps.Keys(files)) {
		blob, err := loader.getBlob(ctx, codeSuggestionPath{loader.headRepo, loader.headCommitID, path})
		if err != nil {
			return nil, err
		}
		content, err := spliceCodeSuggestions(blob.content, files[path])
		if err != nil {
			return nil, err
		}
		changes.Files = append(changes.Files, CodeSuggestionFileChange{Path: path, BlobID: blob.id, Content: content})
	}
	return changes, nil
}
