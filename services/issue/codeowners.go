// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"bufio"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	organization_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"gopkg.in/godo.v2/glob"
)

type CodeownerRule struct {
	glob  string                     // Glob pattern for matching files to owners
	users []*user_model.User         // Users designated as owners of files matching the glob pattern
	teams []*organization_model.Team // Teams designated as owners of files matching the glob pattern
}

type InvalidSyntaxError struct {
	owners []string
}

func (e *InvalidSyntaxError) Error() string {
	return fmt.Sprintf("owners: %v", e.owners)
}

type ExistenceAndPermissionError struct {
	owners []string
}

func (e *ExistenceAndPermissionError) Error() string {
	return fmt.Sprintf("owners: %v", e.owners)
}

func (e *ExistenceAndPermissionError) Owners() []string {
	return e.owners
}

// ParseCodeowners parses the given CODEOWNERS file contents and returns all Users and Teams with write permissions (who own any of the changed files) and any
// validationErrors, which maps line numbers to the error (InvalidSyntaxError or ExistenceAndPermissionError)
func ParseCodeowners(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, changedFiles []string, codeownersContents []byte) (
	userOwners []*user_model.User,
	teamOwners []*organization_model.Team,
	validationErrors map[int]error,
	err error,
) {
	scanner := bufio.NewScanner(strings.NewReader(string(codeownersContents)))
	codeownerMap, validationErrors, err := ScanAndParseCodeowners(ctx, repo, doer, *scanner)

	for _, file := range changedFiles {
		users, teams := GetOwners(codeownerMap, file)
		userOwners = append(userOwners, users...)
		teamOwners = append(teamOwners, teams...)
	}
	userOwners = RemoveDuplicateUsers(userOwners)
	teamOwners = RemoveDuplicateTeams(teamOwners)

	log.Trace("Final result of Codeowner Users: ")
	for _, user := range userOwners {
		log.Trace(user.Name)
	}

	log.Trace("Final result of Codeowner Teams: ")
	for _, team := range teamOwners {
		log.Trace(team.Name)
	}

	return userOwners, teamOwners, validationErrors, err
}

// GetOwners returns the list of owners for a single file given the defined codeownership rules in codeownerMap
func GetOwners(codeownerMap []CodeownerRule, file string) ([]*user_model.User, []*organization_model.Team) {
	for i := len(codeownerMap) - 1; i >= 0; i-- {
		if glob.Globexp(codeownerMap[i].glob).MatchString(file) {
			log.Trace("Codeowner file mappping. File: %s, Ownership map: %v", file, codeownerMap[i])
			return codeownerMap[i].users, codeownerMap[i].teams
		}
	}
	log.Trace("Unmatched codeowners file: ", file)
	return nil, nil
}

// SeparateOwnerAndTeam separates user name/email and team names (org/team-name) based on format
func SeparateOwnerAndTeam(codeownersList []string) (codeownerIndividuals, codeOwnerTeams []string) {
	for _, codeowner := range codeownersList {
		if len(codeowner) > 0 {

			// We remove that @ sign from the codeowner because it's unnecessary for future checks -- they only need the username
			if strings.Compare(codeowner[0:1], "@") == 0 {
				codeowner = codeowner[1:]
			}

			// If the string contains '/' it must be a team, based on format. Otherwise, it's an individual user
			if strings.Contains(codeowner, "/") {
				codeOwnerTeams = append(codeOwnerTeams, codeowner)
			} else {
				codeownerIndividuals = append(codeownerIndividuals, codeowner)
			}
		}
	}

	return codeownerIndividuals, codeOwnerTeams
}

// RemoveDuplicateUsers returns a list without any duplicate users
func RemoveDuplicateUsers(duplicatesPresent []*user_model.User) []*user_model.User {
	// Make a map with all keys initialized to false
	allKeys := make(map[*user_model.User]bool)
	duplicatesRemoved := []*user_model.User{}

	// For each item in the list, add it and mark it "true" in the map, then skip it every time
	for _, item := range duplicatesPresent {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			duplicatesRemoved = append(duplicatesRemoved, item)
		}
	}

	return duplicatesRemoved
}

// RemoveDuplicateTeams returns a list without any duplicate teams
func RemoveDuplicateTeams(duplicatesPresent []*organization_model.Team) []*organization_model.Team {
	// Make a map with all keys initialized to false
	allKeys := make(map[*organization_model.Team]bool)
	duplicatesRemoved := []*organization_model.Team{}

	// For each item in the list, add it and mark it "true" in the map, then skip it every time
	for _, item := range duplicatesPresent {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			duplicatesRemoved = append(duplicatesRemoved, item)
		}
	}

	return duplicatesRemoved
}

// ScanAndParseCodeowners parses the CODEOWNERS contents and extracts the rules/patterns and their associated user/teams along with any validation errors
func ScanAndParseCodeowners(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, scanner bufio.Scanner) (codeownerRules []CodeownerRule, validationErrors map[int]error, err error) {
	validationErrors = make(map[int]error)
	var lineCounter int

	for scanner.Scan() {
		curLine := scanner.Text()
		lineCounter++
		globString, globString2, curLineOwnerCandidates := ParseCodeownersLine(curLine)

		// If there are no users/teams listed, that is a valid rule, but we return empty user/team lists
		if len(curLineOwnerCandidates) == 0 {
			newCodeownerRule := CodeownerRule{
				glob:  globString,
				users: []*user_model.User{},
				teams: []*organization_model.Team{},
			}
			codeownerRules = append(codeownerRules, newCodeownerRule)
		} else {
			if IsValidCodeownersLineSyntax(curLineOwnerCandidates) {
				users, teams, usersAndTeamsExistWithCorrectPermissions := GetUsersAndTeamsWithWritePermissions(ctx, repo, doer, curLineOwnerCandidates)
				if usersAndTeamsExistWithCorrectPermissions {
					newCodeownerRule := CodeownerRule{
						glob:  globString,
						users: users,
						teams: teams,
					}

					codeownerRules = append(codeownerRules, newCodeownerRule)

					if globString2 != "" {
						newCodeownersRule2 := CodeownerRule{
							glob:  globString2,
							users: users,
							teams: teams,
						}

						codeownerRules = append(codeownerRules, newCodeownersRule2)
					}
				} else {
					validationErrors[lineCounter] = &ExistenceAndPermissionError{
						owners: curLineOwnerCandidates,
					}
					log.Trace("Invalid user/team/email given on line " + fmt.Sprint(lineCounter) + ":" + curLine)
				}
			} else {
				validationErrors[lineCounter] = &InvalidSyntaxError{
					owners: curLineOwnerCandidates,
				}
				log.Trace("Invalid syntax given on line " + fmt.Sprint(lineCounter) + ":" + curLine)
			}
		}

		log.Trace("Line number " + fmt.Sprint(lineCounter) + ":")
		log.Trace("Parsed as Glob string: " + globString + "," + globString2 + " Users: " + fmt.Sprint(curLineOwnerCandidates))
	}

	if scanner.Err() != nil {
		log.Trace(scanner.Err().Error())
		return nil, validationErrors, scanner.Err()
	}

	log.Trace("Parsed map from codeowners file: " + fmt.Sprint(codeownerRules))
	return codeownerRules, validationErrors, nil
}

// ParseCodeownersLine extracts two potential globbing rule strings and the owners associated with those rules for a given line
// of a CODEOWNERS file. Note that there are two potential globbing rules for the following situation, when we can't identify
// whether it's a file name or a subdirectory: /docs/github can be either /docs/github or /docs/github/**
func ParseCodeownersLine(line string) (globString, globString2 string, currFileUsers []string) {
	splitStrings := strings.Fields(line)
	var userStopIndex int

	for i := 0; i < len(splitStrings); i++ {
		// The first two checks here handle comments
		if strings.Compare(splitStrings[i], "#") == 0 {
			break
		} else if strings.Contains(splitStrings[i], "#") {
			commentStrings := strings.Split(splitStrings[i], "#")
			if len(commentStrings[0]) > 0 {
				if i == 0 {
					globString = commentStrings[0]
				} else {
					splitStrings[i] = commentStrings[0]
					userStopIndex = i
				}
			}
			break
		} else if i == 0 {
			globString = splitStrings[i]

			// Note the logic here for mapping from Codeowners format to our current globbing library
			if len(globString) < 1 {
				// This should only occur if the first character is '/', which we don't consider a valid rule
			} else if len(globString) == 1 {
				if strings.Compare(globString[0:1], "*") == 0 {
					globString = "**/**/**"
				}
			} else if strings.Compare(globString[0:1], "/") == 0 {
				globString = globString[1:]
			} else if strings.Compare(globString[0:1], "*") == 0 &&
				strings.Compare(globString[1:2], "*") != 0 {
				globString = "**/" + globString
			} else if strings.Compare(globString[0:1], "*") != 0 {
				globString = "**/" + globString
			} else if strings.Compare(globString[(len(globString)-1):], "/") == 0 {
				globString = "**/" + globString + "**"
			}

			if strings.Compare(globString[len(globString)-1:], "/") != 0 &&
				strings.Compare(globString[len(globString)-1:], "*") != 0 {
				globString2 = globString + "/**"
			} else if strings.Compare(globString[len(globString)-1:], "/") == 0 {
				globString += "**"
			}
		} else {
			userStopIndex = i
		}
	}

	if userStopIndex > 0 {
		currFileUsers = splitStrings[1 : userStopIndex+1]
	}

	return globString, globString2, currFileUsers
}

// IsValidCodeownersLineSyntax returns true if the given line of the CODEOWNERS file (after the file pattern) is valid syntactically
func IsValidCodeownersLineSyntax(currFileOwnerCandidates []string) bool {
	for _, user := range currFileOwnerCandidates {
		if !glob.Globexp("@*").MatchString(user) &&
			!glob.Globexp("@*/*").MatchString(user) &&
			!glob.Globexp("*@*.*").MatchString(user) {
			return false
		}
	}
	return true
}

// GetUsersAndTeamsWithWritePermissions gets the Users and Teams from the given array of owner candidates (emails, usernames, and team names).
// Returns nil arrays and false if any Users/Teams are not found or do not have write permissions for the given repository.
func GetUsersAndTeamsWithWritePermissions(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, ownerCandidates []string) (users []*user_model.User, teams []*organization_model.Team, isValidLine bool) {
	currIndividualOwners, currTeamOwners := SeparateOwnerAndTeam(ownerCandidates)

	for _, individual := range currIndividualOwners {
		user, err := GetUserByNameOrEmail(ctx, individual, repo)
		if err == nil {
			if UserHasWritePermissions(ctx, repo, user) {
				users = append(users, user)
			} else {
				return nil, nil, false
			}
		} else {
			return nil, nil, false
		}
	}

	for _, team := range currTeamOwners {
		team, err := GetTeamFromFullName(ctx, team, doer)
		if err == nil {
			if TeamHasWritePermissions(ctx, repo, team) {
				teams = append(teams, team)
			} else {
				return nil, nil, false
			}
		} else {
			return nil, nil, false
		}
	}

	return users, teams, true
}

// GetCodeownersFileContents gets the CODEOWNERS file from the top level,'.gitea', or 'docs' directory of the
// given repository. It uses whichever is found first if there are multiple (there should not be)
func GetCodeownersFileContents(ctx context.Context, commit *git.Commit, gitRepo *git.Repository) ([]byte, error) {
	entry := GetCodeownersGitTreeEntry(commit)
	if entry == nil {
		return nil, nil
	}

	if entry.IsRegular() {
		gitBlob := entry.Blob()
		data, err := gitBlob.GetBlobContentBase64()
		if err != nil {
			return nil, err
		}
		contentBytes, err := b64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, err
		}
		return contentBytes, nil
	} else {
		log.Warn("GetCodeownersFileContents [commit_id: %d, git_tree_entry_id: %d]: CODEOWNERS file found is not a regular file", commit.ID, entry.ID)
		return nil, nil
	}
}

// TODO: Move to within parse function and create custom error type. Then can be used by calling function to handle error how it needs to.
// IsCodeownersWithinSizeLimit returns an error if the file is too big. Nil if acceptable.
func IsCodeownersWithinSizeLimit(contentBytes []byte) error {
	byteLimit := 3 * 1024 * 1024 // 3 MB limit, per GitHub specs
	if len(contentBytes) >= byteLimit {
		return fmt.Errorf("CODEOWNERS file exceeds size limit. Is %d bytes but must be under %d", len(contentBytes), byteLimit)
	}
	return nil
}

// GetCodeownersGitTreeEntry gets the git tree entry of the CODEOWNERS file. Nil if not found in an accepted location.
func GetCodeownersGitTreeEntry(commit *git.Commit) *git.TreeEntry {
	// Accepted directories to search for the CODEOWNERS file
	directoryOptions := []string{"", ".gitea/", "docs/"}

	for _, dir := range directoryOptions {
		entry, _ := commit.GetTreeEntryByPath(dir + "CODEOWNERS")
		if entry != nil {
			return entry
		}
	}
	return nil
}

// GetUserByNameOrEmail gets the user by either its name or email depending on the format of the input
func GetUserByNameOrEmail(ctx context.Context, nameOrEmail string, repo *repo_model.Repository) (*user_model.User, error) {
	var reviewer *user_model.User
	var err error
	if strings.Contains(nameOrEmail, "@") {
		reviewer, err = user_model.GetUserByEmail(ctx, nameOrEmail)
		if err != nil {
			log.Info("GetUserByNameOrEmail [repo_id: %d, owner_email: %s]: user owner in CODEOWNERS file could not be found by email", repo.ID, nameOrEmail)
		}
	} else {
		reviewer, err = user_model.GetUserByName(ctx, nameOrEmail)
		if err != nil {
			log.Info("GetUserByNameOrEmail [repo_id: %d, owner_username: %s]: user owner in CODEOWNERS file could not be found by name", repo.ID, nameOrEmail)
		}
	}
	return reviewer, err
}

// GetTeamFromFullName gets the team given its full name ('{organizationName}/{teamName}'). Nil if not found.
func GetTeamFromFullName(ctx context.Context, fullTeamName string, doer *user_model.User) (*organization_model.Team, error) {
	teamNameSplit := strings.Split(fullTeamName, "/")
	if len(teamNameSplit) != 2 {
		return nil, errors.New("Team name must split into exactly 2 parts when split on '/'")
	}
	organizationName, teamName := teamNameSplit[0], teamNameSplit[1]

	opts := organization_model.FindOrgOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		UserID:         doer.ID,
		IncludePrivate: true,
	}
	organizations, err := organization_model.FindOrgs(opts)
	if err != nil {
		return nil, err
	}

	var organization *organization_model.Organization
	for _, org := range organizations {
		if org.Name == organizationName {
			organization = org
			break
		}
	}

	var team *organization_model.Team
	if organization != nil {
		team, err = organization.GetTeam(ctx, teamName)
		if err != nil {
			return nil, err
		}
	}
	return team, nil
}

// UserHasWritePermissions returns true if the user has write permissions to the code in the repository
func UserHasWritePermissions(ctx context.Context, repo *repo_model.Repository, user *user_model.User) bool {
	permission, err := access_model.GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		log.Debug("models/perm/access/GetUserRepoPermission: %v", err)
		return false
	}
	return permission.CanWrite(unit.TypeCode)
}

// TeamHasWritePermissions returns true if the team has write permissions to the code in the repository
func TeamHasWritePermissions(ctx context.Context, repo *repo_model.Repository, team *organization_model.Team) bool {
	if organization_model.HasTeamRepo(ctx, team.OrgID, team.ID, repo.ID) {
		return team.UnitAccessMode(ctx, unit.TypeCode) == perm.AccessModeWrite
	}
	return false
}
