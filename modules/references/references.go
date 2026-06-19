// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package references

import (
	"regexp"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var (
	// issueNumericPattern matches string that references a numeric issue, e.g. #1287
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([#!])([0-9]+)(?:\s|$|\)|\]|[:;,.?!]\s|[:;,.?!]$)`)
	// issueAlphanumericPattern matches string that references an alphanumeric issue, e.g. ABC-1234
	issueAlphanumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([A-Z]{1,10}-[1-9][0-9]*)(?:\s|$|\)|\]|[:;,.?!]\s|[:;,.?!]$)`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogitea/gitea#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\w-\.]+)/([\w-\.]+(?:/[\w-\.]+)*)[#!]([0-9]+)(?:\s|$|\)|\]|[:;,.?!]\s|[:;,.?!]$)`)
	// crossReferenceIssueAlphanumericPattern matches string that references an alphanumeric issue in a different repository
	// e.g. gogitea/gitea#ABC-1234
	crossReferenceIssueAlphanumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\w-\.]+)/([\w-\.]+(?:/[\w-\.]+)*)#([A-Z]{1,10}-[1-9][0-9]*)(?:\s|$|\)|\]|[:;,.?!]\s|[:;,.?!]$)`)
)

// ... rest of the file ...