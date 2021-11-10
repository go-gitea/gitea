// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Expect that golangci-lint is installed.
// Get the current version of golangci-lint and compare it with the minium
// version. Exit -1 if it's equal or higher than the minium version.
// exit 0 if it's lower than the minium version.

// validVersion, checks if the version only contains dots and digits.
// also that it doesn't contain 2 dots after each other. Also cannot
// end with a dot.
func validVersion(version string) bool {
	if version == "" {
		return false
	}

	wasPreviousDot := false
	for _, cRune := range version {
		switch cRune {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			wasPreviousDot = false
		case '.':
			if wasPreviousDot {
				// Cannot have `..` within a version.
				return false
			}
			wasPreviousDot = true
		default:
			return false
		}
	}

	// Simplified from if wasPreviousDot { return false } else { return true }
	return !wasPreviousDot
}

// compareVersions compares 2 given versions.
// It will return true if the second version is equal or higher than
// the first version. It will return false if the second version is
// lower than the first version.
func compareVersions(firstVersion, secondVersion string) bool {
	firstVersionDigits := strings.Split(firstVersion, ".")
	secondVersionDigits := strings.Split(secondVersion, ".")

	lenSecondVersionDigits := len(secondVersionDigits) - 1

	for idx := range firstVersionDigits {
		if idx > lenSecondVersionDigits {
			return false
		}

		firstNumber, _ := strconv.Atoi(firstVersionDigits[idx])
		secondNumber, _ := strconv.Atoi(secondVersionDigits[idx])
		if firstNumber != secondNumber {
			return firstNumber >= secondNumber
		}
	}

	return true
}

func main() {
	// The minium version should be given by the the first argument.
	if len(os.Args) != 2 {
		log.Fatal("Incorrect amount of arguments was passed, expected 1 argument")
	}

	miniumVersion := os.Args[1]
	if !validVersion(miniumVersion) {
		log.Fatal("Given miniumVersion isn't a valid version")
	}

	// Get the version from golangci-lint
	cmd := exec.Command("golangci-lint", "--version")

	// Run the command and get the output.
	bytesOutput, err := cmd.Output()
	if err != nil {
		log.Fatalf("Running \"golangci-lint --version\" ran into a error: %v", err)
	}
	output := string(bytesOutput)

	// Extract the version from output.
	// Assuming they won't change this in the future
	// We will assume the version starts from 27th character.
	// We shouldn't assume the length of the version, so get the first
	// index of a whitespace after the 27th character.
	if len(output) < 28 {
		log.Fatalf("Output of \"golangci-lint --version\" hasn't the correct length")
	}

	whitespaceAfterVersion := strings.Index(output[27:], " ")
	if whitespaceAfterVersion == -1 {
		log.Fatalf("Couldn't get the whitespace after the version from the output of \"golangci-lint --version\"")
	}

	// Get the version from the output at the correct indexes.
	installedVersion := string(output[27 : 27+whitespaceAfterVersion])

	// Check if it's a valid version.
	if !validVersion(installedVersion) {
		log.Fatal("installedVersion isn't a valid version")
	}

	// If the installedVersion is higher or equal to miniumVersion
	// than it's all fine and thus we will exit with a 1 code.
	// Such that the code will only exit with a 0 code when the
	// installedVerion is lower than miniumVersion.
	if compareVersions(miniumVersion, installedVersion) {
		os.Exit(-1)
	}
}
