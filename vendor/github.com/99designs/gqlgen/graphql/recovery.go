package graphql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
)

type RecoverFunc func(ctx context.Context, err interface{}) (userMessage error)

func DefaultRecover(ctx context.Context, err interface{}) error {
	fmt.Fprintln(os.Stderr, err)
	fmt.Fprintln(os.Stderr)
	debug.PrintStack()

	return errors.New("internal system error")
}
