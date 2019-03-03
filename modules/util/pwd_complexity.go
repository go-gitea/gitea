package util

import (
	"regexp"
)

//Complexity struct for Complexity checks
type Complexity struct {
	Length         int
	ContainsUpper  bool
	ContainsLower  bool
	ContainsDigist bool
	ContainsSpec   bool
	UpperCount     int
	LowerCount     int
	SpecCount      int
	DigitCount     int
	Score          int
}

//CheckComplexity function check pwd
func CheckComplexity(pwd *string) Complexity {
	var KindSymbols = make(map[string]string)
	KindSymbols["lower"] = "[a-z]"
	KindSymbols["upper"] = "[A-Z]"
	KindSymbols["digits"] = "[0-9]"
	KindSymbols["spec"] = `[\!\@\#\$\%\^\&\*\(\\\)\-_\=\+\,\.\?\/\:\;\{\}\[\]~]`
	var c Complexity
	c.Length = len(*pwd)

	MatchLower := regexp.MustCompile(KindSymbols["lower"])
	c.ContainsLower = MatchLower.MatchString(*pwd)
	c.LowerCount = len(MatchLower.FindAllString(*pwd, -1))
	if c.ContainsLower {
		c.Score++
	}

	MatchUpper := regexp.MustCompile(KindSymbols["upper"])
	c.ContainsUpper = MatchUpper.MatchString(*pwd)
	c.UpperCount = len(MatchUpper.FindAllString(*pwd, -1))
	if c.ContainsUpper {
		c.Score++
	}

	MatchDigits := regexp.MustCompile(KindSymbols["digits"])
	c.ContainsDigist = MatchDigits.MatchString(*pwd)
	c.DigitCount = len(MatchDigits.FindAllString(*pwd, -1))
	if c.ContainsDigist {
		c.Score++
	}

	MatchSpec := regexp.MustCompile(KindSymbols["spec"])
	c.ContainsSpec = MatchSpec.MatchString(*pwd)
	c.SpecCount = len(MatchSpec.FindAllString(*pwd, -1))
	if c.ContainsSpec {
		c.Score++
	}
	return c
}
