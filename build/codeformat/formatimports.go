// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package codeformat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

var importPackageGroupOrders = map[string]int{
	"":                     1, // internal
	"code.gitea.io/gitea/": 2,
}

var errInvalidCommentBetweenImports = errors.New("comments between imported packages are invalid, please move comments to the end of the package line")

var (
	importBlockBegin = []byte("\nimport (\n")
	importBlockEnd   = []byte("\n)")
)

type importLineParsed struct {
	group   string
	pkg     string
	content string
}

func parseImportLine(line string) (*importLineParsed, error) {
	il := &importLineParsed{content: line}
	p1 := strings.IndexRune(line, '"')
	if p1 == -1 {
		return nil, errors.New("invalid import line: " + line)
	}
	p1++
	p := strings.IndexRune(line[p1:], '"')
	if p == -1 {
		return nil, errors.New("invalid import line: " + line)
	}
	p2 := p1 + p
	il.pkg = line[p1:p2]

	pDot := strings.IndexRune(il.pkg, '.')
	pSlash := strings.IndexRune(il.pkg, '/')
	if pDot != -1 && pDot < pSlash {
		il.group = "domain-package"
	}
	for groupName := range importPackageGroupOrders {
		if groupName == "" {
			continue // skip internal
		}
		if strings.HasPrefix(il.pkg, groupName) {
			il.group = groupName
		}
	}
	return il, nil
}

type (
	importLineGroup    []*importLineParsed
	importLineGroupMap map[string]importLineGroup
)

func formatGoImports(contentBytes []byte) ([]byte, error) {
	p1 := bytes.Index(contentBytes, importBlockBegin)
	if p1 == -1 {
		return nil, nil
	}
	p1 += len(importBlockBegin)
	p := bytes.Index(contentBytes[p1:], importBlockEnd)
	if p == -1 {
		return nil, nil
	}
	p2 := p1 + p

	importGroups := importLineGroupMap{}
	r := bytes.NewBuffer(contentBytes[p1:p2])
	eof := false
	for !eof {
		line, err := r.ReadString('\n')
		eof = err == io.EOF
		if err != nil && !eof {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
				return nil, errInvalidCommentBetweenImports
			}
			importLine, err := parseImportLine(line)
			if err != nil {
				return nil, err
			}
			importGroups[importLine.group] = append(importGroups[importLine.group], importLine)
		}
	}

	var groupNames []string
	for groupName, importLines := range importGroups {
		groupNames = append(groupNames, groupName)
		sort.Slice(importLines, func(i, j int) bool {
			return strings.Compare(importLines[i].pkg, importLines[j].pkg) < 0
		})
	}

	sort.Slice(groupNames, func(i, j int) bool {
		n1 := groupNames[i]
		n2 := groupNames[j]
		o1 := importPackageGroupOrders[n1]
		o2 := importPackageGroupOrders[n2]
		if o1 != 0 && o2 != 0 {
			return o1 < o2
		}
		if o1 == 0 && o2 == 0 {
			return strings.Compare(n1, n2) < 0
		}
		return o1 != 0
	})

	formattedBlock := bytes.Buffer{}
	for _, groupName := range groupNames {
		hasNormalImports := false
		hasDummyImports := false
		// non-dummy import comes first
		for _, importLine := range importGroups[groupName] {
			if strings.HasPrefix(importLine.content, "_") {
				hasDummyImports = true
			} else {
				formattedBlock.WriteString("\t" + importLine.content + "\n")
				hasNormalImports = true
			}
		}
		// dummy (_ "pkg") comes later
		if hasDummyImports {
			if hasNormalImports {
				formattedBlock.WriteString("\n")
			}
			for _, importLine := range importGroups[groupName] {
				if strings.HasPrefix(importLine.content, "_") {
					formattedBlock.WriteString("\t" + importLine.content + "\n")
				}
			}
		}
		formattedBlock.WriteString("\n")
	}
	formattedBlockBytes := bytes.TrimRight(formattedBlock.Bytes(), "\n")

	var formattedBytes []byte
	formattedBytes = append(formattedBytes, contentBytes[:p1]...)
	formattedBytes = append(formattedBytes, formattedBlockBytes...)
	formattedBytes = append(formattedBytes, contentBytes[p2:]...)
	return formattedBytes, nil
}

// FormatGoImports format the imports by our rules (see unit tests)
func FormatGoImports(file string, doChangedFiles, doWriteFile bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	var contentBytes []byte
	{
		defer f.Close()
		contentBytes, err = io.ReadAll(f)
		if err != nil {
			return err
		}
	}
	formattedBytes, err := formatGoImports(contentBytes)
	if err != nil {
		return err
	}
	if formattedBytes == nil {
		return nil
	}
	if bytes.Equal(contentBytes, formattedBytes) {
		return nil
	}

	if doChangedFiles {
		fmt.Println(file)
	}

	if doWriteFile {
		f, err = os.OpenFile(file, os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(formattedBytes)
		return err
	}

	return err
}
