package executor

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

// Executor executes graphql queries against a schema.
type Executor struct {
	es         graphql.ExecutableSchema
	extensions []graphql.HandlerExtension
	ext        extensions

	errorPresenter graphql.ErrorPresenterFunc
	recoverFunc    graphql.RecoverFunc
	queryCache     graphql.Cache
}

var _ graphql.GraphExecutor = &Executor{}

// New creates a new Executor with the given schema, and a default error and
// recovery callbacks, and no query cache or extensions.
func New(es graphql.ExecutableSchema) *Executor {
	e := &Executor{
		es:             es,
		errorPresenter: graphql.DefaultErrorPresenter,
		recoverFunc:    graphql.DefaultRecover,
		queryCache:     graphql.NoCache{},
		ext:            processExtensions(nil),
	}
	return e
}

func (e *Executor) CreateOperationContext(ctx context.Context, params *graphql.RawParams) (*graphql.OperationContext, gqlerror.List) {
	rc := &graphql.OperationContext{
		DisableIntrospection: true,
		RecoverFunc:          e.recoverFunc,
		ResolverMiddleware:   e.ext.fieldMiddleware,
		Stats: graphql.Stats{
			Read:           params.ReadTime,
			OperationStart: graphql.GetStartTime(ctx),
		},
	}
	ctx = graphql.WithOperationContext(ctx, rc)

	for _, p := range e.ext.operationParameterMutators {
		if err := p.MutateOperationParameters(ctx, params); err != nil {
			return rc, gqlerror.List{err}
		}
	}

	rc.RawQuery = params.Query
	rc.OperationName = params.OperationName

	var listErr gqlerror.List
	rc.Doc, listErr = e.parseQuery(ctx, &rc.Stats, params.Query)
	if len(listErr) != 0 {
		return rc, listErr
	}

	rc.Operation = rc.Doc.Operations.ForName(params.OperationName)
	if rc.Operation == nil {
		return rc, gqlerror.List{gqlerror.Errorf("operation %s not found", params.OperationName)}
	}

	var err *gqlerror.Error
	rc.Variables, err = validator.VariableValues(e.es.Schema(), rc.Operation, params.Variables)
	if err != nil {
		errcode.Set(err, errcode.ValidationFailed)
		return rc, gqlerror.List{err}
	}
	rc.Stats.Validation.End = graphql.Now()

	for _, p := range e.ext.operationContextMutators {
		if err := p.MutateOperationContext(ctx, rc); err != nil {
			return rc, gqlerror.List{err}
		}
	}

	return rc, nil
}

func (e *Executor) DispatchOperation(ctx context.Context, rc *graphql.OperationContext) (graphql.ResponseHandler, context.Context) {
	ctx = graphql.WithOperationContext(ctx, rc)

	var innerCtx context.Context
	res := e.ext.operationMiddleware(ctx, func(ctx context.Context) graphql.ResponseHandler {
		innerCtx = ctx

		tmpResponseContext := graphql.WithResponseContext(ctx, e.errorPresenter, e.recoverFunc)
		responses := e.es.Exec(tmpResponseContext)
		if errs := graphql.GetErrors(tmpResponseContext); errs != nil {
			return graphql.OneShot(&graphql.Response{Errors: errs})
		}

		return func(ctx context.Context) *graphql.Response {
			ctx = graphql.WithResponseContext(ctx, e.errorPresenter, e.recoverFunc)
			resp := e.ext.responseMiddleware(ctx, func(ctx context.Context) *graphql.Response {
				resp := responses(ctx)
				if resp == nil {
					return nil
				}
				resp.Errors = append(resp.Errors, graphql.GetErrors(ctx)...)
				resp.Extensions = graphql.GetExtensions(ctx)
				return resp
			})
			if resp == nil {
				return nil
			}

			return resp
		}
	})

	return res, innerCtx
}

func (e *Executor) DispatchError(ctx context.Context, list gqlerror.List) *graphql.Response {
	ctx = graphql.WithResponseContext(ctx, e.errorPresenter, e.recoverFunc)
	for _, gErr := range list {
		graphql.AddError(ctx, gErr)
	}

	resp := e.ext.responseMiddleware(ctx, func(ctx context.Context) *graphql.Response {
		resp := &graphql.Response{
			Errors: list,
		}
		resp.Extensions = graphql.GetExtensions(ctx)
		return resp
	})

	return resp
}

func (e *Executor) PresentRecoveredError(ctx context.Context, err interface{}) *gqlerror.Error {
	return e.errorPresenter(ctx, e.recoverFunc(ctx, err))
}

func (e *Executor) SetQueryCache(cache graphql.Cache) {
	e.queryCache = cache
}

func (e *Executor) SetErrorPresenter(f graphql.ErrorPresenterFunc) {
	e.errorPresenter = f
}

func (e *Executor) SetRecoverFunc(f graphql.RecoverFunc) {
	e.recoverFunc = f
}

// parseQuery decodes the incoming query and validates it, pulling from cache if present.
//
// NOTE: This should NOT look at variables, they will change per request. It should only parse and validate
// the raw query string.
func (e *Executor) parseQuery(ctx context.Context, stats *graphql.Stats, query string) (*ast.QueryDocument, gqlerror.List) {
	stats.Parsing.Start = graphql.Now()

	if doc, ok := e.queryCache.Get(ctx, query); ok {
		now := graphql.Now()

		stats.Parsing.End = now
		stats.Validation.Start = now
		return doc.(*ast.QueryDocument), nil
	}

	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		errcode.Set(err, errcode.ParseFailed)
		return nil, gqlerror.List{err}
	}
	stats.Parsing.End = graphql.Now()

	stats.Validation.Start = graphql.Now()
	listErr := validator.Validate(e.es.Schema(), doc)
	if len(listErr) != 0 {
		for _, e := range listErr {
			errcode.Set(e, errcode.ValidationFailed)
		}
		return nil, listErr
	}

	e.queryCache.Add(ctx, query, doc)

	return doc, nil
}
