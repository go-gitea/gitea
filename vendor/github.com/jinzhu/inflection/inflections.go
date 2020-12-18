/*
Package inflection pluralizes and singularizes English nouns.

		inflection.Plural("person") => "people"
		inflection.Plural("Person") => "People"
		inflection.Plural("PERSON") => "PEOPLE"

		inflection.Singular("people") => "person"
		inflection.Singular("People") => "Person"
		inflection.Singular("PEOPLE") => "PERSON"

		inflection.Plural("FancyPerson") => "FancydPeople"
		inflection.Singular("FancyPeople") => "FancydPerson"

Standard rules are from Rails's ActiveSupport (https://github.com/rails/rails/blob/master/activesupport/lib/active_support/inflections.rb)

If you want to register more rules, follow:

		inflection.AddUncountable("fish")
		inflection.AddIrregular("person", "people")
		inflection.AddPlural("(bu)s$", "${1}ses") # "bus" => "buses" / "BUS" => "BUSES" / "Bus" => "Buses"
		inflection.AddSingular("(bus)(es)?$", "${1}") # "buses" => "bus" / "Buses" => "Bus" / "BUSES" => "BUS"
*/
package inflection

import (
	"regexp"
	"strings"
)

type inflection struct {
	regexp  *regexp.Regexp
	replace string
}

// Regular is a regexp find replace inflection
type Regular struct {
	find    string
	replace string
}

// Irregular is a hard replace inflection,
// containing both singular and plural forms
type Irregular struct {
	singular string
	plural   string
}

// RegularSlice is a slice of Regular inflections
type RegularSlice []Regular

// IrregularSlice is a slice of Irregular inflections
type IrregularSlice []Irregular

var pluralInflections = RegularSlice{
	{"([a-z])$", "${1}s"},
	{"s$", "s"},
	{"^(ax|test)is$", "${1}es"},
	{"(octop|vir)us$", "${1}i"},
	{"(octop|vir)i$", "${1}i"},
	{"(alias|status)$", "${1}es"},
	{"(bu)s$", "${1}ses"},
	{"(buffal|tomat)o$", "${1}oes"},
	{"([ti])um$", "${1}a"},
	{"([ti])a$", "${1}a"},
	{"sis$", "ses"},
	{"(?:([^f])fe|([lr])f)$", "${1}${2}ves"},
	{"(hive)$", "${1}s"},
	{"([^aeiouy]|qu)y$", "${1}ies"},
	{"(x|ch|ss|sh)$", "${1}es"},
	{"(matr|vert|ind)(?:ix|ex)$", "${1}ices"},
	{"^(m|l)ouse$", "${1}ice"},
	{"^(m|l)ice$", "${1}ice"},
	{"^(ox)$", "${1}en"},
	{"^(oxen)$", "${1}"},
	{"(quiz)$", "${1}zes"},
}

var singularInflections = RegularSlice{
	{"s$", ""},
	{"(ss)$", "${1}"},
	{"(n)ews$", "${1}ews"},
	{"([ti])a$", "${1}um"},
	{"((a)naly|(b)a|(d)iagno|(p)arenthe|(p)rogno|(s)ynop|(t)he)(sis|ses)$", "${1}sis"},
	{"(^analy)(sis|ses)$", "${1}sis"},
	{"([^f])ves$", "${1}fe"},
	{"(hive)s$", "${1}"},
	{"(tive)s$", "${1}"},
	{"([lr])ves$", "${1}f"},
	{"([^aeiouy]|qu)ies$", "${1}y"},
	{"(s)eries$", "${1}eries"},
	{"(m)ovies$", "${1}ovie"},
	{"(c)ookies$", "${1}ookie"},
	{"(x|ch|ss|sh)es$", "${1}"},
	{"^(m|l)ice$", "${1}ouse"},
	{"(bus)(es)?$", "${1}"},
	{"(o)es$", "${1}"},
	{"(shoe)s$", "${1}"},
	{"(cris|test)(is|es)$", "${1}is"},
	{"^(a)x[ie]s$", "${1}xis"},
	{"(octop|vir)(us|i)$", "${1}us"},
	{"(alias|status)(es)?$", "${1}"},
	{"^(ox)en", "${1}"},
	{"(vert|ind)ices$", "${1}ex"},
	{"(matr)ices$", "${1}ix"},
	{"(quiz)zes$", "${1}"},
	{"(database)s$", "${1}"},
}

var irregularInflections = IrregularSlice{
	{"person", "people"},
	{"man", "men"},
	{"child", "children"},
	{"sex", "sexes"},
	{"move", "moves"},
	{"mombie", "mombies"},
}

var uncountableInflections = []string{"equipment", "information", "rice", "money", "species", "series", "fish", "sheep", "jeans", "police"}

var compiledPluralMaps []inflection
var compiledSingularMaps []inflection

func compile() {
	compiledPluralMaps = []inflection{}
	compiledSingularMaps = []inflection{}
	for _, uncountable := range uncountableInflections {
		inf := inflection{
			regexp:  regexp.MustCompile("^(?i)(" + uncountable + ")$"),
			replace: "${1}",
		}
		compiledPluralMaps = append(compiledPluralMaps, inf)
		compiledSingularMaps = append(compiledSingularMaps, inf)
	}

	for _, value := range irregularInflections {
		infs := []inflection{
			inflection{regexp: regexp.MustCompile(strings.ToUpper(value.singular) + "$"), replace: strings.ToUpper(value.plural)},
			inflection{regexp: regexp.MustCompile(strings.Title(value.singular) + "$"), replace: strings.Title(value.plural)},
			inflection{regexp: regexp.MustCompile(value.singular + "$"), replace: value.plural},
		}
		compiledPluralMaps = append(compiledPluralMaps, infs...)
	}

	for _, value := range irregularInflections {
		infs := []inflection{
			inflection{regexp: regexp.MustCompile(strings.ToUpper(value.plural) + "$"), replace: strings.ToUpper(value.singular)},
			inflection{regexp: regexp.MustCompile(strings.Title(value.plural) + "$"), replace: strings.Title(value.singular)},
			inflection{regexp: regexp.MustCompile(value.plural + "$"), replace: value.singular},
		}
		compiledSingularMaps = append(compiledSingularMaps, infs...)
	}

	for i := len(pluralInflections) - 1; i >= 0; i-- {
		value := pluralInflections[i]
		infs := []inflection{
			inflection{regexp: regexp.MustCompile(strings.ToUpper(value.find)), replace: strings.ToUpper(value.replace)},
			inflection{regexp: regexp.MustCompile(value.find), replace: value.replace},
			inflection{regexp: regexp.MustCompile("(?i)" + value.find), replace: value.replace},
		}
		compiledPluralMaps = append(compiledPluralMaps, infs...)
	}

	for i := len(singularInflections) - 1; i >= 0; i-- {
		value := singularInflections[i]
		infs := []inflection{
			inflection{regexp: regexp.MustCompile(strings.ToUpper(value.find)), replace: strings.ToUpper(value.replace)},
			inflection{regexp: regexp.MustCompile(value.find), replace: value.replace},
			inflection{regexp: regexp.MustCompile("(?i)" + value.find), replace: value.replace},
		}
		compiledSingularMaps = append(compiledSingularMaps, infs...)
	}
}

func init() {
	compile()
}

// AddPlural adds a plural inflection
func AddPlural(find, replace string) {
	pluralInflections = append(pluralInflections, Regular{find, replace})
	compile()
}

// AddSingular adds a singular inflection
func AddSingular(find, replace string) {
	singularInflections = append(singularInflections, Regular{find, replace})
	compile()
}

// AddIrregular adds an irregular inflection
func AddIrregular(singular, plural string) {
	irregularInflections = append(irregularInflections, Irregular{singular, plural})
	compile()
}

// AddUncountable adds an uncountable inflection
func AddUncountable(values ...string) {
	uncountableInflections = append(uncountableInflections, values...)
	compile()
}

// GetPlural retrieves the plural inflection values
func GetPlural() RegularSlice {
	plurals := make(RegularSlice, len(pluralInflections))
	copy(plurals, pluralInflections)
	return plurals
}

// GetSingular retrieves the singular inflection values
func GetSingular() RegularSlice {
	singulars := make(RegularSlice, len(singularInflections))
	copy(singulars, singularInflections)
	return singulars
}

// GetIrregular retrieves the irregular inflection values
func GetIrregular() IrregularSlice {
	irregular := make(IrregularSlice, len(irregularInflections))
	copy(irregular, irregularInflections)
	return irregular
}

// GetUncountable retrieves the uncountable inflection values
func GetUncountable() []string {
	uncountables := make([]string, len(uncountableInflections))
	copy(uncountables, uncountableInflections)
	return uncountables
}

// SetPlural sets the plural inflections slice
func SetPlural(inflections RegularSlice) {
	pluralInflections = inflections
	compile()
}

// SetSingular sets the singular inflections slice
func SetSingular(inflections RegularSlice) {
	singularInflections = inflections
	compile()
}

// SetIrregular sets the irregular inflections slice
func SetIrregular(inflections IrregularSlice) {
	irregularInflections = inflections
	compile()
}

// SetUncountable sets the uncountable inflections slice
func SetUncountable(inflections []string) {
	uncountableInflections = inflections
	compile()
}

// Plural converts a word to its plural form
func Plural(str string) string {
	for _, inflection := range compiledPluralMaps {
		if inflection.regexp.MatchString(str) {
			return inflection.regexp.ReplaceAllString(str, inflection.replace)
		}
	}
	return str
}

// Singular converts a word to its singular form
func Singular(str string) string {
	for _, inflection := range compiledSingularMaps {
		if inflection.regexp.MatchString(str) {
			return inflection.regexp.ReplaceAllString(str, inflection.replace)
		}
	}
	return str
}
