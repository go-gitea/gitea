// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"

	licenseclassifier "github.com/google/licenseclassifier/v2"
)

var classifier *licenseclassifier.Classifier

// Copy paste from models/repo.go because we cannot import models package
func repoPath(userName, repoName string) string {
	return filepath.Join(userPath(userName), strings.ToLower(repoName)+".git")
}

func userPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

// Copy paste from modules/repository/file.go because we cannot import models package
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
	contentBuf, err := blob.GetBlobAll()
	if err != nil {
		return nil, fmt.Errorf("GetBlobAll: %w", err)
	}
	return detectLicense(contentBuf), nil
}

// detectLicense returns the licenses detected by the given content buff
func detectLicense(buf []byte) []string {
	if len(buf) == 0 {
		return nil
	}
	if classifier == nil {
		log.Error("detectLicense: license classifier is null.")
		return nil
	}

	matches := classifier.Match(buf)
	var results []string
	for _, r := range matches.Matches {
		if r.MatchType == "License" {
			results = append(results, r.Variant)
		}
	}
	return results
}

func initClassifier() error {
	// threshold should be 0.84~0.86 or the test will be failed
	// TODO: add threshold to app.ini
	data, err := options.AssetFS().ReadFile("", "convertLicenseName")
	if err != nil {
		return err
	}
	var convertLicenseName map[string]string
	err = json.Unmarshal([]byte(data), &convertLicenseName)
	if err != nil {
		return err
	}

	// threshold should be 0.84~0.86 or the test will be failed
	// TODO: add threshold to app.ini
	classifier = licenseclassifier.NewClassifier(.85)
	licenseFiles, err := options.AssetFS().ListFiles("license", true)
	if err != nil {
		return err
	}

	licenseVariantCount := make(map[string]int)
	if len(licenseFiles) > 0 {
		for _, lf := range licenseFiles {
			data, err := options.License(lf)
			if err != nil {
				return err
			}
			variant := lf
			if convertLicenseName != nil {
				v, ok := convertLicenseName[lf]
				if ok {
					variant = v
				}
				licenseVariantCount[variant]++
				if licenseVariantCount[variant] > 1 {
					continue
				}
			}
			classifier.AddContent("License", lf, variant, data)
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

	if err := initClassifier(); err != nil {
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
		repo.Licenses, err = detectLicenseByEntry(licenseFile)
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
