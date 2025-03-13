// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	gotemplate "html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"
)

type blameRow struct {
	RowNumber      int
	Avatar         gotemplate.HTML
	RepoLink       string
	PartSha        string
	PreviousSha    string
	PreviousShaURL string
	IsFirstCommit  bool
	CommitURL      string
	CommitMessage  string
	CommitSince    gotemplate.HTML
	Code           gotemplate.HTML
	EscapeStatus   *charset.EscapeStatus
}

// RefBlame render blame page
func RefBlame(ctx *context.Context) {
	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		ctx.NotFound(nil)
		return
	}

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.RefTypeNameSubURL()

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	var treeNames []string
	paths := make([]string, 0, 5)
	if len(ctx.Repo.TreePath) > 0 {
		treeNames = strings.Split(ctx.Repo.TreePath, "/")
		for i := range treeNames {
			paths = append(paths, strings.Join(treeNames[:i+1], "/"))
		}

		ctx.Data["HasParentPath"] = true
		if len(paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + paths[len(paths)-1]
		}
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "Repo.Commit.GetTreeEntryByPath", err)
		return
	}

	blob := entry.Blob()

	ctx.Data["Paths"] = paths
	ctx.Data["TreeLink"] = treeLink
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink

	ctx.Data["RawFileLink"] = rawLink + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	ctx.Data["PageIsViewCode"] = true

	ctx.Data["IsBlame"] = true

	fileSize := blob.Size()
	ctx.Data["FileSize"] = fileSize
	ctx.Data["FileName"] = blob.Name()

	if fileSize >= setting.UI.MaxDisplayFileSize {
		ctx.Data["IsFileTooLarge"] = true
		ctx.HTML(http.StatusOK, tplRepoHome)
		return
	}

	ctx.Data["NumLines"], err = blob.GetBlobLineCount(nil)
	if err != nil {
		ctx.NotFound(err)
		return
	}

	bypassBlameIgnore, _ := strconv.ParseBool(ctx.FormString("bypass-blame-ignore"))

	result, err := performBlame(ctx, ctx.Repo.Repository.RepoPath(), ctx.Repo.Commit, fileName, bypassBlameIgnore)
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

	ctx.HTML(http.StatusOK, tplRepoHome)
}

type blameResult struct {
	Parts                []*git.BlamePart
	UsesIgnoreRevs       bool
	FaultyIgnoreRevsFile bool
}

func performBlame(ctx *context.Context, repoPath string, commit *git.Commit, file string, bypassBlameIgnore bool) (*blameResult, error) {
	objectFormat := ctx.Repo.GetObjectFormat()

	blameReader, err := git.CreateBlameReader(ctx, objectFormat, repoPath, commit, file, bypassBlameIgnore)
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

			blameReader, err = git.CreateBlameReader(ctx, objectFormat, repoPath, commit, file, true)
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

func fillBlameResult(br *git.BlameReader, r *blameResult) error {
	r.UsesIgnoreRevs = br.UsesIgnoreRevs()

	previousHelper := make(map[string]*git.BlamePart)

	r.Parts = make([]*git.BlamePart, 0, 5)
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

func processBlameParts(ctx *context.Context, blameParts []*git.BlamePart) map[string]*user_model.UserCommit {
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

func renderBlame(ctx *context.Context, blameParts []*git.BlamePart, commitNames map[string]*user_model.UserCommit) {
	repoLink := ctx.Repo.RepoLink

	language, err := files_service.TryGetContentLanguage(ctx.Repo.GitRepo, ctx.Repo.CommitID, ctx.Repo.TreePath)
	if err != nil {
		log.Error("Unable to get file language for %-v:%s. Error: %v", ctx.Repo.Repository, ctx.Repo.TreePath, err)
	}

	lines := make([]string, 0)
	rows := make([]*blameRow, 0)
	escapeStatus := &charset.EscapeStatus{}

	var lexerName string

	avatarUtils := templates.NewAvatarUtils(ctx)
	i := 0
	commitCnt := 0
	for _, part := range blameParts {
		for index, line := range part.Lines {
			i++
			lines = append(lines, line)

			br := &blameRow{
				RowNumber: i,
			}

			commit := commitNames[part.Sha]
			if index == 0 {
				// Count commit number
				commitCnt++

				// User avatar image
				commitSince := templates.TimeSince(commit.Author.When)

				var avatar string
				if commit.User != nil {
					avatar = string(avatarUtils.Avatar(commit.User, 18))
				} else {
					avatar = string(avatarUtils.AvatarByEmail(commit.Author.Email, commit.Author.Name, 18, "tw-mr-2"))
				}

				br.Avatar = gotemplate.HTML(avatar)
				br.RepoLink = repoLink
				br.PartSha = part.Sha
				br.PreviousSha = part.PreviousSha
				br.PreviousShaURL = fmt.Sprintf("%s/blame/commit/%s/%s", repoLink, url.PathEscape(part.PreviousSha), util.PathEscapeSegments(part.PreviousPath))
				br.CommitURL = fmt.Sprintf("%s/commit/%s", repoLink, url.PathEscape(part.Sha))
				br.CommitMessage = commit.CommitMessage
				br.CommitSince = commitSince
			}

			if i != len(lines)-1 {
				line += "\n"
			}
			fileName := fmt.Sprintf("%v", ctx.Data["FileName"])
			line, lexerNameForLine := highlight.Code(fileName, language, line)

			// set lexer name to the first detected lexer. this is certainly suboptimal and
			// we should instead highlight the whole file at once
			if lexerName == "" {
				lexerName = lexerNameForLine
			}

			br.EscapeStatus, br.Code = charset.EscapeControlHTML(line, ctx.Locale)
			rows = append(rows, br)
			escapeStatus = escapeStatus.Or(br.EscapeStatus)
		}
	}

	ctx.Data["EscapeStatus"] = escapeStatus
	ctx.Data["BlameRows"] = rows
	ctx.Data["CommitCnt"] = commitCnt
	ctx.Data["LexerName"] = lexerName
}
