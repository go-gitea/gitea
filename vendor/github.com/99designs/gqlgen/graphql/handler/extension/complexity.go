package extension

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/complexity"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const errComplexityLimit = "COMPLEXITY_LIMIT_EXCEEDED"

// ComplexityLimit allows you to define a limit on query complexity
//
// If a query is submitted that exceeds the limit, a 422 status code will be returned.
type ComplexityLimit struct {
	Func func(ctx context.Context, rc *graphql.OperationContext) int

	es graphql.ExecutableSchema
}

var _ interface {
	graphql.OperationContextMutator
	graphql.HandlerExtension
} = &ComplexityLimit{}

const complexityExtension = "ComplexityLimit"

type ComplexityStats struct {
	// The calculated complexity for this request
	Complexity int

	// The complexity limit for this request returned by the extension func
	ComplexityLimit int
}

// FixedComplexityLimit sets a complexity limit that does not change
func FixedComplexityLimit(limit int) *ComplexityLimit {
	return &ComplexityLimit{
		Func: func(ctx context.Context, rc *graphql.OperationContext) int {
			return limit
		},
	}
}

func (c ComplexityLimit) ExtensionName() string {
	return complexityExtension
}

func (c *ComplexityLimit) Validate(schema graphql.ExecutableSchema) error {
	if c.Func == nil {
		return fmt.Errorf("ComplexityLimit func can not be nil")
	}
	c.es = schema
	return nil
}

func (c ComplexityLimit) MutateOperationContext(ctx context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	op := rc.Doc.Operations.ForName(rc.OperationName)
	complexity := complexity.Calculate(c.es, op, rc.Variables)

	limit := c.Func(ctx, rc)

	rc.Stats.SetExtension(complexityExtension, &ComplexityStats{
		Complexity:      complexity,
		ComplexityLimit: limit,
	})

	if complexity > limit {
		err := gqlerror.Errorf("operation has complexity %d, which exceeds the limit of %d", complexity, limit)
		errcode.Set(err, errComplexityLimit)
		return err
	}

	return nil
}

func GetComplexityStats(ctx context.Context) *ComplexityStats {
	rc := graphql.GetOperationContext(ctx)
	if rc == nil {
		return nil
	}

	s, _ := rc.Stats.GetExtension(complexityExtension).(*ComplexityStats)
	return s
}
