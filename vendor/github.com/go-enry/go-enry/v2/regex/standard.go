// +build !oniguruma

package regex

import (
	"regexp"
)

type EnryRegexp = *regexp.Regexp

func MustCompile(str string) EnryRegexp {
	return regexp.MustCompile(str)
}

func QuoteMeta(s string) string {
	return regexp.QuoteMeta(s)
}
