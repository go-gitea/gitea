// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	licenseclassifier "github.com/google/licenseclassifier/v2"
	"xorm.io/xorm"
)

var (
	classifier     *licenseclassifier.Classifier
	licenseAliases map[string]string
)

// Copy paste from models/repo.go because we cannot import models package
func repoPath(userName, repoName string) string {
	return filepath.Join(userPath(userName), strings.ToLower(repoName)+".git")
}

func userPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

// Copy paste from modules/repository/file.go because we cannot import models package
func findLicenseFile(commit *git.Commit) (string, *git.TreeEntry, error) {
	if commit == nil {
		return "", nil, nil
	}
	entries, err := commit.ListEntries()
	if err != nil {
		return "", nil, fmt.Errorf("ListEntries: %w", err)
	}
	return findFileInEntries(util.FileTypeLicense, entries, "", "", false)
}

func findFileInEntries(fileType util.FileType, entries []*git.TreeEntry, treePath, language string, tryWellKnownDirs bool) (string, *git.TreeEntry, error) {
	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localisation - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", language), ".txt", "") // sorted by priority
	extCount := len(exts)
	targetFiles := make([]*git.TreeEntry, extCount+1)

	docsEntries := make([]*git.TreeEntry, 3) // (one of docs/, .gitea/ or .github/)
	for _, entry := range entries {
		if tryWellKnownDirs && entry.IsDir() {
			// as a special case for the top-level repo introduction README,
			// fall back to subfolders, looking for e.g. docs/README.md, .gitea/README.zh-CN.txt, .github/README.txt, ...
			// (note that docsEntries is ignored unless we are at the root)
			lowerName := strings.ToLower(entry.Name())
			switch lowerName {
			case "docs":
				if entry.Name() == "docs" || docsEntries[0] == nil {
					docsEntries[0] = entry
				}
			case ".gitea":
				if entry.Name() == ".gitea" || docsEntries[1] == nil {
					docsEntries[1] = entry
				}
			case ".github":
				if entry.Name() == ".github" || docsEntries[2] == nil {
					docsEntries[2] = entry
				}
			}
			continue
		}
		if i, ok := util.IsFileExtension(entry.Name(), fileType, exts...); ok {
			log.Debug("Potential %s file: %s", fileType, entry.Name())
			if targetFiles[i] == nil || base.NaturalSortLess(targetFiles[i].Name(), entry.Blob().Name()) {
				if entry.IsLink() {
					target, err := entry.FollowLinks()
					if err != nil && !git.IsErrBadLink(err) {
						return "", nil, err
					} else if target != nil && (target.IsExecutable() || target.IsRegular()) {
						targetFiles[i] = entry
					}
				} else {
					targetFiles[i] = entry
				}
			}
		}
	}
	var targetFile *git.TreeEntry
	for _, f := range targetFiles {
		if f != nil {
			targetFile = f
			break
		}
	}

	if treePath == "" && targetFile == nil {
		for _, subTreeEntry := range docsEntries {
			if subTreeEntry == nil {
				continue
			}
			subTree := subTreeEntry.Tree()
			if subTree == nil {
				// this should be impossible; if subTreeEntry exists so should this.
				continue
			}
			var err error
			childEntries, err := subTree.ListEntries()
			if err != nil {
				return "", nil, err
			}

			subfolder, targetFile, err := findFileInEntries(fileType, childEntries, subTreeEntry.Name(), language, false)
			if err != nil && !git.IsErrNotExist(err) {
				return "", nil, err
			}
			if targetFile != nil {
				return path.Join(subTreeEntry.Name(), subfolder), targetFile, nil
			}
		}
	}

	return "", targetFile, nil
}

func localizedExtensions(ext, languageCode string) (localizedExts []string) {
	if len(languageCode) < 1 {
		return []string{ext}
	}

	lowerLangCode := "." + strings.ToLower(languageCode)

	if strings.Contains(lowerLangCode, "-") {
		underscoreLangCode := strings.ReplaceAll(lowerLangCode, "-", "_")
		indexOfDash := strings.Index(lowerLangCode, "-")
		// e.g. [.zh-cn.md, .zh_cn.md, .zh.md, _zh.md, .md]
		return []string{lowerLangCode + ext, underscoreLangCode + ext, lowerLangCode[:indexOfDash] + ext, "_" + lowerLangCode[1:indexOfDash] + ext, ext}
	}

	// e.g. [.en.md, .md]
	return []string{lowerLangCode + ext, ext}
}

// detectLicenseByEntry returns the licenses detected by the given tree entry
func detectLicenseByEntry(file *git.TreeEntry) ([]string, error) {
	if file == nil {
		return nil, nil
	}

	blob := file.Blob()
	content, err := blob.GetBlobContent(setting.UI.MaxDisplayFileSize)
	if err != nil {
		return nil, fmt.Errorf("GetBlobAll: %w", err)
	}
	return detectLicense(content), nil
}

// detectLicense returns the licenses detected by the given content buff
func detectLicense(content string) []string {
	if len(content) == 0 {
		return nil
	}
	if classifier == nil {
		log.Error("detectLicense: license classifier is null.")
		return nil
	}

	matches, err := classifier.MatchFrom(strings.NewReader(content))
	if err != nil {
		log.Error("licenseclassifier.MatchFrom: %v", err)
		return nil
	}
	if len(matches.Matches) > 0 {
		results := make(container.Set[string], len(matches.Matches))
		for _, r := range matches.Matches {
			if r.MatchType == "License" && !results.Contains(r.Variant) {
				results.Add(r.Variant)
			}
		}
		return results.Values()
	}
	return nil
}

func ConvertLicenseName(name string) string {
	if licenseAliases == nil {
		return name
	}

	v, ok := licenseAliases[name]
	if ok {
		return v
	}
	return name
}

func initClassifier() error {
	data, err := options.AssetFS().ReadFile("", "license-aliases.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &licenseAliases)
	if err != nil {
		return err
	}

	// threshold should be 0.84~0.86 or the test will be failed
	classifier = licenseclassifier.NewClassifier(.85)
	licenseFiles, err := options.AssetFS().ListFiles("license", true)
	if err != nil {
		return err
	}

	licenseNameCount := make(map[string]int)
	if len(licenseFiles) > 0 {
		for _, licenseFile := range licenseFiles {
			data, err := options.License(licenseFile)
			if err != nil {
				return err
			}
			licenseName := ConvertLicenseName(licenseFile)
			licenseNameCount[licenseName]++
			if licenseNameCount[licenseName] > 1 {
				continue
			}
			classifier.AddContent("License", licenseFile, licenseName, data)
		}
	}
	return nil
}

func AddRepositoryLicenses(x *xorm.Engine) error {
	type Repository struct {
		ID            int64 `xorm:"pk autoincr"`
		OwnerName     string
		Name          string `xorm:"INDEX NOT NULL"`
		DefaultBranch string
	}

	type RepoLicense struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CommitID    string
		License     string             `xorm:"VARCHAR(50) UNIQUE(s) INDEX NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX UPDATED"`
	}

	if err := x.Sync(new(RepoLicense)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	repos := make([]*Repository, 0)
	if err := sess.Find(&repos); err != nil {
		return err
	}

	if err := initClassifier(); err != nil {
		return err
	}

	for _, repo := range repos {
		gitRepo, err := git.OpenRepository(git.DefaultContext, repoPath(repo.OwnerName, repo.Name))
		if err != nil {
			log.Error("Error whilst opening git repo for [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			// Allow git repo not exist
			continue
		}
		commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			if git.IsErrNotExist(err) {
				continue
			}
			log.Error("Error whilst getting default branch commit in [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
		_, licenseFile, err := findLicenseFile(commit)
		if err != nil {
			log.Error("Error whilst finding license file in [%d]%s/%s. Error: %v", repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}
		licenses, err := detectLicenseByEntry(licenseFile)
		if err != nil {
			log.Error("Error whilst detecting license from %s in [%d]%s/%s. Error: %v", licenseFile.Name(), repo.ID, repo.OwnerName, repo.Name, err)
			return err
		}

		oldLicenses := make([]RepoLicense, 0)
		if err := sess.Where("`repo_id` = ?", repo.ID).Asc("`license`").Find(&oldLicenses); err != nil {
			return err
		}

		for _, license := range licenses {
			upd := false
			for _, o := range oldLicenses {
				// Update already existing license
				if o.License == license {
					if _, err := sess.ID(o.ID).Cols("`commit_id`").Update(o); err != nil {
						log.Error("Error whilst updating [%d]%s/%s license [%s]. Error: %v", repo.ID, repo.OwnerName, repo.Name, license, err)
						return err
					}
					upd = true
					break
				}
			}
			// Insert new license
			if !upd {
				if _, err := sess.Insert(&RepoLicense{
					RepoID:   repo.ID,
					CommitID: commit.ID.String(),
					License:  license,
				}); err != nil {
					log.Error("Error whilst inserting [%d]%s/%s license [%s]. Error: %v", repo.ID, repo.OwnerName, repo.Name, license, err)
					return err
				}
			}
		}
		// Delete old licenses
		licenseToDelete := make([]int64, 0, len(oldLicenses))
		for _, o := range oldLicenses {
			if o.CommitID != commit.ID.String() {
				licenseToDelete = append(licenseToDelete, o.ID)
			}
		}
		if len(licenseToDelete) > 0 {
			if _, err := sess.In("`id`", licenseToDelete).Delete(&RepoLicense{}); err != nil {
				return err
			}
		}
	}
	return sess.Commit()
}
