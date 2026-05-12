// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package customlint

import (
	"io"
	"os"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

var (
	headerRE    = regexp.MustCompile(`^(// (Copyright [^\n]+|All rights reserved\.)\n)*// Copyright \d{4} (The Gogs Authors|The Gitea Authors|Gitea Authors|Gitea)\.( All rights reserved\.)?\n(// (Copyright [^\n]+|All rights reserved\.)\n)*// SPDX-License-Identifier: [\w.-]+`)
	generatedRE = regexp.MustCompile(`(?m)^// (Code|This file is) [Gg]enerated.*DO NOT EDIT`)
)

var goheaderAnalyzer = &analysis.Analyzer{
	Name: "goheader",
	Doc:  "checks Gitea copyright/SPDX file headers",
	Run:  runGoheader,
}

func runGoheader(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, 512))
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		if generatedRE.Match(data) {
			continue
		}
		if !headerRE.Match(data) {
			pass.Report(analysis.Diagnostic{
				Pos:     file.FileStart,
				Message: "missing or invalid copyright header",
			})
		}
	}
	return nil, nil //nolint:nilnil // analysis.Analyzer.Run contract: no result, no error
}
