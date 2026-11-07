// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strings"
	"time"

	org_model "gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/optional"

	"github.com/dlclark/regexp2/v2"
)

// GetCodeOwnersFromContent returns the code owners configuration
// Return empty slice if files missing
// Return warning messages on parsing errors
// We're trying to do the best we can when parsing a file.
// Invalid lines are skipped. Non-existent users and teams too.
func GetCodeOwnersFromContent(ctx context.Context, data string) (rules []*CodeOwnerRule, warnings []string) {
	globalReview := optional.None[bool]()
	sectionReview := optional.None[bool]()
	section := ""
	var owners []string

	lineNum := 0
	for line := range strings.SplitSeq(data, "\n") {
		lineNum++
		line, reviewRequest, annotationWarnings := parseCodeOwnerAnnotation(line)
		for _, warning := range annotationWarnings {
			warnings = append(warnings, fmt.Sprintf("Line: %d: %s", lineNum, warning))
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if reviewRequest.Has() {
				globalReview = reviewRequest
			}
			continue
		}

		// To support section headers, there could be an edge case for end users:
		// `[a-z] @user` now becomes a section header instead of a rule, but it is really rare to happen.
		// A workaround is to use `/[a-z] @user` to make it a rule.
		if heading := line; strings.HasPrefix(heading, "[") {
			if end := strings.IndexByte(heading, ']'); end > 1 && (end == len(heading)-1 || heading[end+1] == ' ' || heading[end+1] == '\t') {
				section = heading[1:end]
				owners = TokenizeCodeOwnersLine(heading[end+1:])
				sectionReview = reviewRequest
				continue
			}
		}
		tokens := TokenizeCodeOwnersLine(line)
		if len(tokens) == 0 {
			continue
		}
		if len(tokens) == 1 && len(owners) > 0 {
			tokens = append(tokens, owners...)
		}
		if len(tokens) < 2 {
			warnings = append(warnings, fmt.Sprintf("Line: %d: incorrect format", lineNum))
			continue
		}
		rule, wr := ParseCodeOwnersLine(ctx, tokens)
		for _, w := range wr {
			warnings = append(warnings, fmt.Sprintf("Line: %d: %s", lineNum, w))
		}
		if rule == nil {
			continue
		}
		rule.LineNum = lineNum

		rule.Section = section
		rule.ReviewRequest = reviewRequest
		if !rule.ReviewRequest.Has() {
			rule.ReviewRequest = sectionReview
		}
		if !rule.ReviewRequest.Has() {
			rule.ReviewRequest = globalReview
		}
		rules = append(rules, rule)
	}

	return rules, warnings
}

// codeOwnerMatchTimeout bounds a single pattern match so a crafted pattern
// cannot stall via catastrophic backtracking. See also the aggregate budget
// enforced by the caller across the whole rules×files match loop.
const codeOwnerMatchTimeout = 150 * time.Millisecond

type CodeOwnerRule struct {
	LineNum       int
	Section       string
	ReviewRequest optional.Option[bool]

	Rule     *regexp2.Regexp // it supports negative lookahead, does better for end users
	Negative bool

	// TODO: these objects should not be loaded here, but rather lazily when needed.
	// The parsed rule should only contain the user/team names.
	Users []*user_model.User
	Teams []*org_model.Team
}

func ParseCodeOwnersLine(ctx context.Context, tokens []string) (rule *CodeOwnerRule, warnings []string) {
	rule = &CodeOwnerRule{Negative: strings.HasPrefix(tokens[0], "!")}

	// Strip leading "!" for negative rules, then strip leading "/" since
	// git returns relative paths (e.g. "docs/foo.md" not "/docs/foo.md")
	// and the regex is already anchored with ^...$, so the "/" is redundant.
	pattern := strings.TrimPrefix(tokens[0], "!")
	pattern = strings.TrimPrefix(pattern, "/")
	expr := fmt.Sprintf("^%s$", pattern)

	// To make life easier, we support glob expressions if the pattern contains a "*" or "/*" (e.g. "*.md" or "docs/*.md")
	// However, "src/test\\..*\\.js" is still the only way because "src/test.*.js" can't be used as a glob at the moment.
	// TODO: In the future, maybe it's better to introduce a new annotation to specify the pattern type, e.g.: "pattern-syntax=glob" or "pattern-syntax=regexp"
	if strings.HasPrefix(pattern, "*") || strings.Contains(pattern, "/*") {
		compiled, compileErr := glob.CompileToRegexp(pattern)
		if compileErr != nil {
			return nil, []string{fmt.Sprintf("incorrect codeowner glob: %s", compileErr)}
		}
		expr = compiled.String()
	}

	var err error
	rule.Rule, err = regexp2.Compile(expr, regexp2.None)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("incorrect codeowner regexp: %s", err))
		return nil, warnings
	}

	// Bound matching time so user-supplied patterns cannot stall PR creation via catastrophic backtracking.
	rule.Rule.MatchTimeout = codeOwnerMatchTimeout

	for _, user := range tokens[1:] {
		user = strings.TrimPrefix(user, "@")

		// Only @org/team can contain slashes
		if strings.Contains(user, "/") {
			s := strings.Split(user, "/")
			if len(s) != 2 {
				warnings = append(warnings, "incorrect codeowner group: "+user)
				continue
			}
			orgName := s[0]
			teamName := s[1]

			org, err := org_model.GetOrgByName(ctx, orgName)
			if err != nil {
				warnings = append(warnings, "incorrect codeowner organization: "+user)
				continue
			}
			teams, err := org.LoadTeams(ctx)
			if err != nil {
				warnings = append(warnings, "incorrect codeowner team: "+user)
				continue
			}

			for _, team := range teams {
				if team.Name == teamName {
					rule.Teams = append(rule.Teams, team)
				}
			}
		} else {
			u, err := user_model.GetUserByName(ctx, user)
			if err != nil {
				warnings = append(warnings, "incorrect codeowner user: "+user)
				continue
			}
			rule.Users = append(rule.Users, u)
		}
	}

	if len(rule.Users) == 0 && len(rule.Teams) == 0 {
		warnings = append(warnings, "no users/groups matched")
		return nil, warnings
	}

	return rule, warnings
}

func TokenizeCodeOwnersLine(line string) (tokens []string) {
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "\t", " ")
	if line == "" {
		return nil
	}

	escape := false
	var token strings.Builder
	for _, char := range line {
		if escape {
			token.WriteRune(char)
			escape = false
		} else if char == '\\' {
			escape = true
		} else if char == '#' {
			break
		} else if char == ' ' {
			if token.Len() > 0 {
				tokens = append(tokens, token.String())
				token.Reset()
			}
		} else {
			token.WriteRune(char)
		}
	}

	if token.Len() > 0 {
		tokens = append(tokens, token.String())
	}

	return tokens
}

func parseCodeOwnerAnnotation(line string) (string, optional.Option[bool], []string) {
	escape := false
	for i := 0; i < len(line); i++ {
		if escape {
			escape = false
			continue
		}
		if line[i] == '\\' {
			escape = true
			continue
		}
		if line[i] != '#' {
			continue
		}
		review := optional.None[bool]()
		var warnings []string
		if strings.HasPrefix(line[i:], "##") {
			annotation, _, _ := strings.Cut(line[i+2:], "#")
			for field := range strings.FieldsSeq(annotation) {
				key, value, ok := strings.Cut(field, "=")
				if key != "review-request" {
					warnings = append(warnings, "unknown codeowner annotation: "+key)
					continue
				}
				if !ok || (value != "true" && value != "false") {
					warnings = append(warnings, "incorrect review-request annotation: "+field)
					continue
				}
				review = optional.Some(value == "true")
			}
		}
		return line[:i], review, warnings
	}
	return line, optional.None[bool](), nil
}
