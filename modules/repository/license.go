// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/options"
)

type licenseValues struct {
	Owner string
	Email string
	Repo  string
	Year  string
}

func getLicense(name string, values *licenseValues) ([]byte, error) {
	placeholder := getLicensePlaceholder(name)
	data, err := options.License(name)
	if err != nil {
		return nil, fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
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

	return output.Bytes(), nil
}

type licensePlaceholder struct {
	Owner     []string
	Email     []string
	Repo      []string
	Year      []string
	MatchLine *regexp.Regexp
}

func getLicensePlaceholder(name string) *licensePlaceholder {
	// Some universal placeholders
	// Make sure you have check it by `grep -r 'NEW_WORD' options/license` before adding it here
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
	}
	return ret
}
