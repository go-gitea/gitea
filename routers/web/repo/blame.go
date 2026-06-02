// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"gitea.dev/models/gituser"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/languagestats"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/highlight"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

type blameRow struct {
	RowNumber int

	PreviousSha    string
	PreviousShaURL string
	CommitURL      string
	CommitMessage  string
	CommitSince    template.HTML

	AvatarStackData *gituser.AvatarStackData

	Code         template.HTML
	EscapeStatus *charset.EscapeStatus
}

// RefBlame render blame page
func RefBlame(ctx *context.Context) {
	ctx.Data["IsBlame"] = true
	prepareRepoViewContent(ctx, ctx.Repo.RefTypeNameSubURL())

	// Get current entry user currently looking at.
	if ctx.Repo.TreePath == "" {
		ctx.NotFound(nil)
		return
	}
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "Repo.Commit.GetTreeEntryByPath", err)
		return
	}

	blob := entry.Blob()
	fileSize := blob.Size()
	ctx.Data["FileSize"] = fileSize
	ctx.Data["FileTreePath"] = ctx.Repo.TreePath

	tplName := tplRepoViewContent
	if !ctx.FormBool("only_content") {
		prepareHomeTreeSideBarSwitch(ctx)
		tplName = tplRepoView
	}

	if fileSize >= setting.UI.MaxDisplayFileSize {
		ctx.Data["IsFileTooLarge"] = true
		ctx.HTML(http.StatusOK, tplName)
		return
	}

	ctx.Data["NumLines"], err = blob.GetBlobLineCount(nil)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	bypassBlameIgnore, _ := strconv.ParseBool(ctx.FormString("bypass-blame-ignore"))
	result, err := performBlame(ctx, ctx.Repo.Repository, ctx.Repo.Commit, ctx.Repo.TreePath, bypassBlameIgnore)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	ctx.Data["UsesIgnoreRevs"] = result.UsesIgnoreRevs
	ctx.Data["FaultyIgnoreRevsFile"] = result.FaultyIgnoreRevsFile

	commitNames := processBlameParts(ctx, result.Parts)
	if ctx.Written() {
		return
	}

	renderBlame(ctx, result.Parts, commitNames)

	ctx.HTML(http.StatusOK, tplName)
}

type blameResult struct {
	Parts                []*gitrepo.BlamePart
	UsesIgnoreRevs       bool
	FaultyIgnoreRevsFile bool
}

func performBlame(ctx *context.Context, repo *repo_model.Repository, commit *git.Commit, file string, bypassBlameIgnore bool) (*blameResult, error) {
	objectFormat := ctx.Repo.GetObjectFormat()

	blameReader, err := gitrepo.CreateBlameReader(ctx, objectFormat, repo, commit, file, bypassBlameIgnore)
	if err != nil {
		return nil, err
	}

	r := &blameResult{}
	if err := fillBlameResult(blameReader, r); err != nil {
		_ = blameReader.Close()
		return nil, err
	}

	err = blameReader.Close()
	if err != nil {
		if len(r.Parts) == 0 && r.UsesIgnoreRevs {
			// try again without ignored revs

			blameReader, err = gitrepo.CreateBlameReader(ctx, objectFormat, repo, commit, file, true)
			if err != nil {
				return nil, err
			}

			r := &blameResult{
				FaultyIgnoreRevsFile: true,
			}
			if err := fillBlameResult(blameReader, r); err != nil {
				_ = blameReader.Close()
				return nil, err
			}

			return r, blameReader.Close()
		}
		return nil, err
	}
	return r, nil
}

func fillBlameResult(br *gitrepo.BlameReader, r *blameResult) error {
	r.UsesIgnoreRevs = br.UsesIgnoreRevs()

	previousHelper := make(map[string]*gitrepo.BlamePart)

	r.Parts = make([]*gitrepo.BlamePart, 0, 5)
	for {
		blamePart, err := br.NextPart()
		if err != nil {
			return fmt.Errorf("BlameReader.NextPart failed: %w", err)
		}
		if blamePart == nil {
			break
		}

		if prev, ok := previousHelper[blamePart.Sha]; ok {
			if blamePart.PreviousSha == "" {
				blamePart.PreviousSha = prev.PreviousSha
				blamePart.PreviousPath = prev.PreviousPath
			}
		} else {
			previousHelper[blamePart.Sha] = blamePart
		}

		r.Parts = append(r.Parts, blamePart)
	}

	return nil
}

func processBlameParts(ctx *context.Context, blameParts []*gitrepo.BlamePart) map[string]*gituser.UserCommit {
	// store commit data by SHA to look up avatar info etc
	commitNames := make(map[string]*gituser.UserCommit)
	// and as blameParts can reference the same commits multiple
	// times, we cache the lookup work locally
	commits := make([]*git.Commit, 0, len(blameParts))
	commitCache := map[string]*git.Commit{}
	commitCache[ctx.Repo.Commit.ID.String()] = ctx.Repo.Commit

	for _, part := range blameParts {
		sha := part.Sha
		if _, ok := commitNames[sha]; ok {
			continue
		}

		// find the blamePart commit, to look up parent & email address for avatars
		commit, ok := commitCache[sha]
		var err error
		if !ok {
			commit, err = ctx.Repo.GitRepo.GetCommit(sha)
			if err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound(err)
				} else {
					ctx.ServerError("Repo.GitRepo.GetCommit", err)
				}
				return nil
			}
			commitCache[sha] = commit
		}

		commits = append(commits, commit)
	}

	// populate commit email addresses to later look up avatars.
	userCommits, err := gituser.GetUserCommitsByGitCommits(ctx, commits, ctx.Repo.RepoLink, ctx.Repo.RefFullName)
	if err != nil {
		ctx.ServerError("GetUserCommitsByGitCommits", err)
		return nil
	}
	for _, c := range userCommits {
		commitNames[c.GitCommit.ID.String()] = c
	}

	return commitNames
}

func renderBlameFillFirstBlameRow(ctx *context.Context, repoLink string, part *gitrepo.BlamePart, commit *gituser.UserCommit, br *blameRow) {
	br.AvatarStackData = gituser.BuildAvatarStackData(ctx, commit.GitCommit.AllParticipantIdentities(), nil)
	br.PreviousSha = part.PreviousSha
	br.PreviousShaURL = fmt.Sprintf("%s/blame/commit/%s/%s", repoLink, url.PathEscape(part.PreviousSha), util.PathEscapeSegments(part.PreviousPath))
	br.CommitURL = fmt.Sprintf("%s/commit/%s", repoLink, url.PathEscape(part.Sha))
	br.CommitMessage = commit.GitCommit.MessageUTF8()
	br.CommitSince = templates.TimeSince(commit.GitCommit.Author.When)
}

func renderBlame(ctx *context.Context, blameParts []*gitrepo.BlamePart, commitNames map[string]*gituser.UserCommit) {
	language, err := languagestats.GetFileLanguage(ctx, ctx.Repo.GitRepo, ctx.Repo.CommitID, ctx.Repo.TreePath)
	if err != nil {
		log.Error("Unable to get file language for %-v:%s. Error: %v", ctx.Repo.Repository, ctx.Repo.TreePath, err)
	}

	buf := &bytes.Buffer{}
	rows := make([]*blameRow, 0)
	rowNumber := 0 // will be 1-based
	for _, part := range blameParts {
		for partLineIdx, line := range part.Lines {
			rowNumber++

			br := &blameRow{RowNumber: rowNumber}
			rows = append(rows, br)

			if int64(buf.Len()) < setting.UI.MaxDisplayFileSize {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}

			if partLineIdx == 0 {
				renderBlameFillFirstBlameRow(ctx, ctx.Repo.RepoLink, part, commitNames[part.Sha], br)
			}
		}
	}

	escapeStatus := &charset.EscapeStatus{}

	bufContent := buf.Bytes()
	bufContent = charset.ToUTF8(bufContent, charset.ConvertOpts{})
	highlighted, _, lexerDisplayName := highlight.RenderCodeSlowGuess(path.Base(ctx.Repo.TreePath), language, util.UnsafeBytesToString(bufContent))
	unsafeLines := highlight.UnsafeSplitHighlightedLines(highlighted)
	for i, br := range rows {
		var line template.HTML
		if i < len(unsafeLines) {
			line = template.HTML(util.UnsafeBytesToString(unsafeLines[i]))
		}
		br.EscapeStatus, br.Code = charset.EscapeControlHTML(line, ctx.Locale)
		escapeStatus = escapeStatus.Or(br.EscapeStatus)
	}

	ctx.Data["EscapeStatus"] = escapeStatus
	ctx.Data["BlameRows"] = rows
	ctx.Data["LexerName"] = lexerDisplayName
}
