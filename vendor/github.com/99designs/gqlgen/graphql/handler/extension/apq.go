package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/99designs/gqlgen/graphql/errcode"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/99designs/gqlgen/graphql"
	"github.com/mitchellh/mapstructure"
)

const errPersistedQueryNotFound = "PersistedQueryNotFound"
const errPersistedQueryNotFoundCode = "PERSISTED_QUERY_NOT_FOUND"

// AutomaticPersistedQuery saves client upload by optimistically sending only the hashes of queries, if the server
// does not yet know what the query is for the hash it will respond telling the client to send the query along with the
// hash in the next request.
// see https://github.com/apollographql/apollo-link-persisted-queries
type AutomaticPersistedQuery struct {
	Cache graphql.Cache
}

type ApqStats struct {
	// The hash of the incoming query
	Hash string

	// SentQuery is true if the incoming request sent the full query
	SentQuery bool
}

const apqExtension = "APQ"

var _ interface {
	graphql.OperationParameterMutator
	graphql.HandlerExtension
} = AutomaticPersistedQuery{}

func (a AutomaticPersistedQuery) ExtensionName() string {
	return "AutomaticPersistedQuery"
}

func (a AutomaticPersistedQuery) Validate(schema graphql.ExecutableSchema) error {
	if a.Cache == nil {
		return fmt.Errorf("AutomaticPersistedQuery.Cache can not be nil")
	}
	return nil
}

func (a AutomaticPersistedQuery) MutateOperationParameters(ctx context.Context, rawParams *graphql.RawParams) *gqlerror.Error {
	if rawParams.Extensions["persistedQuery"] == nil {
		return nil
	}

	var extension struct {
		Sha256  string `mapstructure:"sha256Hash"`
		Version int64  `mapstructure:"version"`
	}

	if err := mapstructure.Decode(rawParams.Extensions["persistedQuery"], &extension); err != nil {
		return gqlerror.Errorf("invalid APQ extension data")
	}

	if extension.Version != 1 {
		return gqlerror.Errorf("unsupported APQ version")
	}

	fullQuery := false
	if rawParams.Query == "" {
		// client sent optimistic query hash without query string, get it from the cache
		query, ok := a.Cache.Get(ctx, extension.Sha256)
		if !ok {
			err := gqlerror.Errorf(errPersistedQueryNotFound)
			errcode.Set(err, errPersistedQueryNotFoundCode)
			return err
		}
		rawParams.Query = query.(string)
	} else {
		// client sent optimistic query hash with query string, verify and store it
		if computeQueryHash(rawParams.Query) != extension.Sha256 {
			return gqlerror.Errorf("provided APQ hash does not match query")
		}
		a.Cache.Add(ctx, extension.Sha256, rawParams.Query)
		fullQuery = true
	}

	graphql.GetOperationContext(ctx).Stats.SetExtension(apqExtension, &ApqStats{
		Hash:      extension.Sha256,
		SentQuery: fullQuery,
	})

	return nil
}

func GetApqStats(ctx context.Context) *ApqStats {
	rc := graphql.GetOperationContext(ctx)
	if rc == nil {
		return nil
	}

	s, _ := rc.Stats.GetExtension(apqExtension).(*ApqStats)
	return s
}

func computeQueryHash(query string) string {
	b := sha256.Sum256([]byte(query))
	return hex.EncodeToString(b[:])
}
