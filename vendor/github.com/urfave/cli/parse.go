package cli

import (
	"flag"
	"strings"
)

type iterativeParser interface {
	newFlagSet() (*flag.FlagSet, error)
	useShortOptionHandling() bool
}

// To enable short-option handling (e.g., "-it" vs "-i -t") we have to
// iteratively catch parsing errors.  This way we achieve LR parsing without
// transforming any arguments. Otherwise, there is no way we can discriminate
// combined short options from common arguments that should be left untouched.
func parseIter(ip iterativeParser, args []string) (*flag.FlagSet, error) {
	for {
		set, err := ip.newFlagSet()
		if err != nil {
			return nil, err
		}

		err = set.Parse(args)
		if !ip.useShortOptionHandling() || err == nil {
			return set, err
		}

		errStr := err.Error()
		trimmed := strings.TrimPrefix(errStr, "flag provided but not defined: ")
		if errStr == trimmed {
			return nil, err
		}

		// regenerate the initial args with the split short opts
		newArgs := []string{}
		for i, arg := range args {
			if arg != trimmed {
				newArgs = append(newArgs, arg)
				continue
			}

			shortOpts := splitShortOptions(set, trimmed)
			if len(shortOpts) == 1 {
				return nil, err
			}

			// add each short option and all remaining arguments
			newArgs = append(newArgs, shortOpts...)
			newArgs = append(newArgs, args[i+1:]...)
			args = newArgs
		}
	}
}

func splitShortOptions(set *flag.FlagSet, arg string) []string {
	shortFlagsExist := func(s string) bool {
		for _, c := range s[1:] {
			if f := set.Lookup(string(c)); f == nil {
				return false
			}
		}
		return true
	}

	if !isSplittable(arg) || !shortFlagsExist(arg) {
		return []string{arg}
	}

	separated := make([]string, 0, len(arg)-1)
	for _, flagChar := range arg[1:] {
		separated = append(separated, "-"+string(flagChar))
	}

	return separated
}

func isSplittable(flagArg string) bool {
	return strings.HasPrefix(flagArg, "-") && !strings.HasPrefix(flagArg, "--") && len(flagArg) > 2
}
