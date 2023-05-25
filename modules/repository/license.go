// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/licensecheck"
)

type licenseValues struct {
	Owner string
	Email string
	Repo  string
	Year  string
}

func getLicense(name string, values *licenseValues) ([]byte, error) {
	data, err := options.License(name)
	if err != nil {
		return nil, fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
	}
	return fillLicensePlaceholder(name, values, data), nil
}

func fillLicensePlaceholder(name string, values *licenseValues, origin []byte) []byte {
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

func UpdateRepoLicenses(ctx context.Context, repo *repo_model.Repository) error {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	commitID, err := gitRepo.GetBranchCommitID(repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommitID: %w", err)
	}
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		return fmt.Errorf("GetCommit: %w", err)
	}
	entries, err := commit.ListEntries()
	if err != nil {
		return fmt.Errorf("ListEntries: %w", err)
	}

	// Find license file
	_, licenseFile, err := FindFileInEntries(util.FileTypeLicense, entries, "", "", false)
	if err != nil {
		return fmt.Errorf("FindFileInEntries: %w", err)
	}

	if licenseFile != nil {
		// Read license file content
		blob := licenseFile.Blob()
		contentBuf, err := blob.GetBlobAll()
		if err != nil {
			return fmt.Errorf("GetBlobByPath: %w", err)
		}

		// check license
		var licenses []string
		cov := licensecheck.Scan(contentBuf)
		for _, m := range cov.Match {
			licenses = append(licenses, m.ID)
		}
		repo.Licenses = licenses
		if err := repo_model.UpdateRepositoryCols(ctx, repo, "licenses"); err != nil {
			return fmt.Errorf("UpdateRepositoryCols: %v", err)
		}
	}

	return nil
}
