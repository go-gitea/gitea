package issue

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/godo.v2/glob"
)

type Codeowners struct {
	glob   string
	owners []string
}

func main() {

	codeownerString := "*.txt @user1 @user2 @user3 @user4 @user5\n" +
		"docs/ @user2 @user3 @user6 @user7 @SBC/user8\n" +
		"/docs/ @user3\n" +
		"/docs/github @user10\n"

	files := []string{
		"user1.txt",
		"docs/dir1/user2.txt",
		"logs/docs/user2.txt",
		"docs/user3.txt",
		"newdir/pretty.js",
		"docs/github/user10.txt",
		"docs/github",
		"docs/github/",
		"properties/prop.txt",
		"main/properties/go/user1.txt",
		"docs/maintain/test.txt",
		"docs/check.txt",
		"hello_world.txt",
		"main.c",
		"build/logs/tobe.fry",
		"main/go/logs",
		"apps/consoleApp.cpp",
	}

	codeownerBytes := []byte(codeownerString)
	globMap := parseCodeownerBytes(codeownerBytes)
	// globMap := parseCodeownerFile("CODEOWNERS")

	var codeownersList []string
	for _, file := range files {
		codeownersList = append(codeownersList, getOwners(globMap, file)...)
	}
	codeownerIndividuals, codeOwnerTeams := separateOwnerAndTeam(codeownersList)
	fmt.Println(codeownerIndividuals)
	fmt.Println(codeOwnerTeams)

}

func getOwners(globMap []Codeowners, file string) []string {

	for i := len(globMap) - 1; i >= 0; i-- {
		if glob.Globexp(globMap[i].glob).MatchString(file) {
			fmt.Println("File:", file, "Result:", globMap[i])

			return globMap[i].owners
		}
	}
	fmt.Println("File did not match:", file)
	return nil
}

func separateOwnerAndTeam(codeownersList []string) ([]string, []string) {

	var codeownerIndividuals []string
	var codeOwnerTeams []string

	codeownersList = removeDuplicateString(codeownersList)

	for _, codeowner := range codeownersList {

		if strings.Compare(codeowner[0:1], "@") == 0 {
			codeowner = codeowner[1:]
		}

		if strings.Contains(codeowner, "/") {
			codeOwnerTeams = append(codeOwnerTeams, codeowner)
		} else {
			codeownerIndividuals = append(codeownerIndividuals, codeowner)
		}
	}

	return codeownerIndividuals, codeOwnerTeams

}

// Removing duplicates has to be done manually in Golang
func removeDuplicateString(duplicatesPresent []string) []string {
	allKeys := make(map[string]bool)
	duplicatesRemoved := []string{}

	for _, item := range duplicatesPresent {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			duplicatesRemoved = append(duplicatesRemoved, item)
		}
	}

	return duplicatesRemoved

}

func parseCodeownerFile(fileToRead string) []Codeowners {

	file, err := os.Open(fileToRead)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer file.Close()

	// Create a new scanner to read from the file
	scanner := bufio.NewScanner(file)
	return scanAndParse(*scanner)

}

func parseCodeownerString(codeownerString string) []Codeowners {

	// Create a new scanner to read from the string
	scanner := bufio.NewScanner(strings.NewReader(codeownerString))
	return scanAndParse(*scanner)

}

func parseCodeownerBytes(codeownerBytes []byte) []Codeowners {

	// Create a new scanner to read from the byte array
	scanner := bufio.NewScanner(strings.NewReader(string(codeownerBytes)))
	return scanAndParse(*scanner)
}

func scanAndParse(scanner bufio.Scanner) []Codeowners {

	var globMap []Codeowners

	for scanner.Scan() {

		nextLine := scanner.Text()

		// strings.Fields() splits the string by whitespace
		splitStrings := strings.Fields(nextLine)
		var globString string
		var globString2 string
		var userStopIndex int = 0
		var currFileUsers []string

		for i := 0; i < len(splitStrings); i++ {

			// fmt.Println(splitStrings[i])

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
					// Can we handle a situation where the only file type is /?
					// I don't think so because I think that they would just have to use *
				} else if len(globString) == 1 {
					if strings.Compare(globString[0:1], "*") == 0 {
						globString = "**/**/**"
					}
				} else if strings.Compare(globString[0:1], "/") == 0 {
					globString = globString[1:] /*+ "**"*/
				} else if strings.Compare(globString[0:1], "*") == 0 &&
					strings.Compare(globString[1:2], "*") != 0 {
					globString = "**/" + globString
				} else if strings.Compare(globString[(len(globString)-1):], "/") == 0 {
					globString = "**/" + globString + "**"
				}

				if strings.Compare(globString[len(globString)-1:], "/") != 0 &&
					strings.Compare(globString[len(globString)-1:], "*") != 0 {
					globString2 = globString + "/**"
				} else if strings.Compare(globString[len(globString)-1:], "/") == 0 {
					globString = globString + "**"
				}

			} else {
				userStopIndex = i
			}

		}

		if userStopIndex > 0 {
			currFileUsers = splitStrings[1 : userStopIndex+1]
		}

		if len(currFileUsers) > 0 {

			newCodeowner := Codeowners{
				glob:   globString,
				owners: currFileUsers,
			}

			globMap = append(globMap, newCodeowner)

			if globString2 != "" {
				newCodeowner2 := Codeowners{
					glob:   globString2,
					owners: currFileUsers,
				}

				globMap = append(globMap, newCodeowner2)
			}
		}

		// fmt.Println(nextLine)
		// fmt.Println("Glob string: ", globString)
		// fmt.Println("Current users: ", currFileUsers)
	}

	if scanner.Err() != nil {
		fmt.Println("Error reading file", scanner.Err())
		globMap = nil
		return globMap
	}

	fmt.Println(globMap)
	return globMap

}
