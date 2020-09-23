// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	repo_service "code.gitea.io/gitea/services/repository"
	"github.com/unknwon/com"
)

const (
	tplRepos          base.TplName = "admin/repo/list"
	tplUnadoptedRepos base.TplName = "admin/repo/unadopted"
)

// Repos show all the repositories
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRepositories"] = true

	routers.RenderRepoSearch(ctx, &routers.RepoSearchOptions{
		Private:  true,
		PageSize: setting.UI.Admin.RepoPagingNum,
		TplName:  tplRepos,
	})
}

// DeleteRepo delete one repository
func DeleteRepo(ctx *context.Context) {
	repo, err := models.GetRepositoryByID(ctx.QueryInt64("id"))
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	if err := repo_service.DeleteRepository(ctx.User, repo); err != nil {
		ctx.ServerError("DeleteRepository", err)
		return
	}
	log.Trace("Repository deleted: %s", repo.FullName())

	ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/repos?page=" + ctx.Query("page") + "&sort=" + ctx.Query("sort"),
	})
}

// UnadoptedRepos lists the unadopted repositories
func UnadoptedRepos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.repositories")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminRepositories"] = true

	opts := models.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.QueryInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	repoNamesToCheck := make([]string, 0, opts.PageSize)

	repoNames := make([]string, 0, opts.PageSize)
	var ctxUser *models.User

	count := 0

	// We're going to iterate by pagesize.
	root := filepath.Join(setting.RepoRootPath)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() || path == root {
			return nil
		}

		if !strings.ContainsRune(path[len(root)+1:], filepath.Separator) {
			// Got a new user

			// Clean up old repoNamesToCheck
			if len(repoNamesToCheck) > 0 {
				repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
					Page:     1,
					PageSize: opts.PageSize,
				}, LowerNames: repoNamesToCheck})
				if err != nil {
					return err
				}
				for _, name := range repoNamesToCheck {
					found := false
				repoLoopCatchup:
					for i, repo := range repos {
						if repo.LowerName == name {
							found = true
							repos = append(repos[:i], repos[i+1:]...)
							break repoLoopCatchup
						}
					}
					if !found {
						if count >= start && count < end {
							repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
						}
						count++
					}
				}
				repoNamesToCheck = repoNamesToCheck[:0]
			}

			ctxUser, err = models.GetUserByName(info.Name())
			if err != nil {
				if models.IsErrUserNotExist(err) {
					log.Debug("Missing user: %s", info.Name())
					return filepath.SkipDir
				}
				return err
			}
			return nil
		}

		name := info.Name()

		if !strings.HasSuffix(name, ".git") {
			return filepath.SkipDir
		}
		name = name[:len(name)-4]
		if models.IsUsableRepoName(name) != nil || strings.ToLower(name) != name {
			return filepath.SkipDir
		}
		if count < end {
			repoNamesToCheck = append(repoNamesToCheck, name)
			if len(repoNamesToCheck) >= opts.PageSize {
				repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
					Page:     1,
					PageSize: opts.PageSize,
				}, LowerNames: repoNamesToCheck})
				if err != nil {
					return err
				}
				for _, name := range repoNamesToCheck {
					found := false
				repoLoop:
					for i, repo := range repos {
						if repo.Name == name {
							found = true
							repos = append(repos[:i], repos[i+1:]...)
							break repoLoop
						}
					}
					if !found {
						if count >= start && count < end {
							repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
						}
						count++
					}
				}
				repoNamesToCheck = repoNamesToCheck[:0]
			}
			return filepath.SkipDir
		}
		count++
		return filepath.SkipDir
	}); err != nil {
		ctx.ServerError("filepath.Walk", err)
		return
	}

	if len(repoNamesToCheck) > 0 {
		repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
			Page:     1,
			PageSize: opts.PageSize,
		}, LowerNames: repoNamesToCheck})
		if err != nil {
			ctx.ServerError("filepath.Walk", err)
			return
		}
		for _, name := range repoNamesToCheck {
			found := false
		repoLoop:
			for i, repo := range repos {
				if repo.LowerName == name {
					found = true
					repos = append(repos[:i], repos[i+1:]...)
					break repoLoop
				}
			}
			if !found {
				if count >= start && count < end {
					repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
				}
				count++
			}
		}
	}
	ctx.Data["Dirs"] = repoNames
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(200, tplUnadoptedRepos)
}

// AdoptOrDeleteRepository adopts or deletes a repository
func AdoptOrDeleteRepository(ctx *context.Context) {
	dir := ctx.Query("id")
	action := ctx.Query("action")
	dirSplit := strings.SplitN(dir, "/", 2)
	if len(dirSplit) != 2 {
		ctx.Redirect(setting.AppSubURL + "/admin/repos")
		return
	}

	ctxUser, err := models.GetUserByName(dirSplit[0])
	if err != nil {
		if models.IsErrUserNotExist(err) {
			log.Debug("User does not exist: %s", dirSplit[0])
			ctx.Redirect(setting.AppSubURL + "/admin/repos")
			return
		}
		ctx.ServerError("GetUserByName", err)
		return
	}

	repoName := dirSplit[1]

	// check not a repo
	if has, err := models.IsRepositoryExist(ctxUser, repoName); err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	} else if has || !com.IsDir(models.RepoPath(ctxUser.Name, repoName)) {
		log.Debug("has: %t, notDir: %s", has, models.RepoPath(ctxUser.Name, repoName))
		// Fallthrough to failure mode
	} else if action == "adopt" {
		if _, err := repository.AdoptRepository(ctx.User, ctxUser, models.CreateRepoOptions{
			Name:      dirSplit[1],
			IsPrivate: true,
		}); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.adopt_preexisting_success", dir))
	} else if action == "delete" {
		if err := repository.DeleteUnadoptedRepository(ctx.User, ctxUser, dirSplit[1]); err != nil {
			ctx.ServerError("repository.AdoptRepository", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.delete_preexisting_success", dir))
	}
	ctx.Redirect(setting.AppSubURL + "/admin/repos/unadopted")
}
