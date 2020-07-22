package graphql

import (
	"context"
	"fmt"

	"github.com/graphql-go/graphql/gqlerrors"
)

type (
	// ParseFinishFunc is called when the parse of the query is done
	ParseFinishFunc func(error)
	// parseFinishFuncHandler handles the call of all the ParseFinishFuncs from the extenisons
	parseFinishFuncHandler func(error) []gqlerrors.FormattedError

	// ValidationFinishFunc is called when the Validation of the query is finished
	ValidationFinishFunc func([]gqlerrors.FormattedError)
	// validationFinishFuncHandler responsible for the call of all the ValidationFinishFuncs
	validationFinishFuncHandler func([]gqlerrors.FormattedError) []gqlerrors.FormattedError

	// ExecutionFinishFunc is called when the execution is done
	ExecutionFinishFunc func(*Result)
	// executionFinishFuncHandler calls all the ExecutionFinishFuncs from each extension
	executionFinishFuncHandler func(*Result) []gqlerrors.FormattedError

	// ResolveFieldFinishFunc is called with the result of the ResolveFn and the error it returned
	ResolveFieldFinishFunc func(interface{}, error)
	// resolveFieldFinishFuncHandler calls the resolveFieldFinishFns for all the extensions
	resolveFieldFinishFuncHandler func(interface{}, error) []gqlerrors.FormattedError
)

// Extension is an interface for extensions in graphql
type Extension interface {
	// Init is used to help you initialize the extension
	Init(context.Context, *Params) context.Context

	// Name returns the name of the extension (make sure it's custom)
	Name() string

	// ParseDidStart is being called before starting the parse
	ParseDidStart(context.Context) (context.Context, ParseFinishFunc)

	// ValidationDidStart is called just before the validation begins
	ValidationDidStart(context.Context) (context.Context, ValidationFinishFunc)

	// ExecutionDidStart notifies about the start of the execution
	ExecutionDidStart(context.Context) (context.Context, ExecutionFinishFunc)

	// ResolveFieldDidStart notifies about the start of the resolving of a field
	ResolveFieldDidStart(context.Context, *ResolveInfo) (context.Context, ResolveFieldFinishFunc)

	// HasResult returns if the extension wants to add data to the result
	HasResult() bool

	// GetResult returns the data that the extension wants to add to the result
	GetResult(context.Context) interface{}
}

// handleExtensionsInits handles all the init functions for all the extensions in the schema
func handleExtensionsInits(p *Params) gqlerrors.FormattedErrors {
	errs := gqlerrors.FormattedErrors{}
	for _, ext := range p.Schema.extensions {
		func() {
			// catch panic from an extension init fn
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.Init: %v", ext.Name(), r.(error))))
				}
			}()
			// update context
			p.Context = ext.Init(p.Context, p)
		}()
	}
	return errs
}

// handleExtensionsParseDidStart runs the ParseDidStart functions for each extension
func handleExtensionsParseDidStart(p *Params) ([]gqlerrors.FormattedError, parseFinishFuncHandler) {
	fs := map[string]ParseFinishFunc{}
	errs := gqlerrors.FormattedErrors{}
	for _, ext := range p.Schema.extensions {
		var (
			ctx      context.Context
			finishFn ParseFinishFunc
		)
		// catch panic from an extension's parseDidStart functions
		func() {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.ParseDidStart: %v", ext.Name(), r.(error))))
				}
			}()
			ctx, finishFn = ext.ParseDidStart(p.Context)
			// update context
			p.Context = ctx
			fs[ext.Name()] = finishFn
		}()
	}
	return errs, func(err error) []gqlerrors.FormattedError {
		errs := gqlerrors.FormattedErrors{}
		for name, fn := range fs {
			func() {
				// catch panic from a finishFn
				defer func() {
					if r := recover(); r != nil {
						errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.ParseFinishFunc: %v", name, r.(error))))
					}
				}()
				fn(err)
			}()
		}
		return errs
	}
}

// handleExtensionsValidationDidStart notifies the extensions about the start of the validation process
func handleExtensionsValidationDidStart(p *Params) ([]gqlerrors.FormattedError, validationFinishFuncHandler) {
	fs := map[string]ValidationFinishFunc{}
	errs := gqlerrors.FormattedErrors{}
	for _, ext := range p.Schema.extensions {
		var (
			ctx      context.Context
			finishFn ValidationFinishFunc
		)
		// catch panic from an extension's validationDidStart function
		func() {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.ValidationDidStart: %v", ext.Name(), r.(error))))
				}
			}()
			ctx, finishFn = ext.ValidationDidStart(p.Context)
			// update context
			p.Context = ctx
			fs[ext.Name()] = finishFn
		}()
	}
	return errs, func(errs []gqlerrors.FormattedError) []gqlerrors.FormattedError {
		extErrs := gqlerrors.FormattedErrors{}
		for name, finishFn := range fs {
			func() {
				// catch panic from a finishFn
				defer func() {
					if r := recover(); r != nil {
						extErrs = append(extErrs, gqlerrors.FormatError(fmt.Errorf("%s.ValidationFinishFunc: %v", name, r.(error))))
					}
				}()
				finishFn(errs)
			}()
		}
		return extErrs
	}
}

// handleExecutionDidStart handles the ExecutionDidStart functions
func handleExtensionsExecutionDidStart(p *ExecuteParams) ([]gqlerrors.FormattedError, executionFinishFuncHandler) {
	fs := map[string]ExecutionFinishFunc{}
	errs := gqlerrors.FormattedErrors{}
	for _, ext := range p.Schema.extensions {
		var (
			ctx      context.Context
			finishFn ExecutionFinishFunc
		)
		// catch panic from an extension's executionDidStart function
		func() {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.ExecutionDidStart: %v", ext.Name(), r.(error))))
				}
			}()
			ctx, finishFn = ext.ExecutionDidStart(p.Context)
			// update context
			p.Context = ctx
			fs[ext.Name()] = finishFn
		}()
	}
	return errs, func(result *Result) []gqlerrors.FormattedError {
		extErrs := gqlerrors.FormattedErrors{}
		for name, finishFn := range fs {
			func() {
				// catch panic from a finishFn
				defer func() {
					if r := recover(); r != nil {
						extErrs = append(extErrs, gqlerrors.FormatError(fmt.Errorf("%s.ExecutionFinishFunc: %v", name, r.(error))))
					}
				}()
				finishFn(result)
			}()
		}
		return extErrs
	}
}

// handleResolveFieldDidStart handles the notification of the extensions about the start of a resolve function
func handleExtensionsResolveFieldDidStart(exts []Extension, p *executionContext, i *ResolveInfo) ([]gqlerrors.FormattedError, resolveFieldFinishFuncHandler) {
	fs := map[string]ResolveFieldFinishFunc{}
	errs := gqlerrors.FormattedErrors{}
	for _, ext := range p.Schema.extensions {
		var (
			ctx      context.Context
			finishFn ResolveFieldFinishFunc
		)
		// catch panic from an extension's resolveFieldDidStart function
		func() {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, gqlerrors.FormatError(fmt.Errorf("%s.ResolveFieldDidStart: %v", ext.Name(), r.(error))))
				}
			}()
			ctx, finishFn = ext.ResolveFieldDidStart(p.Context, i)
			// update context
			p.Context = ctx
			fs[ext.Name()] = finishFn
		}()
	}
	return errs, func(val interface{}, err error) []gqlerrors.FormattedError {
		extErrs := gqlerrors.FormattedErrors{}
		for name, finishFn := range fs {
			func() {
				// catch panic from a finishFn
				defer func() {
					if r := recover(); r != nil {
						extErrs = append(extErrs, gqlerrors.FormatError(fmt.Errorf("%s.ResolveFieldFinishFunc: %v", name, r.(error))))
					}
				}()
				finishFn(val, err)
			}()
		}
		return extErrs
	}
}

func addExtensionResults(p *ExecuteParams, result *Result) {
	if len(p.Schema.extensions) != 0 {
		for _, ext := range p.Schema.extensions {
			func() {
				defer func() {
					if r := recover(); r != nil {
						result.Errors = append(result.Errors, gqlerrors.FormatError(fmt.Errorf("%s.GetResult: %v", ext.Name(), r.(error))))
					}
				}()
				if ext.HasResult() {
					if result.Extensions == nil {
						result.Extensions = make(map[string]interface{})
					}
					result.Extensions[ext.Name()] = ext.GetResult(p.Context)
				}
			}()
		}
	}
}
