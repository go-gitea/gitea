package validator

import (
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type ErrorOption func(err *gqlerror.Error)

func Message(msg string, args ...interface{}) ErrorOption {
	return func(err *gqlerror.Error) {
		err.Message += fmt.Sprintf(msg, args...)
	}
}

func At(position *ast.Position) ErrorOption {
	return func(err *gqlerror.Error) {
		if position == nil {
			return
		}
		err.Locations = append(err.Locations, gqlerror.Location{
			Line:   position.Line,
			Column: position.Column,
		})
		if position.Src.Name != "" {
			err.SetFile(position.Src.Name)
		}
	}
}

func SuggestListQuoted(prefix string, typed string, suggestions []string) ErrorOption {
	suggested := SuggestionList(typed, suggestions)
	return func(err *gqlerror.Error) {
		if len(suggested) > 0 {
			err.Message += " " + prefix + " " + QuotedOrList(suggested...) + "?"
		}
	}
}

func SuggestListUnquoted(prefix string, typed string, suggestions []string) ErrorOption {
	suggested := SuggestionList(typed, suggestions)
	return func(err *gqlerror.Error) {
		if len(suggested) > 0 {
			err.Message += " " + prefix + " " + OrList(suggested...) + "?"
		}
	}
}

func Suggestf(suggestion string, args ...interface{}) ErrorOption {
	return func(err *gqlerror.Error) {
		err.Message += " Did you mean " + fmt.Sprintf(suggestion, args...) + "?"
	}
}
