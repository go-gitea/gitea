package version

import (
	"regexp"
	"strings"
)

var modifierRegex = `[._-]?(?:(stable|beta|b|RC|alpha|a|patch|pl|p)(?:[.-]?(\d+))?)?([.-]?dev)?`

var regexpMasterLikeBranches = regexp.MustCompile(`^(?:dev-)?(?:master|trunk|default)$`)
var regexpBranchNormalize = regexp.MustCompile(`(?i)^v?(\d+)(\.(?:\d+|[x*]))?(\.(?:\d+|[x*]))?(\.(?:\d+|[x*]))?$`)

// Normalizes a version string to be able to perform comparisons on it
//
// Example:
//     version.Normalize("10.4.13-b")
//     Returns: 10.4.13.0-beta
//
func Normalize(version string) string {

	// ignore aliases and just assume the alias is required instead of the source
	result := RegFind(`^([^,\s]+) +as +([^,\s]+)$`, version)
	if result != nil {
		version = result[1]
	}

	// match master-like branches
	if regexpMasterLikeBranches.MatchString(strings.ToLower(version)) {
		return "9999999-dev"
	}

	if strings.HasPrefix(strings.ToLower(version), "dev-") {
		return "dev-" + version[4:len(version)]
	}

	index := 0

	// match classical versioning
	result = RegFind(`(?i)^v?(\d{1,3})(\.\d+)?(\.\d+)?(\.\d+)?`+modifierRegex+`$`, version)
	if result != nil {
		version = ""
		for _, val := range result[1:5] {
			if val != "" {
				version = version + val
			} else {
				version = version + ".0"
			}
		}

		index = 5
	} else {
		// match date-based versioning
		result = RegFind(`(?i)^v?(\d{4}(?:[.:-]?\d{2}){1,6}(?:[.:-]?\d{1,3})?)`+modifierRegex+`$`, version)
		if result != nil {
			version = regexp.MustCompile(`\D`).ReplaceAllString(result[1], "-")
			index = 2
		}
	}

	if index != 0 {
		if result[index] != "" {
			if result[index] == "stable" {
				return version
			}

			version = version + "-" + expandStability(result[index])
			if result[index+1] != "" {
				version = version + result[index+1]
			}
		}

		if result[index+2] != "" {
			version = version + "-dev"
		}

		return version
	}

	result = RegFind(`(?i)(.*?)[.-]?dev$`, version)
	if result != nil {
		return normalizeBranch(result[1])
	}

	return version
}

func normalizeBranch(name string) string {
	name = strings.Trim(name, " ")

	if name == "master" || name == "trunk" || name == "default" {
		return Normalize(name)
	}

	replace := strings.NewReplacer("*", "9999999", "x", "9999999")

	matched := regexpBranchNormalize.FindAllStringSubmatch(name, -1)
	if matched != nil {
		name = ""
		for _, val := range matched[0][1:5] {
			if val != "" {
				name = name + replace.Replace(val)
			} else {
				name = name + ".9999999"
			}
		}

		return name + "-dev"

	}

	if strings.HasSuffix(strings.ToLower(name), "-dev") {
		return name
	}

	return "dev-" + name
}
