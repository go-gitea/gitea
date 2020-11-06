// +build !go1.8

package handlers

type loggingResponseWriter interface {
	commonLoggingResponseWriter
}
