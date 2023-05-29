// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/google/licensecheck"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Copy paste from models/repo.go because we cannot import models package
func repoPath(userName, repoName string) string {
	return filepath.Join(userPath(userName), strings.ToLower(repoName)+".git")
}

func userPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

func findLicenseFile(gitRepo *git.Repository, branchName string) (string, *git.TreeEntry, error) {
	if branchName == "" {
		return "", nil, nil
	}
	if gitRepo == nil {
		return "", nil, nil
	}

	commit, err := gitRepo.GetBranchCommit(branchName)
	if err != nil {
		if git.IsErrNotExist(err) {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("GetBranchCommit: %w", err)
	}
	entries, err := commit.ListEntries()
	if err != nil {
		return "", nil, fmt.Errorf("ListEntries: %w", err)
	}
	return repo_module.FindFileInEntries(util.FileTypeLicense, entries, "", "", false)
}

func detectLicense(file *git.TreeEntry) ([]string, error) {
	if file == nil {
		return nil, nil
	}

	// Read license file content
	blob := file.Blob()
	contentBuf, err := blob.GetBlobAll()
	if err != nil {
		return nil, fmt.Errorf("GetBlobAll: %w", err)
	}

	// check license
	var licenses []string
	cov := licensecheck.Scan(contentBuf)
	for _, m := range cov.Match {
		licenses = append(licenses, m.ID)
	}

	return licenses, nil
}

func AddRepositoryLicenses(x *xorm.Engine) error {
	type Repository struct {
		ID            int64 `xorm:"pk autoincr"`
		OwnerName     string
		Name          string `xorm:"INDEX NOT NULL"`
		DefaultBranch string
		Licenses      []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	repos := make([]*Repository, 0)
	if err := sess.Where(builder.IsNull{"licenses"}).Find(&repos); err != nil {
		return err
	}

	for _, repo := range repos {
		gitRepo, err := git.OpenRepository(git.DefaultContext, repoPath(repo.OwnerName, repo.Name))
		if err != nil {
			log.Error("Error whilst opening git repo for [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
		_, licenseFile, err := findLicenseFile(gitRepo, repo.DefaultBranch)
		if err != nil {
			log.Error("Error whilst finding license file in [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
		repo.Licenses, err = detectLicense(licenseFile)
		if err != nil {
			log.Error("Error whilst detecting license from %s in [%d]%s/%s. Error: %v", licenseFile.Name(), repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
		if _, err := sess.ID(repo.ID).Cols("licenses").NoAutoTime().Update(repo); err != nil {
			log.Error("Error whilst updating [%d]%s/%s licenses column. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
	}
	return sess.Commit()
}
