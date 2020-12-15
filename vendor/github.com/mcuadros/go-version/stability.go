package version

import (
	"regexp"
	"strings"
)

const (
	Development = iota
	Alpha
	Beta
	RC
	Stable
)

func expandStability(stability string) string {
	stability = strings.ToLower(stability)

	switch stability {
	case "a":
		return "alpha"
	case "b":
		return "beta"
	case "p":
		return "patch"
	case "pl":
		return "patch"
	case "rc":
		return "RC"
	}

	return stability
}

func parseStability(version string) string {
	version = regexp.MustCompile(`(?i)#.+$`).ReplaceAllString(version, " ")
	version = strings.ToLower(version)

	if strings.HasPrefix(version, "dev-") || strings.HasSuffix(version, "-dev") {
		return "dev"
	}

	result := RegFind(`(?i)^v?(\d{1,3})(\.\d+)?(\.\d+)?(\.\d+)?`+modifierRegex+`$`, version)
	if result != nil {
		if len(result) > 3 {
			return "dev"
		}
	}

	if result[1] != "" {
		if "beta" == result[1] || "b" == result[1] {
			return "beta"
		}
		if "alpha" == result[1] || "a" == result[1] {
			return "alpha"
		}
		if "rc" == result[1] {
			return "RC"
		}
	}

	return "stable"
}

func GetStability(version string) int {
	result := RegFind(`(?i)(stable|RC|beta|alpha|dev)`, Normalize(version))
	if len(result) == 0 {
		return Stable
	}

	switch result[1] {
	case "dev":
		return Development
	case "alpha":
		return Alpha
	case "beta":
		return Beta
	case "RC":
		return RC
	}

	return Stable
}
