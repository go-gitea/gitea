package transfer

import (
	"fmt"
	"strings"
)

// batch request argument keys.
const (
	HashAlgoKey  = "hash-algo"
	TransferKey  = "transfer"
	RefnameKey   = "refname"
	ExpiresInKey = "expires-in"
	ExpiresAtKey = "expires-at"
	SizeKey      = "size"
	PathKey      = "path"
	LimitKey     = "limit"
	CursorKey    = "cursor"
)

// ParseArgs parses the given args.
func ParseArgs(parts []string) (Args, error) {
	args := make(Args, 0)
	for _, line := range parts {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument: %q", line)
		}
		key, value := parts[0], parts[1]
		args[key] = value
	}
	return args, nil
}

// ArgsToList converts the given args to a list.
func ArgsToList(args Args) []string {
	list := make([]string, 0)
	for key, value := range args {
		list = append(list, fmt.Sprintf("%s=%s", key, value))
	}
	return list
}

// Args is a key-value pair of arguments.
type Args map[string]string

// String returns the string representation of the arguments.
func (a Args) String() string {
	return strings.Join(ArgsToList(a), " ")
}
