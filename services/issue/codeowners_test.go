// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	// "fmt"
	"testing"
	// "code.gitea.io/gitea/models/db"
	// repo_model "code.gitea.io/gitea/models/repo"
	// user_model "code.gitea.io/gitea/models/user"
	// "code.gitea.io/gitea/modules/log"
	// "github.com/stretchr/testify/assert"
)

func TestCodeownerParser(t *testing.T) {
	// 	/* This string acts as the codeowners file for our automated tests.
	// 	It tests for multiple users, single users, teams, and emails*/
	// 	codeownerString := "* @globalUser\n" +
	// 		"*.txt @user1 @user2 \n" +
	// 		"docs/ @user2 @user3 user4@gmail.com @ORG/user8\n" +
	// 		"/docs/ @user3\n" +
	// 		"/docs/maintain/ @user7\n" +
	// 		"/docs/github @user10\n" +
	// 		"logs @user9\n" +
	// 		"logs/ @user11\n" +
	// 		"**/apps user@user12.org\n" +
	// 		"/build/logs/ @ORG/user13\n" +
	// 		"/fakeFolder/DoesntExist.txt @noUserPresent\n" + // no user present should not be a reviewer
	// 		"/nouser/nofile @user14 @org/user15\n" + // user14 should not be a reviewer
	// 		"json/folder/ @jsonUser\n" + // jsonuser should NOT be a reviewer
	// 		"*.json\n" +
	// 		"* everyUserEndsHere\n" +
	// 		"*.json thisisnotauser\n" // these bottom two lines should be skipped, and the above line should be used (invalid syntax)

	// 	/* Each of the following is formatted such that the initial user's name
	// 	(the one they should match) with is contained somewhere in the file name. */
	// 	files := []string{
	// 		"user1.txt",
	// 		"properties/user1/prop.txt",
	// 		"main/properties/go/user1.txt",
	// 		"legos/docs/user2.txt",
	// 		"legos/docs/maintain/user2.txt",
	// 		"docs/dir1/user3.txt",
	// 		"docs/user3.txt",
	// 		"docs/maintain/dir1/dir2/user7.txt",
	// 		"main/go/user9/logs",
	// 		"docs/github/user10.txt",
	// 		"docs/github",
	// 		"docs/github/",
	// 		"build/logs/user11.fry",
	// 		"build/logs/user11",
	// 		"build/logs/user11/",
	// 		"apps/consoleApp.cpp",
	// 		"console/main/apps/user12/main.c",
	// 		"json/folder/noUser.json",
	// 		"newdir/nomatch/pretty.js",
	// 		"nomatch/main.c",
	// 	}

	// 	codeownerBytes := []byte(codeownerString)
	// 	repo := repo_model.Repository{} // Make "mock" or pull from test repos they already have
	// 	doer := user_model.User{}       // Make "mock" or pull from test repos they already have
	// 	codeownerUsers, codeownerTeams, _, err := ParseCodeowners(db.DefaultContext, &repo, &doer, files, codeownerBytes)

	// 	assert.NoError(t, err)

	// 	expectedUserNames := []string{
	// 		"user1",
	// 		"user2",
	// 		"user3",
	// 		"user4@gmail.com",
	// 		"user7",
	// 		"user9",
	// 		"user10",
	// 		"user@user12.org",
	// 		"globalUser",
	// 	}

	// 	expectedTeamNames := []string{
	// 		"ORG/user8",
	// 		"ORG/user13",
	// 	}

	// 	actualUserNames := []string{}
	// 	actualTeamNames := []string{}
	// 	for _, user := range codeownerUsers {
	// 		actualUserNames = append(actualUserNames, user.Name)
	// 	}
	// 	for _, team := range codeownerTeams {
	// 		actualTeamNames = append(actualTeamNames, team.Name)
	// 	}

	// 	codeownerIndividualsAccurate := compareLists(actualUserNames, expectedUserNames)
	// 	codeownerTeamsAccurate := compareLists(actualTeamNames, expectedTeamNames)

	// 	assert.True(t, codeownerIndividualsAccurate)
	// 	assert.True(t, codeownerTeamsAccurate)
	// }

	// func compareLists(list1, list2 []string) bool {
	// 	if len(list1) != len(list2) {
	// 		return false
	// 	}

	// 	for i := range list1 {
	// 		if list1[i] != list2[i] {
	// 			log.Trace("Mismatched at " + fmt.Sprint(i) + ". List 1: " + fmt.Sprint(list1[i]) +
	// 				" List 2: " + fmt.Sprint(list2[i]))
	// 			return false
	// 		}
	// 	}

	// return true
}
