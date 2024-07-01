package transfer

// Logger is a logging interface.
type Logger interface {
	// Log logs the given message and structured arguments.
	Log(msg string, kv ...interface{})
}

type noopLogger struct{}

var _ Logger = (*noopLogger)(nil)

// Log implements Logger.
func (*noopLogger) Log(string, ...interface{}) {}
