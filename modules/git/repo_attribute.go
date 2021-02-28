// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/gobwas/glob"
)

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
}

// CheckAttribute return the Blame object of file
func (repo *Repository) CheckAttribute(opts CheckAttributeOpts) (map[string]map[string]string, error) {
	err := LoadGitVersion()
	if err != nil {
		return nil, fmt.Errorf("Git version missing: %v", err)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"check-attr", "-z"}

	if opts.AllAttributes {
		cmdArgs = append(cmdArgs, "-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmdArgs = append(cmdArgs, attribute)
			}
		}
	}

	// git check-attr --cached first appears in git 1.7.8
	if opts.CachedOnly && CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
	}

	cmdArgs = append(cmdArgs, "--")

	for _, arg := range opts.Filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := NewCommand(cmdArgs...)

	if err := cmd.RunInDirPipeline(repo.Path, stdOut, stdErr); err != nil {
		return nil, fmt.Errorf("Failed to run check-attr: %v\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	var name2attribute2info = make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}

// AttrCheckResultType result type of AttrCheckResult
type AttrCheckResultType int

const (
	// AttrCheckResultTypeUnspecified the attribute is not defined for the path
	AttrCheckResultTypeUnspecified AttrCheckResultType = iota
	// AttrCheckResultTypeUnset the attribute is defined as false
	AttrCheckResultTypeUnset
	// AttrCheckResultTypeSet the attribute is defined as true
	AttrCheckResultTypeSet
	// AttrCheckResultTypeValue a value has been assigned to the attribute
	AttrCheckResultTypeValue
)

// AttrCheckResult the result of CheckAttributeFile
type AttrCheckResult struct {
	typ  AttrCheckResultType
	data string
}

// IsSet if the attribute is defined as true
func (r *AttrCheckResult) IsSet() bool {
	return r.typ == AttrCheckResultTypeSet
}

// Value get the value of AttrCheckResult
func (r *AttrCheckResult) Value() string {
	if r.typ != AttrCheckResultTypeValue {
		return ""
	}
	return r.data
}

// AttrChecker Attribute checker
// format attr: partens
type AttrChecker map[string][]*attrCheckerItem

type attrCheckerItem struct {
	pattern glob.Glob
	rs      *AttrCheckResult
}

// LoadAttrbutCheckerFromCommit load AttrChecker from a commit
func LoadAttrbutCheckerFromCommit(commit *Commit) (AttrChecker, error) {
	gitAttrEntry, err := commit.GetTreeEntryByPath("/.gitattributes")
	if err != nil {
		if !IsErrNotExist(err) {
			return nil, err
		}
		return nil, nil
	}
	if gitAttrEntry.IsDir() {
		return nil, nil
	}

	blob := gitAttrEntry.Blob()
	dataRc, err := blob.DataAsync()
	if err != nil {
		return nil, err
	}
	defer dataRc.Close()
	gitAttr := make([]byte, 1024)
	n, _ := dataRc.Read(gitAttr)
	gitAttr = gitAttr[:n]

	return LoadAttrbutCheckerFromReader(bytes.NewReader(gitAttr))
}

// LoadAttrbutCheckerFromReader load AttrChecker from content reader
func LoadAttrbutCheckerFromReader(r io.Reader) (AttrChecker, error) {
	cheker := make(AttrChecker)

	readr := bufio.NewScanner(r)
	for readr.Scan() {
		t := readr.Text()
		// format: pattern attr1 attr2 ...
		if len(t) == 0 {
			continue
		}

		splits := strings.Split(t, " ")
		if len(splits) < 2 {
			continue
		}

		// to let `/AAA/*.txt` can match `AAA/bb.txt`, have to
		// remove first / if exit
		splits[0] = strings.TrimPrefix(splits[0], "/")

		// get parten
		g, err := glob.Compile(splits[0], '/')
		if err != nil {
			return nil, err
		}

		check := func(attr string) (string, *AttrCheckResult) {
			// one attr may has three status:
			// set: XXX
			// unset: -XXX
			// value: XXX=VVV
			if kv := strings.SplitN(attr, "=", 2); len(kv) == 2 {
				return kv[0], &AttrCheckResult{
					typ:  AttrCheckResultTypeValue,
					data: kv[1],
				}
			}
			typ := AttrCheckResultTypeSet
			if strings.HasPrefix(attr, "-") {
				attr = attr[1:]
				typ = AttrCheckResultTypeUnset
			}
			return attr, &AttrCheckResult{typ: typ}
		}

		// check attrs
		attrs := splits[1:]
		for _, tmp := range attrs {
			attr, rs := check(tmp)
			v, ok := cheker[attr]
			if !ok {
				v = make([]*attrCheckerItem, 0, 5)
			}

			v = append(v, &attrCheckerItem{
				pattern: g,
				rs:      rs,
			})
			cheker[attr] = v
		}
	}

	return cheker, nil
}

// Check check an git attr
func (c AttrChecker) Check(requestAttr, path string) *AttrCheckResult {
	if c == nil {
		return nil
	}

	v, ok := c[requestAttr]
	if !ok {
		return &AttrCheckResult{typ: AttrCheckResultTypeUnspecified}
	}

	for _, item := range v {
		if !item.pattern.Match(path) {
			continue
		}
		return item.rs
	}

	return &AttrCheckResult{typ: AttrCheckResultTypeUnspecified}
}
