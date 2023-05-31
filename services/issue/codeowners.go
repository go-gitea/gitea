package issue

import (
	"bufio"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/godo.v2/glob"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	organization_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

type Codeowners struct {
	glob  string
	users []*user_model.User
	teams []*organization_model.Team
}

var globalCodeownerMap []Codeowners
var globalCodeownerIndividuals []Codeowners
var globalCodeownerTeams []Codeowners

func ParseCodeowners(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, changedFiles []string, codeownersContents []byte) ([]*user_model.User, []*organization_model.Team, error) {

	// This calls the actual parser
	codeownerMap, err := ParseCodeownerBytes(ctx, repo, doer, codeownersContents)

	// We have to declare a new list of strings to be able to append all codeowners
	//	As we get them file by file in the following for loop
	users := []*user_model.User{}
	teams := []*organization_model.Team{}
	for _, file := range changedFiles {
		newUsers, newTeams := GetOwners(codeownerMap, file)
		users = append(users, newUsers...)
		teams = append(teams, newTeams...)
	}

	users = RemoveDuplicateUsers(users)
	teams = RemoveDuplicateTeams(teams)

	log.Trace("Final result of Codeowner Users: ")
	for _, user := range users {
		log.Trace(user.Name)
	}

	log.Trace("Final result of Codeowner Teams: ")
	for _, team := range teams {
		log.Trace(team.Name)
	}

	return users, teams, err
}

// GetOwners returns the list of owners (including teams) for a single file. It matches from our globMap
// to the changed files that it receives from the for loop in the ParseCodeowners function above.
func GetOwners(codeownerMap []Codeowners, file string) ([]*user_model.User, []*organization_model.Team) {

	for i := len(codeownerMap) - 1; i >= 0; i-- {
		if glob.Globexp(codeownerMap[i].glob).MatchString(file) {
			fmt.Println("File:", file, "Result:", codeownerMap[i])

			return codeownerMap[i].users, codeownerMap[i].teams
		}
	}
	log.Trace("!!!Unmatched file: ", file)
	return nil, nil
}

// SeparateOwnerAndTeam separates owners and teams based on format.
// Note that it also calls RemoveDuplicateString to remove duplicates
func SeparateOwnerAndTeam(codeownersList []string) ([]string, []string) {

	codeownerIndividuals := []string{}
	codeOwnerTeams := []string{}

	// codeownersList = RemoveDuplicateString(codeownersList)

	for _, codeowner := range codeownersList {

		if len(codeowner) > 0 {

			// We remove that @ sign from the codeowner because it's unnecessary for
			// 	future checks -- they only need the username
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

// Removing duplicates has to be done manually in Golang
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

// Removing duplicates has to be done manually in Golang
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

func ParseCodeownerBytes(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, codeownerBytes []byte) ([]Codeowners, error) {

	// Create a new scanner to read from the byte array
	scanner := bufio.NewScanner(strings.NewReader(string(codeownerBytes)))
	return ScanAndParse(ctx, repo, doer, *scanner)
}

// ScanAndParse is the director function for handling the incoming codeowner contents
func ScanAndParse(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, scanner bufio.Scanner) ([]Codeowners, error) {

	// globMap maps each line using a key/value pair held by the Codeowners Data type.
	//		It maps from the file type (e.g., *.js) to the users for that file type (e.g., @user1 @user2)
	var codeownerMap []Codeowners
	var lineCounter int = 0

	for scanner.Scan() {

		// We handle the codeowners file line-by-line, as all rules should follow that format
		nextLine := scanner.Text()
		lineCounter++
		globString, globString2, currFileUsers := ParseCodeownersLine(nextLine)

		// If there are no users listed, that is a valid rule, but we return an empty string, and then the PR
		//		handles that outside of the parser
		if len(currFileUsers) > 0 {
			if IsValidCodeownersLine(currFileUsers) {
				users, teams, isValid := HasValidWritePermissions(ctx, repo, doer, currFileUsers)
				if isValid {

					newCodeowner := Codeowners{
						glob:  globString,
						users: users,
						teams: teams,
					}

					codeownerMap = append(codeownerMap, newCodeowner)

					if globString2 != "" {
						newCodeowner2 := Codeowners{
							glob:  globString2,
							users: users,
							teams: teams,
						}

						codeownerMap = append(codeownerMap, newCodeowner2)
					}
				} else {
					//error type1 (invalid permissions)
				}
			} else {

				// fill in the error map with info
				//error type 2 -- syntax error

				log.Trace("Invalid syntax given on line " + fmt.Sprint(lineCounter) + ":" +
					nextLine)
			}
		} else {

			newCodeowner := Codeowners{
				glob:  globString,
				users: []*user_model.User{},
				teams: []*organization_model.Team{},
			}

			codeownerMap = append(codeownerMap, newCodeowner)
		}

		log.Trace("Line number " + fmt.Sprint(lineCounter) + ":")
		log.Trace("Parsed as Glob string: " + globString + "," + globString2 +
			"Users: " + fmt.Sprint(currFileUsers))
	}

	if scanner.Err() != nil {
		log.Trace(scanner.Err().Error())
		codeownerMap = nil
		return codeownerMap, scanner.Err()
	}

	log.Trace("Parsed map from codeowners file: " + fmt.Sprint(codeownerMap))
	return codeownerMap, scanner.Err()
}

// ParseCodeownersLine extracts two potential globbing rule strings and the owners associated with those rules for a given line
//
//	of a CODEOWNERS file. Note that there are two potential globbing rules for the following situation, when we can't identify
//	whether it's a file name or a subdirectory: /docs/github can be either /docs/github or /docs/github/**
func ParseCodeownersLine(line string) (globString, globString2 string, currFileUsers []string) {

	// strings.Fields() splits the string by whitespace
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

// IsValidCodeownersLine returns true if the given line of the CODEOWNERS file is valid syntactically (after the file pattern)
func IsValidCodeownersLine(currFileUsers []string) bool {

	for _, user := range currFileUsers {
		if !glob.Globexp("@*").MatchString(user) &&
			!glob.Globexp("@*/*").MatchString(user) &&
			!glob.Globexp("*@*.*").MatchString(user) {
			return false
		}
	}

	// Add checks here to validate users and teams (write permissions)
	// That way, we can use the parser to validate everything except current PR users and simplify our code base a lot.
	// Note that we have to change where we separate users and teams to here (which may actually be cleaner), in addition to
	//		Removing that @ here instead of above

	return true
}

func HasValidWritePermissions(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, currFileOwners []string) ([]*user_model.User, []*organization_model.Team, bool) {

	currIndividualOwners, currTeamOwners := SeparateOwnerAndTeam(currFileOwners)
	users := []*user_model.User{}
	teams := []*organization_model.Team{}

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
	// Accepted directories to search for the CODEOWNERS file
	possibleDirectories := []string{"", ".gitea/", "docs/"}

	entry := GetCodeownersGitTreeEntry(commit, possibleDirectories)
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
	}
	log.Warn("GetCodeownersFileContents [commit_id: %d, git_tree_entry_id: %d]: CODEOWNERS file found is not a regular file", commit.ID, entry.ID)

	return nil, nil
}

// IsCodeownersWithinSizeLimit returns an error if the file is too big. Nil if acceptable.
func IsCodeownersWithinSizeLimit(contentBytes []byte) error {
	byteLimit := 3 * 1024 * 1024 // 3 MB limit, per GitHub specs
	if len(contentBytes) >= byteLimit {
		return errors.New(fmt.Sprintf("CODEOWNERS file exceeds size limit. Is %d bytes but must be under %d", len(contentBytes), byteLimit))
	}
	return nil
}

// GetCodeownersGitTreeEntry gets the git tree entry of the CODEOWNERS file, given an array of directories to search in. Nil if not found.
func GetCodeownersGitTreeEntry(commit *git.Commit, directoryOptions []string) *git.TreeEntry {
	for _, dir := range directoryOptions {
		entry, _ := commit.GetTreeEntryByPath(dir + "CODEOWNERS")
		if entry != nil {
			return entry
		}
	}
	return nil
}

// GetValidUserCodeownerReviewers gets the Users that actually exist, are authorized to review the pull request, and have explicit write permissions for the repo
func GetValidUserCodeownerReviewers(ctx context.Context, userNamesOrEmails []string, repo *repo_model.Repository, doer *user_model.User, isAdd bool, issue *issues_model.Issue) (reviewers []*user_model.User) {
	reviewers = []*user_model.User{}

	permDoer, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		log.Error("models/perm/access/GetUserRepoPermission: %v", err)
		return reviewers // empty
	}

	for _, nameOrEmail := range userNamesOrEmails {
		reviewer, err := GetUserByNameOrEmail(ctx, nameOrEmail, repo)
		if reviewer != nil && err == nil {
			err = IsValidUserCodeowner(err, ctx, reviewer, doer, isAdd, issue, permDoer, repo)
			if err == nil {
				reviewers = append(reviewers, reviewer)
			}
		}
	}
	return reviewers
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

// GetValidTeamCodeownerReviewers gets the Teams that actually exist, are authorized to review the pull request, and have explicit write permissions for the repo
func GetValidTeamCodeownerReviewers(ctx context.Context, fullTeamNames []string, repo *repo_model.Repository, doer *user_model.User, isAdd bool, issue *issues_model.Issue) (teamReviewers []*organization_model.Team) {
	teamReviewers = []*organization_model.Team{}
	if repo.Owner.IsOrganization() {
		for _, fullTeamName := range fullTeamNames {
			teamReviewer, err := GetTeamFromFullName(ctx, fullTeamName, doer)
			if err != nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: error finding the team [%v]", repo.ID, fullTeamName, err)
			} else if teamReviewer == nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: no error returned, but the team was nil (could not be found)", repo.ID, fullTeamName)
			} else {
				err = IsValidTeamCodeowner(ctx, teamReviewer, doer, isAdd, issue, repo)
				if err == nil {
					teamReviewers = append(teamReviewers, teamReviewer)
				}
			}
		}
	}
	return teamReviewers
}

// IsValidUserCodeowner returns an error if the user is not eligible to be a codeowner for the given repository (must be an eligible reviewer
// and have explcit write permissions). Nil if valid.
func IsValidUserCodeowner(err error, ctx context.Context, reviewer *user_model.User, doer *user_model.User, isAdd bool, issue *issues_model.Issue, permDoer access_model.Permission, repo *repo_model.Repository) error {
	err = IsValidReviewRequest(ctx, reviewer, doer, isAdd, issue, &permDoer)
	if err == nil {
		if UserHasWritePermissions(ctx, repo, reviewer) {
			return nil
		} else {
			log.Info("IsValidUserCodeowner [repo_id: %d, user_id: %d]: user reviewer does not have write permissions and cannot be a codeowner", repo.ID, reviewer.ID)
		}
	} else {
		log.Info("IsValidUserCodeowner [repo_id: %d, user_id: %d]: user reviewer is not a valid review request", repo.ID, reviewer.ID)
	}
	return errors.New(fmt.Sprintf("User %s is not a valid codeowner in the given repository", reviewer.Name))
}

// IsValidTeamCodeowner returns an error if the team is not eligible to be a codeowner for the given repository (must be an eligible reviewer
// and have explcit write permissions). Nil if valid.
func IsValidTeamCodeowner(ctx context.Context, teamReviewer *organization_model.Team, doer *user_model.User, isAdd bool, issue *issues_model.Issue, repo *repo_model.Repository) error {
	err := IsValidTeamReviewRequest(ctx, teamReviewer, doer, isAdd, issue)
	if err == nil {
		if TeamHasWritePermissions(ctx, repo, teamReviewer) {
			return nil
		} else {
			log.Info("IsValidTeamCodeowner [repo_id: %d, team_id: %d]: team reviewer does not have write permissions and cannot be a codeowner", repo.ID, teamReviewer.ID)
		}
	} else {
		log.Info("IsValidTeamCodeowner [repo_id: %d, team_id: %d]: team reviewer is not a valid review request", repo.ID, teamReviewer.ID)
	}
	return errors.New(fmt.Sprintf("Team %s is not a valid codeowner in the given repository", teamReviewer.Name))
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
	} else {
		return false
	}
}
