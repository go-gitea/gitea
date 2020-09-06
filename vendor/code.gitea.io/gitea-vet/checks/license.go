// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package checks

import (
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	header     = regexp.MustCompile(`.*Copyright.*\d{4}.*(Gitea|Gogs)`)
	goGenerate = "//go:generate"
	buildTag   = "// +build"
)

var License = &analysis.Analyzer{
	Name: "license",
	Doc:  "check for a copyright header",
	Run:  runLicense,
}

func runLicense(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		if len(file.Comments) == 0 {
			pass.Reportf(file.Pos(), "Copyright not found")
			continue
		}

		if len(file.Comments[0].List) == 0 {
			pass.Reportf(file.Pos(), "Copyright not found or wrong")
			continue
		}

		commentGroup := 0
		if strings.HasPrefix(file.Comments[0].List[0].Text, goGenerate) {
			if len(file.Comments[0].List) > 1 {
				pass.Reportf(file.Pos(), "Must be an empty line between the go:generate and the Copyright")
				continue
			}
			commentGroup++
		}

		if strings.HasPrefix(file.Comments[0].List[0].Text, buildTag) {
			commentGroup++
		}

		if len(file.Comments) < commentGroup+1 {
			pass.Reportf(file.Pos(), "Copyright not found")
			continue
		}

		if len(file.Comments[commentGroup].List) < 1 {
			pass.Reportf(file.Pos(), "Copyright not found or wrong")
			continue
		}

		var check bool
		for _, comment := range file.Comments[commentGroup].List {
			if header.MatchString(comment.Text) {
				check = true
			}
		}

		if !check {
			pass.Reportf(file.Pos(), "Copyright did not match check")
		}
	}
	return nil, nil
}
