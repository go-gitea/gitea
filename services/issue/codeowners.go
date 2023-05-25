package issue

import (
	"bufio"
	"fmt"
	"strings"

	"gopkg.in/godo.v2/glob"
)

type Codeowners struct {
	glob   string
	owners []string
}

func ParseCodeowners(changedFiles []string, codeownersContents []byte) ([]string, []string, error) {

	// This calls the actual parser
	globMap, err := ParseCodeownerBytes(codeownersContents)

	// We have to declare a new list of strings to be able to append all codeowners
	//	As we get them file by file in the following for loop
	var codeownersList []string
	for _, file := range changedFiles {
		codeownersList = append(codeownersList, GetOwners(globMap, file)...)
	}
	codeownerIndividuals, codeOwnerTeams := SeparateOwnerAndTeam(codeownersList)
	fmt.Println(codeownerIndividuals)
	fmt.Println(codeOwnerTeams)

	// TODO: Do we need to return an error as well?
	return codeownerIndividuals, codeOwnerTeams, err

}

// GetOwners returns the list of owners (including teams) for a single file. It matches from our globMap
//
//	to the changed files that it receives from the for loop in the ParseCodeowners function above.
func GetOwners(globMap []Codeowners, file string) []string {

	for i := len(globMap) - 1; i >= 0; i-- {
		if glob.Globexp(globMap[i].glob).MatchString(file) {
			fmt.Println("File:", file, "Result:", globMap[i])

			return globMap[i].owners
		}
	}
	fmt.Println("File did not match:", file)
	return nil
}

// SeparateOwnerAndTeam separates owners and teams based on format.
//
//	Note that it also calls RemoveDuplicateString to remove duplicates
func SeparateOwnerAndTeam(codeownersList []string) ([]string, []string) {

	codeownerIndividuals := []string{}
	codeOwnerTeams := []string{}

	codeownersList = RemoveDuplicateString(codeownersList)

	for _, codeowner := range codeownersList {

		if len(codeowner) > 0 {
			if strings.Compare(codeowner[0:1], "@") == 0 {
				codeowner = codeowner[1:]
			}

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
func RemoveDuplicateString(duplicatesPresent []string) []string {
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

func ParseCodeownerBytes(codeownerBytes []byte) ([]Codeowners, error) {

	// Create a new scanner to read from the byte array
	scanner := bufio.NewScanner(strings.NewReader(string(codeownerBytes)))
	return ScanAndParse(*scanner)
}

func ScanAndParse(scanner bufio.Scanner) ([]Codeowners, error) {

	var globMap []Codeowners

	for scanner.Scan() {

		nextLine := scanner.Text()

		// strings.Fields() splits the string by whitespace
		splitStrings := strings.Fields(nextLine)
		var globString string
		var globString2 string
		var userStopIndex int
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
		} else {

			newCodeowner := Codeowners{
				glob:   globString,
				owners: []string{""},
			}

			globMap = append(globMap, newCodeowner)
		}

		// fmt.Println(nextLine)
		// fmt.Println("Glob string: ", globString)
		// fmt.Println("Current users: ", currFileUsers)
	}

	if scanner.Err() != nil {
		fmt.Println("Error reading file", scanner.Err())
		globMap = nil
		return globMap, scanner.Err()
	}

	fmt.Println(globMap)
	return globMap, scanner.Err()

}
