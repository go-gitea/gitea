// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/util"

	licenseclassifier "github.com/google/licenseclassifier/v2"
)

var (
	classifier     *licenseclassifier.Classifier
	licenseAliases map[string]string

	// licenseUpdaterQueue represents a queue to handle update repo licenses
	licenseUpdaterQueue *queue.WorkerPoolQueue[*LicenseUpdaterOptions]
)

func Init() error {
	licenseUpdaterQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "repo_license_updater", repoLicenseUpdater)
	if licenseUpdaterQueue == nil {
		return fmt.Errorf("unable to create repo_license_updater queue")
	}
	go graceful.GetManager().RunWithCancel(licenseUpdaterQueue)
	return nil
}

type LicenseUpdaterOptions struct {
	RepoID int64
}

func repoLicenseUpdater(items ...*LicenseUpdaterOptions) []*LicenseUpdaterOptions {
	ctx := graceful.GetManager().ShutdownContext()

	for _, opts := range items {
		repo, err := repo_model.GetRepositoryByID(ctx, opts.RepoID)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: GetRepositoryByID: %v", opts.RepoID, err)
			continue
		}
		if repo.IsEmpty {
			continue
		}

		gitRepo, err := gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: OpenRepository: %v", opts.RepoID, err)
			continue
		}
		defer gitRepo.Close()

		commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: GetBranchCommit: %v", opts.RepoID, err)
			continue
		}
		if err = updateRepoLicenses(ctx, repo, commit); err != nil {
			log.Error("repoLicenseUpdater [%d] failed: updateRepoLicenses: %v", opts.RepoID, err)
		}
	}
	return nil
}

func AddRepoToLicenseUpdaterQueue(opts *LicenseUpdaterOptions) error {
	if opts == nil {
		return nil
	}
	return licenseUpdaterQueue.Push(opts)
}

func loadLicenseAliases() error {
	if licenseAliases != nil {
		return nil
	}

	data, err := options.AssetFS().ReadFile("", "license-aliases.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &licenseAliases)
	if err != nil {
		return err
	}
	return nil
}

func ConvertLicenseName(name string) string {
	if err := loadLicenseAliases(); err != nil {
		return name
	}

	v, ok := licenseAliases[name]
	if ok {
		return v
	}
	return name
}

func initClassifier() error {
	if classifier != nil {
		return nil
	}

	// threshold should be 0.84~0.86 or the test will be failed
	classifier = licenseclassifier.NewClassifier(.85)
	licenseFiles, err := options.AssetFS().ListFiles("license", true)
	if err != nil {
		return err
	}

	existLicense := make(container.Set[string])
	if len(licenseFiles) > 0 {
		for _, licenseFile := range licenseFiles {
			licenseName := ConvertLicenseName(licenseFile)
			if existLicense.Contains(licenseName) {
				continue
			}
			existLicense.Add(licenseName)
			data, err := options.License(licenseFile)
			if err != nil {
				return err
			}
			classifier.AddContent("License", licenseFile, licenseName, data)
		}
	}
	return nil
}

type LicenseValues struct {
	Owner string
	Email string
	Repo  string
	Year  string
}

func GetLicense(name string, values *LicenseValues) ([]byte, error) {
	data, err := options.License(name)
	if err != nil {
		return nil, fmt.Errorf("GetLicense[%s]: %w", name, err)
	}
	return fillLicensePlaceholder(name, values, data), nil
}

func fillLicensePlaceholder(name string, values *LicenseValues, origin []byte) []byte {
	placeholder := getLicensePlaceholder(name)

	scanner := bufio.NewScanner(bytes.NewReader(origin))
	output := bytes.NewBuffer(nil)
	for scanner.Scan() {
		line := scanner.Text()
		if placeholder.MatchLine == nil || placeholder.MatchLine.MatchString(line) {
			for _, v := range placeholder.Owner {
				line = strings.ReplaceAll(line, v, values.Owner)
			}
			for _, v := range placeholder.Email {
				line = strings.ReplaceAll(line, v, values.Email)
			}
			for _, v := range placeholder.Repo {
				line = strings.ReplaceAll(line, v, values.Repo)
			}
			for _, v := range placeholder.Year {
				line = strings.ReplaceAll(line, v, values.Year)
			}
		}
		output.WriteString(line + "\n")
	}

	return output.Bytes()
}

type licensePlaceholder struct {
	Owner     []string
	Email     []string
	Repo      []string
	Year      []string
	MatchLine *regexp.Regexp
}

func getLicensePlaceholder(name string) *licensePlaceholder {
	// Some universal placeholders.
	// If you want to add a new one, make sure you have check it by `grep -r 'NEW_WORD' options/license` and all of them are placeholders.
	ret := &licensePlaceholder{
		Owner: []string{
			"<name of author>",
			"<owner>",
			"[NAME]",
			"[name of copyright owner]",
			"[name of copyright holder]",
			"<COPYRIGHT HOLDERS>",
			"<copyright holders>",
			"<AUTHOR>",
			"<author's name or designee>",
			"[one or more legally recognised persons or entities offering the Work under the terms and conditions of this Licence]",
		},
		Email: []string{
			"[EMAIL]",
		},
		Repo: []string{
			"<program>",
			"<one line to give the program's name and a brief idea of what it does.>",
		},
		Year: []string{
			"<year>",
			"[YEAR]",
			"{YEAR}",
			"[yyyy]",
			"[Year]",
			"[year]",
		},
	}

	// Some special placeholders for specific licenses.
	// It's unsafe to apply them to all licenses.
	switch name {
	case "0BSD":
		return &licensePlaceholder{
			Owner:     []string{"AUTHOR"},
			Email:     []string{"EMAIL"},
			Year:      []string{"YEAR"},
			MatchLine: regexp.MustCompile(`Copyright \(C\) YEAR by AUTHOR EMAIL`), // there is another AUTHOR in the file, but it's not a placeholder
		}

		// Other special placeholders can be added here.
	}
	return ret
}

// updateRepoLicenses will update repository licenses col if license file exists
func updateRepoLicenses(ctx context.Context, repo *repo_model.Repository, commit *git.Commit) error {
	if commit == nil {
		return nil
	}

	_, licenseFile, err := findLicenseFile(commit)
	if err != nil {
		return fmt.Errorf("findLicenseFile: %w", err)
	}

	licenses := make([]string, 0)
	if licenseFile != nil {
		r, err := licenseFile.Blob().DataAsync()
		if err != nil {
			return err
		}
		defer r.Close()

		licenses, err = detectLicense(r)
		if err != nil {
			return fmt.Errorf("detectLicense: %w", err)
		}
	}
	return repo_model.UpdateRepoLicenses(ctx, repo, commit.ID.String(), licenses)
}

// GetDetectedLicenseFileName returns license file name in the repository if it exists
func GetDetectedLicenseFileName(ctx context.Context, repo *repo_model.Repository, commit *git.Commit) (string, error) {
	if commit == nil {
		return "", nil
	}
	_, licenseFile, err := findLicenseFile(commit)
	if err != nil {
		return "", fmt.Errorf("findLicenseFile: %w", err)
	}
	if licenseFile != nil {
		return licenseFile.Name(), nil
	}
	return "", nil
}

// findLicenseFile returns the entry of license file in the repository if it exists
func findLicenseFile(commit *git.Commit) (string, *git.TreeEntry, error) {
	if commit == nil {
		return "", nil, nil
	}
	entries, err := commit.ListEntries()
	if err != nil {
		return "", nil, fmt.Errorf("ListEntries: %w", err)
	}
	return FindFileInEntries(util.FileTypeLicense, entries, "", "", false)
}

// detectLicense returns the licenses detected by the given content buff
func detectLicense(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, nil
	}
	if err := initClassifier(); err != nil {
		return nil, err
	}

	matches, err := classifier.MatchFrom(r)
	if err != nil {
		return nil, err
	}
	if len(matches.Matches) > 0 {
		results := make(container.Set[string], len(matches.Matches))
		for _, r := range matches.Matches {
			if r.MatchType == "License" && !results.Contains(r.Variant) {
				results.Add(r.Variant)
			}
		}
		return results.Values(), nil
	}
	return nil, nil
}
