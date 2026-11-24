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
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
)

const CodeSuggestionsBatchCommitMessage = "Apply batched suggestions from code review"

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

type CodeSuggestionVersion struct {
	CommentID      int64
	ContentVersion int
}

type CodeSuggestionFileChange struct {
	Path    string
	BlobID  string
	Content []byte
}

type CodeSuggestionChanges struct {
	Files         []CodeSuggestionFileChange
	Comments      issues_model.CommentList
	Conversations issues_model.CommentList
}

type codeSuggestionConversation struct {
	path string
	line int64
}

type codeSuggestionPath struct {
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
	headCloser    io.Closer
	headCommitID  string
	canPushToPath func(string) bool
	cache         *cache.EphemeralCache
}

func openCodeSuggestionLoader(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) (_ *codeSuggestionLoader, errRet error) {
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
	if !issues_model.CanMaintainerWriteToBranch(ctx, permission, pr.HeadBranch, doer) {
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
		return nil, ErrCodeSuggestionHeadChanged
	} else if err != nil {
		return nil, err
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
			if _, _, _, err := asymkey_service.SignCRUDAction(ctx, doer, headRepo, pr.HeadBranch); asymkey_service.IsErrWontSign(err) {
				return nil, util.ErrPermissionDenied
			} else if err != nil {
				return nil, err
			}
		}
		canPush := protectedBranch.CanUserPush(ctx, doer, permission)
		protectedPaths, unprotectedPaths := protectedBranch.GetProtectedFilePatterns(), protectedBranch.GetUnprotectedFilePatterns()
		loader.canPushToPath = func(path string) bool {
			return (canPush || protectedBranch.IsUnprotectedFile(unprotectedPaths, path)) && !protectedBranch.IsProtectedFile(protectedPaths, path)
		}
	}
	return loader, nil
}

func (loader *codeSuggestionLoader) getBlob(ctx context.Context, key codeSuggestionPath) (codeSuggestionBlob, error) {
	blob, err := cache.GetWithEphemeralCache(ctx, loader.cache, "blob", key, loader.readBlob)
	return blob, cmp.Or(err, blob.rejection)
}

func (loader *codeSuggestionLoader) readBlob(ctx context.Context, key codeSuggestionPath) (codeSuggestionBlob, error) {
	commit, err := loader.headRepo.GetCommit(ctx, key.commitID)
	var entry *git.TreeEntry
	if err == nil {
		entry, err = commit.GetTreeEntryByPath(ctx, loader.headRepo, key.path)
	}
	switch {
	case errors.Is(err, util.ErrNotExist):
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("outdated")}, nil
	case err != nil:
		return codeSuggestionBlob{}, err
	case !entry.IsRegular() && !entry.IsExecutable(), entry.GetSize(ctx, loader.headRepo) > setting.UI.MaxDisplayFileSize:
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("unsupported_file")}, nil
	}

	content, err := entry.Blob(loader.headRepo).GetBlobBytes(ctx, setting.UI.MaxDisplayFileSize+1)
	if err != nil {
		return codeSuggestionBlob{}, err
	}
	_, pointerErr := lfs.ReadPointerFromBuffer(content)
	if !isCodeSuggestionText(content) || pointerErr == nil {
		return codeSuggestionBlob{rejection: rejectCodeSuggestion("unsupported_file")}, nil
	}
	return codeSuggestionBlob{id: entry.ID.String(), content: content}, nil
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
	switch {
	case loader.pr.HasMerged || loader.pr.Issue.IsClosed:
		return codeSuggestionSelection{}, rejectCodeSuggestion("closed")
	case comment.Line <= 0 || comment.CodeStartLine() <= 0:
		return codeSuggestionSelection{}, rejectCodeSuggestion("deleted_line")
	case len(suggestions) != 1:
		return codeSuggestionSelection{}, rejectCodeSuggestion("multiple")
	case comment.ReviewID == 0 || comment.Invalidated:
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	review, err := loader.getReview(ctx, comment)
	if errors.Is(err, util.ErrNotExist) || err == nil && review.IssueID != loader.pr.IssueID {
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	} else if err != nil {
		return codeSuggestionSelection{}, err
	}
	switch review.Type {
	case issues_model.ReviewTypePending:
		return codeSuggestionSelection{}, rejectCodeSuggestion("pending")
	case issues_model.ReviewTypeApprove, issues_model.ReviewTypeComment, issues_model.ReviewTypeReject:
	default:
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	}
	first, err := cache.GetWithEphemeralCache(ctx, loader.cache, "conversation", codeSuggestionConversation{comment.TreePath, comment.Line},
		func(ctx context.Context, key codeSuggestionConversation) (*issues_model.Comment, error) {
			return issues_model.GetPublishedCodeConversationRoot(ctx, loader.pr.IssueID, key.path, key.line)
		})
	switch {
	case errors.Is(err, util.ErrNotExist):
		return codeSuggestionSelection{}, rejectCodeSuggestion("outdated")
	case err != nil:
		return codeSuggestionSelection{}, err
	case first.IsResolved():
		return codeSuggestionSelection{}, rejectCodeSuggestion("resolved")
	}

	edit, err := loader.validateEdit(ctx, comment, suggestions[0])
	return codeSuggestionSelection{edit: edit, conversation: first}, err
}

func (loader *codeSuggestionLoader) validateEdit(ctx context.Context, comment *issues_model.Comment, suggestion string) (codeSuggestionEdit, error) {
	anchor := codeSuggestionPath{comment.CodeCommitID(), comment.TreePath}
	original, err := loader.getBlob(ctx, anchor)
	if err != nil {
		return codeSuggestionEdit{}, err
	}
	blob, err := loader.getBlob(ctx, codeSuggestionPath{loader.headCommitID, comment.TreePath})
	if err != nil {
		return codeSuggestionEdit{}, err
	}
	if original.id != blob.id {
		hunks, err := cache.GetWithEphemeralCache(ctx, loader.cache, "diffHunks", anchor,
			func(ctx context.Context, key codeSuggestionPath) ([]git.FileDiffHunk, error) {
				return git.GetFileDiffHunks(ctx, loader.headRepo, key.commitID, loader.headCommitID, key.path)
			})
		if err != nil {
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
	if endLine < startLine {
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
	var result []byte
	previousEnd := 0
	for _, edit := range edits {
		if edit.start < previousEnd {
			return nil, rejectCodeSuggestion("overlap")
		}
		result = append(result, content[previousEnd:edit.start]...)
		result = append(result, edit.replacement...)
		previousEnd = edit.end
	}
	return append(result, content[previousEnd:]...), nil
}

func LoadCodeSuggestionStates(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, comments []*issues_model.Comment) {
	if doer == nil || !issue.IsPull {
		return
	}

	var loader *codeSuggestionLoader
	for _, comment := range comments {
		if comment.Type != issues_model.CommentTypeCode || comment.IssueID != issue.ID || comment.CodeCommitID() == "" {
			continue
		}
		suggestions := markdown.ParseCodeSuggestions(comment.Content)
		if len(suggestions) == 0 {
			continue
		}
		if issue.IsClosed {
			comment.CodeSuggestion = &issues_model.CodeSuggestionState{Reason: codeSuggestionReasonPrefix + "closed"}
			continue
		}
		if loader == nil {
			if err := issue.LoadPullRequest(ctx); err != nil {
				log.Error("LoadCodeSuggestionStates: load pull request for issue %d: %v", issue.ID, err)
				return
			}
			var err error
			loader, err = openCodeSuggestionLoader(ctx, doer, issue.PullRequest)
			if err != nil {
				if !errors.Is(err, util.ErrPermissionDenied) && !errors.Is(err, ErrCodeSuggestionHeadChanged) {
					log.Error("LoadCodeSuggestionStates: open loader for pull request %d: %v", issue.PullRequest.ID, err)
				}
				return
			}
			defer loader.headCloser.Close()
		}
		if loader.canPushToPath != nil && !loader.canPushToPath(comment.TreePath) {
			continue
		}
		_, err := loader.validate(ctx, comment, suggestions)
		var state issues_model.CodeSuggestionState
		if rejection, ok := errors.AsType[codeSuggestionRejection](err); ok {
			state.Reason = string(rejection)
		} else if err != nil {
			log.Error("LoadCodeSuggestionStates: validate comment %d: %v", comment.ID, err)
			continue
		} else if err := comment.LoadPoster(ctx); err != nil {
			log.Error("LoadCodeSuggestionStates: load poster for comment %d: %v", comment.ID, err)
			continue
		}
		comment.CodeSuggestion = &state
	}
}

func PrepareCodeSuggestionChanges(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, versions []CodeSuggestionVersion, expectedHead string) (*CodeSuggestionChanges, error) {
	if len(versions) > maxCodeSuggestions {
		return nil, util.ErrorWrapTranslatable(util.ErrInvalidArgument, codeSuggestionReasonPrefix+"batch_limit", maxCodeSuggestions)
	}
	loader, err := openCodeSuggestionLoader(ctx, doer, pr)
	if err != nil {
		return nil, err
	}
	defer loader.headCloser.Close()
	if loader.headCommitID != expectedHead {
		return nil, ErrCodeSuggestionHeadChanged
	}

	ids := make([]int64, len(versions))
	for index, version := range versions {
		ids[index] = version.CommentID
	}
	commentsByID, err := issues_model.FindCodeCommentsByIDs(ctx, pr.IssueID, ids)
	if err != nil {
		return nil, err
	}
	changes := &CodeSuggestionChanges{Comments: make(issues_model.CommentList, 0, len(ids))}
	files := make(map[string][]codeSuggestionEdit)
	for _, version := range versions {
		comment := commentsByID[version.CommentID]
		if comment == nil {
			return nil, util.ErrNotExist
		}
		if comment.Review, err = loader.getReview(ctx, comment); err != nil && !errors.Is(err, util.ErrNotExist) {
			return nil, err
		} else if review := comment.Review; review != nil && review.Type == issues_model.ReviewTypePending && review.ReviewerID != doer.ID {
			return nil, util.ErrNotExist
		}
		if loader.canPushToPath != nil && !loader.canPushToPath(comment.TreePath) {
			return nil, util.ErrPermissionDenied
		}
		if version.ContentVersion != comment.ContentVersion {
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
		blob, _ := loader.getBlob(ctx, codeSuggestionPath{loader.headCommitID, path})
		content, err := spliceCodeSuggestions(blob.content, files[path])
		if err != nil {
			return nil, err
		}
		changes.Files = append(changes.Files, CodeSuggestionFileChange{Path: path, BlobID: blob.id, Content: content})
	}
	return changes, nil
}
