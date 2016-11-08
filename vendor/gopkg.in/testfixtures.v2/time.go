package testfixtures

import "regexp"

var (
	regexpDate     = regexp.MustCompile("\\d\\d\\d\\d-\\d\\d-\\d\\d")
	regexpDateTime = regexp.MustCompile("\\d\\d\\d\\d-\\d\\d-\\d\\d \\d\\d:\\d\\d:\\d\\d")
	regexpTime     = regexp.MustCompile("\\d\\d:\\d\\d:\\d\\d")
)

func isDate(value interface{}) bool {
	str, isStr := value.(string)
	if !isStr {
		return false
	}

	return regexpDate.MatchString(str)
}

func isDateTime(value interface{}) bool {
	str, isStr := value.(string)
	if !isStr {
		return false
	}

	return regexpDateTime.MatchString(str)
}

func isTime(value interface{}) bool {
	str, isStr := value.(string)
	if !isStr {
		return false
	}

	return regexpTime.MatchString(str)
}
