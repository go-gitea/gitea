package errcode

import (
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const ValidationFailed = "GRAPHQL_VALIDATION_FAILED"
const ParseFailed = "GRAPHQL_PARSE_FAILED"

type ErrorKind int

const (
	// issues with graphql (validation, parsing).  422s in http, GQL_ERROR in websocket
	KindProtocol ErrorKind = iota
	// user errors, 200s in http, GQL_DATA in websocket
	KindUser
)

var codeType = map[string]ErrorKind{
	ValidationFailed: KindProtocol,
	ParseFailed:      KindProtocol,
}

// RegisterErrorType should be called by extensions that want to customize the http status codes for errors they return
func RegisterErrorType(code string, kind ErrorKind) {
	codeType[code] = kind
}

// Set the error code on a given graphql error extension
func Set(err *gqlerror.Error, value string) {
	if err.Extensions == nil {
		err.Extensions = map[string]interface{}{}
	}

	err.Extensions["code"] = value
}

// get the kind of the first non User error, defaults to User if no errors have a custom extension
func GetErrorKind(errs gqlerror.List) ErrorKind {
	for _, err := range errs {
		if code, ok := err.Extensions["code"].(string); ok {
			if kind, ok := codeType[code]; ok && kind != KindUser {
				return kind
			}
		}
	}

	return KindUser
}
