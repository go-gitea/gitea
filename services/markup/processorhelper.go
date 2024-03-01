// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"fmt"
	"html/template"
	"io"

	"code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/translation"
	gitea_context "code.gitea.io/gitea/services/context"
	file_service "code.gitea.io/gitea/services/repository/files"
)

func ProcessorHelper() *markup.ProcessorHelper {
	return &markup.ProcessorHelper{
		ElementDir: "auto", // set dir="auto" for necessary (eg: <p>, <h?>, etc) tags
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			mentionedUser, err := user.GetUserByName(ctx, username)
			if err != nil {
				return false
			}

			giteaCtx, ok := ctx.(*gitea_context.Context)
			if !ok {
				// when using general context, use user's visibility to check
				return mentionedUser.Visibility.IsPublic()
			}

			// when using gitea context (web context), use user's visibility and user's permission to check
			return user.IsUserVisibleToViewer(giteaCtx, mentionedUser, giteaCtx.Doer)
		},
		GetRepoFileContent: func(ctx context.Context, ownerName, repoName, commitSha, filePath string) ([]template.HTML, error) {
			repo, err := repo.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
			if err != nil {
				return nil, err
			}

			var user *user.User

			giteaCtx, ok := ctx.(*gitea_context.Context)
			if ok {
				user = giteaCtx.Doer
			}

			perms, err := access.GetUserRepoPermission(ctx, repo, user)
			if err != nil {
				return nil, err
			}
			if !perms.CanRead(unit.TypeCode) {
				return nil, fmt.Errorf("cannot access repository code")
			}

			gitRepo, err := gitrepo.OpenRepository(ctx, repo)
			if err != nil {
				return nil, err
			}

			commit, err := gitRepo.GetCommit(commitSha)
			if err != nil {
				return nil, err
			}

			language, err := file_service.TryGetContentLanguage(gitRepo, commitSha, filePath)
			if err != nil {
				log.Error("Unable to get file language for %-v:%s. Error: %v", repo, filePath, err)
			}

			blob, err := commit.GetBlobByPath(filePath)
			if err != nil {
				return nil, err
			}

			dataRc, err := blob.DataAsync()
			if err != nil {
				return nil, err
			}
			defer dataRc.Close()

			buf, _ := io.ReadAll(dataRc)

			fileContent, _, err := highlight.File(blob.Name(), language, buf)
			if err != nil {
				log.Error("highlight.File failed, fallback to plain text: %v", err)
				fileContent = highlight.PlainText(buf)
			}

			return fileContent, nil
		},
		GetLocale: func(ctx context.Context) (translation.Locale, error) {
			giteaCtx, ok := ctx.(*gitea_context.Context)
			if ok {
				return giteaCtx.Locale, nil
			}

			giteaBaseCtx, ok := ctx.(*gitea_context.Base)
			if ok {
				return giteaBaseCtx.Locale, nil
			}

			return nil, fmt.Errorf("could not retrieve locale from context")
		},
	}
}
