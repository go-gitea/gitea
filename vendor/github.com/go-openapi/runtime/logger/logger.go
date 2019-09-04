package logger

import "os"

type Logger interface {
	Printf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

func DebugEnabled() bool {
	return os.Getenv("SWAGGER_DEBUG") != "" || os.Getenv("DEBUG") != ""
}
