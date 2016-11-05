package builder

import "errors"

var (
	ErrNotSupportType    = errors.New("not supported SQL type")
	ErrNoNotInConditions = errors.New("No NOT IN conditions")
	ErrNoInConditions    = errors.New("No IN conditions")
)
