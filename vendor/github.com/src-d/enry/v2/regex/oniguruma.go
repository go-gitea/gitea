// +build oniguruma

package regex

import (
	rubex "github.com/src-d/go-oniguruma"
)

type EnryRegexp = *rubex.Regexp

func MustCompile(str string) EnryRegexp {
	return rubex.MustCompileASCII(str)
}

func QuoteMeta(s string) string {
	return rubex.QuoteMeta(s)
}
