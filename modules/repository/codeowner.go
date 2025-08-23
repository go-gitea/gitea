// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
)

var codeOwnerFiles = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitea/CODEOWNERS"}

func IsCodeOwnerFile(f string) bool {
	return slices.Contains(codeOwnerFiles, f)
}

func GetCodeOwnerFiles() []string {
	return codeOwnerFiles
}

// GetCodeOwnersFromContent returns the code owners configuration
// Return empty slice if files missing
// Return warning messages on parsing errors
// We're trying to do the best we can when parsing a file.
// Invalid lines are skipped. Non-existent users and teams too.
func GetCodeOwnersFromContent(ctx context.Context, data string) ([]*CodeOwnerRule, []string) {
	if len(data) == 0 {
		return nil, nil
	}

	rules := make([]*CodeOwnerRule, 0)
	lines := strings.Split(data, "\n")
	warnings := make([]string, 0)

	for i, line := range lines {
		tokens := TokenizeCodeOwnersLine(line)
		if len(tokens) == 0 {
			continue
		} else if len(tokens) < 2 {
			warnings = append(warnings, fmt.Sprintf("Line: %d: incorrect format", i+1))
			continue
		}
		rule, wr := ParseCodeOwnersLine(ctx, tokens)
		for _, w := range wr {
			warnings = append(warnings, fmt.Sprintf("Line: %d: %s", i+1, w))
		}
		if rule == nil {
			continue
		}

		rules = append(rules, rule)
	}

	return rules, warnings
}

type CodeOwnerRule struct {
	Rule     *regexp.Regexp
	Negative bool
	Users    []*user_model.User
	Teams    []*org_model.Team
}

func ParseCodeOwnersLine(ctx context.Context, tokens []string) (*CodeOwnerRule, []string) {
	var err error
	rule := &CodeOwnerRule{
		Users:    make([]*user_model.User, 0),
		Teams:    make([]*org_model.Team, 0),
		Negative: strings.HasPrefix(tokens[0], "!"),
	}

	warnings := make([]string, 0)

	rule.Rule, err = regexp.Compile(fmt.Sprintf("^%s$", strings.TrimPrefix(tokens[0], "!")))
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("incorrect codeowner regexp: %s", err))
		return nil, warnings
	}

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

	if (len(rule.Users) == 0) && (len(rule.Teams) == 0) {
		warnings = append(warnings, "no users/groups matched")
		return nil, warnings
	}

	return rule, warnings
}

func TokenizeCodeOwnersLine(line string) []string {
	if len(line) == 0 {
		return nil
	}

	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "\t", " ")

	tokens := make([]string, 0)

	escape := false
	token := ""
	for _, char := range line {
		if escape {
			token += string(char)
			escape = false
		} else if string(char) == "\\" {
			escape = true
		} else if string(char) == "#" {
			break
		} else if string(char) == " " {
			if len(token) > 0 {
				tokens = append(tokens, token)
				token = ""
			}
		} else {
			token += string(char)
		}
	}

	if len(token) > 0 {
		tokens = append(tokens, token)
	}

	return tokens
}
