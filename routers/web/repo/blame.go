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

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/languagestats"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

type blameRow struct {
	RowNumber int

	Avatar         template.HTML
	PreviousSha    string
	PreviousShaURL string
	CommitURL      string
	CommitMessage  string
	CommitSince    template.HTML

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

func processBlameParts(ctx *context.Context, blameParts []*gitrepo.BlamePart) map[string]*user_model.UserCommit {
	// store commit data by SHA to look up avatar info etc
	commitNames := make(map[string]*user_model.UserCommit)
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
	validatedCommits, err := user_model.ValidateCommitsWithEmails(ctx, commits)
	if err != nil {
		ctx.ServerError("ValidateCommitsWithEmails", err)
		return nil
	}
	for _, c := range validatedCommits {
		commitNames[c.ID.String()] = c
	}

	return commitNames
}

func renderBlameFillFirstBlameRow(repoLink string, avatarUtils *templates.AvatarUtils, part *gitrepo.BlamePart, commit *user_model.UserCommit, br *blameRow) {
	if commit.User != nil {
		br.Avatar = avatarUtils.Avatar(commit.User, 18)
	} else {
		br.Avatar = avatarUtils.AvatarByEmail(commit.Author.Email, commit.Author.Name, 18)
	}

	br.PreviousSha = part.PreviousSha
	br.PreviousShaURL = fmt.Sprintf("%s/blame/commit/%s/%s", repoLink, url.PathEscape(part.PreviousSha), util.PathEscapeSegments(part.PreviousPath))
	br.CommitURL = fmt.Sprintf("%s/commit/%s", repoLink, url.PathEscape(part.Sha))
	br.CommitMessage = commit.CommitMessage
	br.CommitSince = templates.TimeSince(commit.Author.When)
}

func renderBlame(ctx *context.Context, blameParts []*gitrepo.BlamePart, commitNames map[string]*user_model.UserCommit) {
	language, err := languagestats.GetFileLanguage(ctx, ctx.Repo.GitRepo, ctx.Repo.CommitID, ctx.Repo.TreePath)
	if err != nil {
		log.Error("Unable to get file language for %-v:%s. Error: %v", ctx.Repo.Repository, ctx.Repo.TreePath, err)
	}

	buf := &bytes.Buffer{}
	rows := make([]*blameRow, 0)
	avatarUtils := templates.NewAvatarUtils(ctx)
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
				renderBlameFillFirstBlameRow(ctx.Repo.RepoLink, avatarUtils, part, commitNames[part.Sha], br)
			}
		}
	}

	escapeStatus := &charset.EscapeStatus{}

	bufContent := buf.Bytes()
	bufContent = charset.ToUTF8(bufContent, charset.ConvertOpts{})
	highlighted, lexerName := highlight.Code(path.Base(ctx.Repo.TreePath), language, util.UnsafeBytesToString(bufContent))
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
	ctx.Data["LexerName"] = lexerName
}
